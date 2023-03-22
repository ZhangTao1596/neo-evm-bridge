package constantclient

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/neo-ngd/neo-go/pkg/core/state"
	"github.com/neo-ngd/neo-go/pkg/rpc/client"
	"github.com/neo-ngd/neo-go/pkg/rpc/response"
	"github.com/neo-ngd/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	mstate "github.com/nspcc-dev/neo-go/pkg/core/state"
	mio "github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	mresult "github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type ConstantClient struct {
	mainSeeds []string
	sideSeeds []string
	mIndex    int
	sIndex    int
	mClient   *rpcclient.Client
	sClient   *client.Client
}

func New(mseeds, sseeds []string) *ConstantClient {
	c := &ConstantClient{
		mainSeeds: mseeds,
		sideSeeds: sseeds,
		mIndex:    0,
		sIndex:    0,
		mClient:   nil,
		sClient:   nil,
	}
	c.ensureNewClient(true)
	c.ensureNewClient(false)
	return c
}

func newClient(index *int, count int, newClient func(index int) (interface{}, error)) interface{} {
	i := *index
	cli, err := newClient(*index)
	for err != nil {
		*index = (*index + 1) % count
		if *index == i {
			panic(fmt.Errorf("can't initialize client"))
		}
		cli, err = newClient(*index)
	}
	return cli
}

func (c *ConstantClient) ensureNewClient(isMain bool) {
	if isMain {
		c.mClient = newClient(&c.mIndex, len(c.mainSeeds), func(index int) (interface{}, error) {
			cli, err := rpcclient.New(context.Background(), c.mainSeeds[index], rpcclient.Options{})
			if err != nil {
				return nil, err
			}
			err = cli.Init()
			return cli, err
		}).(*rpcclient.Client)
	} else {
		c.sClient = newClient(&c.sIndex, len(c.sideSeeds), func(index int) (interface{}, error) {
			cli, err := client.New(context.Background(), c.sideSeeds[index], client.Options{})
			if err != nil {
				return nil, err
			}
			err = cli.Init()
			return cli, err
		}).(*client.Client)
	}
}

func isSideNetworkError(err error) bool {
	_, ok := err.(*neorpc.Error)
	return !ok
}

func isMainNetworkError(err error) bool {
	_, ok := err.(*response.Error)
	return !ok
}

func isNetworkError(err error, isMain bool) bool {
	if isMain {
		return isMainNetworkError(err)
	}
	return isSideNetworkError(err)
}

func (c *ConstantClient) ensureRequest(isMain bool, doRequest func() (interface{}, error), resultExpected bool) (interface{}, error) {
	for {
		r, err := doRequest()
		if err != nil {
			if resultExpected || !isNetworkError(err, isMain) {
				c.ensureNewClient(false)
				continue
			}
		}
		return r, err
	}
}

func (c *ConstantClient) GetApplicationLog(txid util.Uint256) *mresult.ApplicationLog {
	r, err := c.ensureRequest(true, func() (interface{}, error) {
		return c.mClient.GetApplicationLog(txid, nil)
	}, true)
	if r == nil || err != nil {
		return nil
	}
	return r.(*mresult.ApplicationLog)
}

func (c *ConstantClient) GetBlock(index uint32) *block.Block {
	r, err := c.ensureRequest(true, func() (interface{}, error) {
		return c.mClient.GetBlockByIndex(index)
	}, false)
	if r == nil || err != nil {
		return nil
	}
	return r.(*block.Block)
}

func (c *ConstantClient) GetStateRoot(index uint32) *mstate.MPTRoot {
	r, err := c.ensureRequest(true, func() (interface{}, error) {
		return c.mClient.GetStateRootByHeight(index)
	}, false)
	if r == nil || err != nil {
		return nil
	}
	return r.(*mstate.MPTRoot)
}

func proofToBytes(proof *mresult.ProofWithKey) []byte {
	w := mio.NewBufBinWriter()
	proof.EncodeBinary(w.BinWriter)
	return w.Bytes()
}

func (c *ConstantClient) GetProof(rootHash util.Uint256, contractHash util.Uint160, key []byte) []byte {
	r, err := c.ensureRequest(true, func() (interface{}, error) {
		return c.mClient.FindStates(rootHash, contractHash, key, nil, nil)
	}, false)
	if r == nil || err != nil {
		return nil
	}
	res := r.(mresult.FindStates)
	return proofToBytes(res.FirstProof)
}

func (c *ConstantClient) Eth_NativeContract(name string) *state.NativeContract {
	r, _ := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.GetNativeContracts()
	}, true)
	cs := r.([]state.NativeContract)
	for _, contract := range cs {
		if contract.Name == name {
			return &contract
		}
	}
	return nil
}

func (c *ConstantClient) Eth_ChainId() uint64 {
	r, _ := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_ChainId()
	}, true)
	return r.(uint64)
}

func (c *ConstantClient) Eth_GasPrice() *big.Int {
	r, _ := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_GasPrice()
	}, true)
	return r.(*big.Int)
}

func (c *ConstantClient) Eth_GetTransactionCount(address common.Address) uint64 {
	r, _ := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_GetTransactionCount(address)
	}, true)
	return r.(uint64)
}

func (c *ConstantClient) Eth_EstimateGas(tx *result.TransactionObject) (uint64, error) {
	r, err := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_EstimateGas(tx)
	}, false)
	if err != nil {
		return 0, err
	}
	return r.(uint64), nil
}

func (c *ConstantClient) Eth_SendRawTransaction(rawTx []byte) (common.Hash, error) {
	r, err := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_SendRawTransaction(rawTx)
	}, false)
	if err != nil {
		return common.Hash{}, err
	}
	return r.(common.Hash), nil
}

func (c *ConstantClient) Eth_GetTransactionByHash(hash common.Hash) *result.TransactionOutputRaw {
	r, _ := c.ensureRequest(false, func() (interface{}, error) {
		return c.sClient.Eth_GetTransactionByHash(hash)
	}, true)
	return r.(*result.TransactionOutputRaw)
}
