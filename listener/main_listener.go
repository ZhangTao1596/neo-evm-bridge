package listener

import (
	"errors"
	"fmt"
	"math/big"
	"neo-evm-bridge/config"
	"strconv"
	"time"

	"github.com/ZhangTao1596/neo-evm-bridge/constantclient"

	"github.com/joeqian10/neo3-gogogo/crypto"
	"github.com/joeqian10/neo3-gogogo/helper"
	"github.com/joeqian10/neo3-gogogo/mpt"
	"github.com/joeqian10/neo3-gogogo/rpc"
	"github.com/joeqian10/neo3-gogogo/rpc/models"
	"github.com/joeqian10/neo3-gogogo/vm"
	"github.com/neo-ngd/neo-go/pkg/core/transaction"
)

const (
	DepositPrefix          = 0x01
	ValidatorsKey          = 0x03
	StateValidatorRole     = 4
	RoleManagementContract = "0x49cf4e5378ffcd4dec034fd98a174c5491e395e2"
)

var ()

type Listener struct {
	config                        *config.Config
	evmLayerContract              helper.UInt160
	running                       bool
	lastHeader                    *models.RpcBlockHeader
	txPool                        []*transaction.Transaction
	roleManagementContractAddress *helper.UInt160
	client                        *constantclient.ConstantClient
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
	return &Listener{
		config:                        config,
		evmLayerContract:              *c,
		roleManagementContractAddress: roleManagement,
	}, nil
}

func (l *Listener) Stop() {
	l.running = false
}

func (l *Listener) Listen() {
	const startIndex = 0
	l.client = l.newRpcClient()
	for i := startIndex; ; {
		if !l.running {
			break
		}
		blockResp := l.client.GetBlock(fmt.Sprintf("%d", i))
		if needRetry(blockResp.ErrorResponse) {
			time.Sleep(time.Second)
			//retry
			continue
		}
		if isBlockUnreached(blockResp) {
			time.Sleep(15 * time.Second)
			continue
		}
		block := blockResp.Result
		batch := new(taskBatch)
		batch.block = &block
		batch.isJoint = l.isJointHeader(&block.RpcBlockHeader)
		for _, tx := range block.Tx {
			txid, err := helper.UInt256FromString(tx.Hash)
			if err != nil {
				panic(fmt.Errorf("invalid tx id: %w", err))
			}
			applicationlog := l.client.GetApplicationLog(tx.Hash)
			for needRetry(applicationlog.ErrorResponse) {
				time.Sleep(time.Second)
				//retry
				applicationlog = l.client.GetApplicationLog(tx.Hash)
			}
			for _, execution := range applicationlog.Result.Executions {
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

func (l *Listener) newRpcClient() *rpc.RpcClient {
	client := rpc.NewClient("http://localhost:11332")
	if client == nil {
		//try another
	}
	return client
}

func (l *Listener) saveHandled() error {
	return nil
}

func (l *Listener) isJointHeader(header *models.RpcBlockHeader) bool {
	return false
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
	if batch.isJoint || len(batch.tasks) > 0 {
		//sync header, synced?
	}
	var stateroot *mpt.StateRoot
	if len(batch.tasks) > 0 {
		staterootResp := l.client.GetStateRoot(uint32(batch.block.Index))
		stateroot = &staterootResp.Result
		//sync state root synced?
	}
	//send header and stateroot tx and wait a block
	for _, t := range batch.tasks {
		switch v := t.(type) {
		case depositTask:
			index := batch.block.Index
			txid := v.txid
			txProof, err := proveTx(batch.block, txid)
			if err != nil {
				panic(fmt.Errorf("can't build tx proof: %w", err))
			}
			key := append([]byte{DepositPrefix}, big.NewInt(int64(v.requestId)).Bytes()...)
			proofResp := l.client.GetProof(stateroot.RootHash, l.config.MainContract, crypto.Base64Encode(key))
			if needRetry(proofResp.ErrorResponse) {
				//retry
			} else if proofResp.HasError() {
				//panic?
			}

		case validatorsDesignateTask:
		case stateValidatorsChangeTask:
		default:
			panic("unkown task")
		}
	}
	return nil
}

func proveTx(block *models.RpcBlock, txid *helper.UInt256) ([]byte, error) {
	return nil, errors.New("unimplement")
}

func needRetry(eresp rpc.ErrorResponse) bool {
	return eresp.NetError != nil
}

func isBlockUnreached(resp rpc.GetBlockResponse) bool {
	return resp.Error.Code == -100 && resp.Error.Message == "Unknown block"
}

type taskBatch struct {
	block   *models.RpcBlock
	isJoint bool
	tasks   []interface{}
}

func (b *taskBatch) addTask(t interface{}) {
	b.tasks = append(b.tasks, t)
}

type depositTask struct {
	txid      *helper.UInt256
	requestId uint64
}

type validatorsDesignateTask struct {
	txid *helper.UInt256
}

type stateValidatorsChangeTask struct {
	txid  *helper.UInt256
	index uint32
}
