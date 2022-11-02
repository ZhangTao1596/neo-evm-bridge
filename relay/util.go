package relay

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joeqian10/neo3-gogogo/crypto"
	"github.com/joeqian10/neo3-gogogo/helper"
	"github.com/joeqian10/neo3-gogogo/io"
	"github.com/joeqian10/neo3-gogogo/mpt"
	"github.com/joeqian10/neo3-gogogo/rpc/models"
	"github.com/neo-ngd/neo-go/pkg/core/block"
	"github.com/neo-ngd/neo-go/pkg/core/transaction"
	"github.com/neo-ngd/neo-go/pkg/crypto/hash"
	nio "github.com/neo-ngd/neo-go/pkg/io"
)

func proveTx(block *models.RpcBlock, txid string) ([]byte, error) {
	hashes := make([]common.Hash, len(block.Tx))
	for i, tx := range block.Tx {
		h, err := helper.UInt256FromString(tx.Hash)
		if err != nil {
			return nil, fmt.Errorf("can't parse tx hash in header: %w", err)
		}
		hashes[i] = common.BytesToHash(h.ToByteArray())
	}
	tree, err := hash.NewMerkleTree(hashes)
	if err != nil {
		return nil, err
	}
	h, err := helper.UInt256FromString(txid)
	if err != nil {
		return nil, fmt.Errorf("can't parse tx hash in header: %w", err)
	}
	proofs, path, err := tree.Prove(common.BytesToHash(h.ToByteArray()))
	if err != nil {
		return nil, err
	}
	proof := make([]byte, len(proofs)*common.HashLength+4)
	binary.LittleEndian.PutUint32(proof, path)
	for i, hash := range proofs {
		copy(proof[4+i*common.HashLength:], hash[:])
	}
	return proof, nil
}

func rpcHeaderToBlockHeader(h models.RpcBlockHeader) (*block.Header, error) {
	nonce, err := strconv.ParseUint(h.Nonce, 16, 64)
	if err != nil {
		return nil, fmt.Errorf("can't parse nonce in header: %w", err)
	}
	consensus, err := crypto.AddressToScriptHash(h.NextConsensus, helper.DefaultAddressVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse next consensus in header: %w", err)
	}
	verification, err := crypto.Base64Decode(h.Witnesses[0].Verification)
	if err != nil {
		return nil, fmt.Errorf("can't parse verification in header: %w", err)
	}
	invocation, err := crypto.Base64Decode(h.Witnesses[0].Invocation)
	if err != nil {
		return nil, fmt.Errorf("can't parse invocation in header: %w", err)
	}
	preHash, err := helper.UInt256FromString(h.PreviousBlockHash)
	if err != nil {
		return nil, fmt.Errorf("can't parse prehash in header: %w", err)
	}
	merkleRoot, err := helper.UInt256FromString(h.MerkleRoot)
	if err != nil {
		return nil, fmt.Errorf("can't parse merkle root in header: %w", err)
	}
	header := block.Header{
		Version:       uint32(h.Version),
		PrevHash:      common.BytesToHash(preHash.ToByteArray()),
		MerkleRoot:    common.BytesToHash(merkleRoot.ToByteArray()),
		Timestamp:     uint64(h.Time),
		Nonce:         nonce,
		Index:         uint32(h.Index),
		NextConsensus: common.BytesToAddress(consensus.ToByteArray()),
		PrimaryIndex:  h.PrimaryIndex,
		Witness: transaction.Witness{
			VerificationScript: verification,
			InvocationScript:   invocation,
		},
	}
	return &header, nil
}

func blockHeaderToBytes(header *block.Header) ([]byte, error) {
	writer := nio.NewBufBinWriter()
	header.EncodeBinary(writer.BinWriter)
	if writer.Err != nil {
		return nil, writer.Err
	}
	return writer.Bytes(), nil
}

func staterootToBytes(stateroot *mpt.StateRoot) ([]byte, error) {
	writer := io.NewBufBinaryWriter()
	stateroot.Serialize(writer.BinaryWriter)
	if writer.Err != nil {
		return nil, writer.Err
	}
	return writer.Bytes(), nil
}
