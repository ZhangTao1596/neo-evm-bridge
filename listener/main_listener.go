package listener

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ZhangTao1596/neo-evm-bridge/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ZhangTao1596/neo-evm-bridge/constantclient"

	"github.com/joeqian10/neo3-gogogo/crypto"
	"github.com/joeqian10/neo3-gogogo/helper"
	"github.com/joeqian10/neo3-gogogo/mpt"
	"github.com/joeqian10/neo3-gogogo/rpc/models"
	"github.com/joeqian10/neo3-gogogo/vm"
	"github.com/neo-ngd/neo-go/pkg/core/state"
	"github.com/neo-ngd/neo-go/pkg/core/transaction"
	"github.com/neo-ngd/neo-go/pkg/rpc/response/result"
	"github.com/neo-ngd/neo-go/pkg/wallet"
)

const (
	DepositPrefix          = 0x01
	ValidatorsKey          = 0x03
	StateValidatorRole     = 4
	RoleManagementContract = "0x49cf4e5378ffcd4dec034fd98a174c5491e395e2"
	ConnectorContractName  = "Connector"
	BlockTimeSeconds       = 15
)

type Listener struct {
	config                        *config.Config
	evmLayerContract              helper.UInt160
	running                       bool
	lastHeader                    *models.RpcBlockHeader
	roleManagementContractAddress *helper.UInt160
	client                        *constantclient.ConstantClient
	connector                     *state.NativeContract
	account                       *wallet.Account
}

func NewListener(config *config.Config) (*Listener, error) {
	c, err := helper.UInt160FromString(config.MainContract)
	if err != nil {
		return nil, err
	}
	roleManagement, err := helper.UInt160FromString(RoleManagementContract)
	if err != nil {
		return nil, err
	}
	client := constantclient.New(config.MainSeeds, config.SideSeeds)

	connector := client.GetNativeContract(ConnectorContractName)
	if connector == nil {
		return nil, errors.New("can't get connector contract")
	}
	return &Listener{
		config:                        config,
		evmLayerContract:              *c,
		roleManagementContractAddress: roleManagement,
		client:                        client,
		connector:                     connector,
	}, nil
}

func (l *Listener) Stop() {
	l.running = false
}

func (l *Listener) Listen() {
	const startIndex = uint32(0)

	for i := startIndex; ; {
		if !l.running {
			break
		}
		block := l.client.GetBlock(i)
		if block == nil {
			time.Sleep(15 * time.Second)
			continue
		}
		batch := new(taskBatch)
		batch.block = block
		batch.isJoint = l.isJointHeader(&block.RpcBlockHeader)
		for _, tx := range block.Tx {
			txid, err := helper.UInt256FromString(tx.Hash)
			if err != nil {
				panic(fmt.Errorf("invalid tx id: %w", err))
			}
			applicationlog := l.client.GetApplicationLog(tx.Hash)
			for _, execution := range applicationlog.Executions {
				if execution.Trigger == "Application" && execution.VMState == "HALT" {
					for _, notification := range execution.Notifications {
						if isDeposit, requestId, _, _, _ := l.isDepositEvent(&notification); isDeposit {
							batch.addTask(depositTask{
								txid:      txid,
								requestId: requestId,
							})

						} else if isDesignate, _ := l.isDesignateValidatorsEvent(&notification); isDesignate {
							batch.addTask(validatorsDesignateTask{
								txid: txid,
							})
						} else if isStateValidatorsDesignate, index := l.isStateValidatorsDesignatedEvent(&notification); isStateValidatorsDesignate {
							batch.addTask(stateValidatorsChangeTask{
								txid:  txid,
								index: index,
							})
						}
					}
				}
			}
		}
		err := l.sync(batch)
		if err != nil {
			panic(fmt.Errorf("can't sync block %d: %w", i, err))
		}
		err = l.saveHandled()
		if err != nil {
			panic(fmt.Errorf("can't save block %d: %w", i, err))
		}
		l.lastHeader = &block.RpcBlockHeader
		i++
	}
}

func (l *Listener) saveHandled() error {
	return nil
}

func (l *Listener) isJointHeader(header *models.RpcBlockHeader) bool {
	if l.lastHeader == nil {
		block := l.client.GetBlock(uint32(header.Index) - 1)
		l.lastHeader = &block.RpcBlockHeader
	}
	return l.lastHeader.NextConsensus != header.NextConsensus
}

func (l *Listener) isRoleManagement(notification *models.RpcNotification) bool {
	contractInNotication, err := helper.UInt160FromString(notification.Contract)
	if err != nil {
		return false
	}
	return *contractInNotication == *l.roleManagementContractAddress
}

func (l *Listener) isStateValidatorsDesignatedEvent(notification *models.RpcNotification) (bool, uint32) {
	if !l.isRoleManagement(notification) || notification.EventName != "Designation" {
		return false, 0
	}
	if notification.State.Type != vm.Array.String() {
		panic("invalid role deposit event type")
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	if len(arr) != 2 {
		panic("invalid role deposite event arguments count")
	}
	role, err := strconv.Atoi(arr[0].Value.(string))
	if err != nil {
		panic(fmt.Errorf("can't parse role: %w", err))
	}
	if role != StateValidatorRole {
		return false, 0
	}
	index, err := strconv.ParseUint(arr[1].Value.(string), 10, 32)
	if err != nil {
		panic(fmt.Errorf("can't parse index: %w", err))
	}
	return true, uint32(index)
}

func (l *Listener) isDepositEvent(notification *models.RpcNotification) (isDeposit bool, requestId uint64, from helper.UInt160, amount int, to helper.UInt160) {
	if !l.isEvmLayerContract(notification) || notification.EventName != "OnDeposited" {
		isDeposit = false
		return
	}
	if notification.State.Type != vm.Array.String() {
		panic("invalid deposited event type")
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	if len(arr) != 4 {
		panic("invalid deposited event arguments count")
	}
	requestId, err := strconv.ParseUint(arr[0].Value.(string), 10, 64)
	if err != nil {
		panic(fmt.Errorf("can't parse request id: %w", err))
	}
	if arr[1].Type != vm.ByteString.String() {
		panic("invalid from type in deposit event")
	}
	from = *helper.UInt160FromBytes(arr[1].Value.([]byte))
	if arr[2].Type != vm.Integer.String() {
		panic("invalid amount type in deposit event")
	}
	amount, err = strconv.Atoi(arr[2].Value.(string))
	if err != nil {
		panic(fmt.Errorf("can't parse amount: %w", err))
	}
	if arr[3].Type != vm.ByteString.String() {
		panic("invalid to type in deposit event")
	}
	to = *helper.UInt160FromBytes(arr[3].Value.([]byte))
	return true, requestId, from, amount, to
}

func (l *Listener) isDesignateValidatorsEvent(notification *models.RpcNotification) (isDesignated bool, pks []crypto.ECPoint) {
	if !l.isEvmLayerContract(notification) || notification.EventName != "OnValidatorsChanged" {
		isDesignated = false
		return
	}
	if notification.State.Type != vm.Array.String() {
		panic("invalid designated event type")
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	pks = make([]crypto.ECPoint, len(arr))
	for i, p := range arr {
		if p.Type != vm.ByteString.String() {
			panic("invalid ecpoint type in deposit event")
		}
		pt, err := crypto.NewECPointFromBytes(p.Value.([]byte))
		if err != nil {
			panic(fmt.Errorf("can't parse ecpoint: %w", err))
		}
		pks[i] = *pt
	}
	return true, pks
}

func (l *Listener) isEvmLayerContract(notification *models.RpcNotification) bool {
	contractInNotication, err := helper.UInt160FromString(notification.Contract)
	if err != nil {
		return false
	}
	return *contractInNotication == l.evmLayerContract
}

func (l *Listener) sync(batch *taskBatch) error {
	transactions := []*types.Transaction{}
	if batch.isJoint || len(batch.tasks) > 0 {
		header, err := rpcHeaderToBlockHeader(batch.block.RpcBlockHeader)
		if err != nil {
			return err
		}
		b, err := blockHeaderToBytes(header)
		if err != nil {
			return err
		}
		tx, err := l.invokeSyncObject("syncHeader", b)
		if err != nil {
			return err
		}
		transactions = append(transactions, tx)
	}
	var stateroot *mpt.StateRoot
	if len(batch.tasks) > 0 {
		stateroot = l.client.GetStateRoot(uint32(batch.block.Index))
		b, err := staterootToBytes(stateroot)
		if err != nil {
			return err
		}
		tx, err := l.invokeSyncObject("syncStateRoot", b)
		if err != nil {
			return err
		}
		transactions = append(transactions, tx)
	}
	err := l.commitTransactions(transactions)
	if err != nil {
		return err
	}
	transactions = transactions[:0]
	for _, t := range batch.tasks {
		var (
			key    []byte
			method string
		)
		switch v := t.(type) {
		case depositTask:
			method = "requestMint"
			key = append([]byte{DepositPrefix}, big.NewInt(int64(v.requestId)).Bytes()...)
		case validatorsDesignateTask:
			method = "syncValidators"
			key = []byte{ValidatorsKey}
		case stateValidatorsChangeTask:
			method = "syncStateRootValidatorsAddress"
			key = make([]byte, 5)
			key[0] = StateValidatorRole
			binary.BigEndian.PutUint32(key[1:], v.index)
		default:
			panic("unkown task")
		}
		txproof, err := proveTx(batch.block, t.TxId())
		if err != nil {
			panic(fmt.Errorf("can't build tx proof: %w", err))
		}
		stateproof := l.client.GetProof(stateroot.RootHash, l.config.MainContract, crypto.Base64Encode(key))
		tx, err := l.invokeStateSync(method, uint32(batch.block.Index), t.TxId(), txproof, stateproof)
		if err != nil {
			return err
		}
		transactions = append(transactions, tx)
	}
	return l.commitTransactions(transactions)
}

func (l *Listener) invokeSyncObject(method string, object []byte) (*types.Transaction, error) {
	data, err := l.connector.Abi.Pack(method, object)
	if err != nil {
		return nil, err
	}
	return l.createEthLayerTransaction(data)
}

func (l *Listener) invokeStateSync(method string, index uint32, txid *helper.UInt256, txproof []byte, stateproof []byte) (*types.Transaction, error) {
	data, err := l.connector.Abi.Pack(method, index, big.NewInt(0).SetBytes(txid.ToByteArray()), txproof, stateproof)
	if err != nil {
		return nil, err
	}
	return l.createEthLayerTransaction(data)
}

func (l *Listener) createEthLayerTransaction(data []byte) (*types.Transaction, error) {
	var err error
	chainId := l.client.Eth_ChainId()
	gasPrice := l.client.Eth_GasPrice()
	nonce := l.client.Eth_GetTransactionCount(l.account.Address)
	ltx := &types.LegacyTx{
		Nonce:    nonce,
		To:       &(l.connector.Address),
		GasPrice: gasPrice,
		Value:    big.NewInt(0),
		Data:     data,
	}
	tx := &transaction.EthTx{
		Transaction: *types.NewTx(ltx),
	}
	gas, err := l.client.Eth_EstimateGas(&result.TransactionObject{
		From:     l.account.Address,
		To:       tx.To(),
		GasPrice: tx.GasPrice(),
		Value:    tx.Value(),
		Data:     tx.Data(),
	})
	if err != nil {
		return nil, err
	}
	ltx.Gas = gas
	tx.Transaction = *types.NewTx(ltx)
	err = l.account.SignTx(chainId, transaction.NewTx(tx))
	if err != nil {
		return nil, fmt.Errorf("can't sign tx: %w", err)
	}
	return &tx.Transaction, nil
}

func (l *Listener) commitTransactions(transactions []*types.Transaction) error {
	hashes := make(map[common.Hash]bool, len(transactions))
	for _, tx := range transactions {
		b, err := tx.MarshalBinary()
		if err != nil {
			return err
		}
		h, err := l.client.Eth_SendRawTransaction(b)
		if err != nil {
			return err
		}
		hashes[h] = false
	}
	retry := 3
	appending := []string{}
	for retry > 0 {
		time.Sleep(BlockTimeSeconds * time.Second)
		appending = appending[:0]
		for h, s := range hashes {
			if !s {
				txResp := l.client.Eth_GetTransactionByHash(h)
				if txResp != nil {
					hashes[h] = true
				} else {
					appending = append(appending, h.String())
				}
			}
		}
		if len(appending) == 0 {
			return nil
		}
	}
	return fmt.Errorf("can't commit transactions: [%s]", strings.Join(appending, ","))
}

type taskBatch struct {
	block   *models.RpcBlock
	isJoint bool
	tasks   []task
}

func (b *taskBatch) addTask(t task) {
	b.tasks = append(b.tasks, t)
}

type task interface {
	TxId() *helper.UInt256
}

type depositTask struct {
	txid      *helper.UInt256
	requestId uint64
}

func (t depositTask) TxId() *helper.UInt256 {
	return t.txid
}

type validatorsDesignateTask struct {
	txid *helper.UInt256
}

func (t validatorsDesignateTask) TxId() *helper.UInt256 {
	return t.txid
}

type stateValidatorsChangeTask struct {
	txid  *helper.UInt256
	index uint32
}

func (t stateValidatorsChangeTask) TxId() *helper.UInt256 {
	return t.txid
}
