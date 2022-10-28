package relay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
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

type Relayer struct {
	config                        *config.Config
	evmLayerContract              helper.UInt160
	lastHeader                    *models.RpcBlockHeader
	roleManagementContractAddress *helper.UInt160
	client                        *constantclient.ConstantClient
	connector                     *state.NativeContract
	account                       *wallet.Account
}

func NewRelayer(config *config.Config, acc *wallet.Account) (*Relayer, error) {
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

	return &Relayer{
		config:                        config,
		evmLayerContract:              *c,
		roleManagementContractAddress: roleManagement,
		client:                        client,
		connector:                     connector,
		account:                       acc,
	}, nil
}

func (l *Relayer) Run() {
	for i := l.config.Start; ; {
		log.Printf("syncing block, index=%d", i)
		block := l.client.GetBlock(i)
		if block == nil {
			time.Sleep(15 * time.Second)
			continue
		}
		batch := new(taskBatch)
		batch.block = block
		batch.isJoint = l.isJointHeader(&block.RpcBlockHeader)
		for _, tx := range block.Tx {
			log.Printf("syncing tx, hash=%s\n", tx.Hash)
			txid, err := helper.UInt256FromString(tx.Hash)
			if err != nil {
				panic(fmt.Errorf("invalid tx id: %w", err))
			}
			applicationlog := l.client.GetApplicationLog(tx.Hash)
			for _, execution := range applicationlog.Executions {
				if execution.Trigger == "Application" && execution.VMState == "HALT" {
					for _, notification := range execution.Notifications {
						isDeposit, requestId, from, amount, to, err := l.isDepositEvent(&notification)
						if err != nil {
							panic(err)
						}
						if isDeposit {
							log.Printf("deposit event, id=%d, from=%s, amount=%d, to=%s\n", requestId, from, amount, to)
							batch.addTask(depositTask{
								txid:      txid,
								requestId: requestId,
							})
						} else {
							isDesignate, pks, err := l.isDesignateValidatorsEvent(&notification)
							if err != nil {
								panic(err)
							}
							if isDesignate {
								log.Printf("designate event, pks=%s\n", pks)
								batch.addTask(validatorsDesignateTask{
									txid: txid,
								})
							} else {
								isStateValidatorsDesignate, index, err := l.isStateValidatorsDesignatedEvent(&notification)
								if err != nil {
									panic(err)
								}
								if isStateValidatorsDesignate {
									log.Printf("state validators designate event, index=%d\n", index)
									batch.addTask(stateValidatorsChangeTask{
										txid:  txid,
										index: index,
									})
								}
							}
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

func (l *Relayer) saveHandled() error {
	return nil
}

func (l *Relayer) isJointHeader(header *models.RpcBlockHeader) bool {
	if l.lastHeader == nil && header.Index > 0 {
		block := l.client.GetBlock(uint32(header.Index) - 1)
		l.lastHeader = &block.RpcBlockHeader
	}
	return header.Index == 0 || l.lastHeader.NextConsensus != header.NextConsensus
}

func (l *Relayer) isRoleManagement(notification *models.RpcNotification) bool {
	contractInNotication, err := helper.UInt160FromString(notification.Contract)
	if err != nil {
		return false
	}
	return *contractInNotication == *l.roleManagementContractAddress
}

func (l *Relayer) isStateValidatorsDesignatedEvent(notification *models.RpcNotification) (bool, uint32, error) {
	if !l.isRoleManagement(notification) || notification.EventName != "Designation" {
		return false, 0, nil
	}
	if notification.State.Type != vm.Array.String() {
		return false, 0, errors.New("invalid role deposit event type")
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	if len(arr) != 2 {
		return false, 0, errors.New("invalid role deposite event arguments count")
	}
	role, err := strconv.Atoi(arr[0].Value.(string))
	if err != nil {
		return false, 0, fmt.Errorf("can't parse role: %w", err)
	}
	if role != StateValidatorRole {
		return false, 0, nil
	}
	index, err := strconv.ParseUint(arr[1].Value.(string), 10, 32)
	if err != nil {
		return false, 0, fmt.Errorf("can't parse index: %w", err)
	}
	return true, uint32(index), nil
}

func (l *Relayer) isDepositEvent(notification *models.RpcNotification) (isDeposit bool, requestId uint64, from helper.UInt160, amount int, to helper.UInt160, err error) {
	if !l.isEvmLayerContract(notification) || notification.EventName != "OnDeposited" {
		isDeposit = false
		return
	}
	if notification.State.Type != vm.Array.String() {
		err = errors.New("invalid deposited event type")
		return
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	if len(arr) != 4 {
		err = errors.New("invalid deposited event arguments count")
		return
	}
	requestId, err = strconv.ParseUint(arr[0].Value.(string), 10, 64)
	if err != nil {
		err = fmt.Errorf("can't parse request id: %w", err)
		return
	}
	if arr[1].Type != vm.ByteString.String() {
		err = errors.New("invalid from type in deposit event")
		return
	}
	bf, err := crypto.Base64Decode(arr[1].Value.(string))
	if err != nil {
		err = fmt.Errorf("can't parse from: %w", err)
		return
	}
	from = *helper.UInt160FromBytes(bf)
	if arr[2].Type != vm.Integer.String() {
		panic("invalid amount type in deposit event")
	}
	amount, err = strconv.Atoi(arr[2].Value.(string))
	if err != nil {
		err = fmt.Errorf("can't parse amount: %w", err)
		return
	}
	if arr[3].Type != vm.ByteString.String() {
		err = errors.New("invalid to type in deposit event")
		return
	}
	bt, err := crypto.Base64Decode(arr[3].Value.(string))
	if err != nil {
		err = fmt.Errorf("can't parse to: %w", err)
		return
	}
	to = *helper.UInt160FromBytes(bt)
	return true, requestId, from, amount, to, nil
}

func (l *Relayer) isDesignateValidatorsEvent(notification *models.RpcNotification) (isDesignated bool, pks []crypto.ECPoint, err error) {
	fmt.Println(notification)
	if !l.isEvmLayerContract(notification) || notification.EventName != "OnValidatorsChanged" {
		isDesignated = false
		return
	}
	if notification.State.Type != vm.Array.String() {
		err = errors.New("invalid designated event type")
		return
	}
	notification.State.Convert()
	arr := notification.State.Value.([]models.InvokeStack)
	pks = make([]crypto.ECPoint, len(arr))
	for i, p := range arr {
		if p.Type != vm.ByteString.String() {
			err = errors.New("invalid ecpoint type in deposit event")
			return
		}
		pt, e := crypto.NewECPointFromBytes(p.Value.([]byte))
		if err != nil {
			err = fmt.Errorf("can't parse ecpoint: %w", e)
			return
		}
		pks[i] = *pt
	}
	return true, pks, nil
}

func (l *Relayer) isEvmLayerContract(notification *models.RpcNotification) bool {
	contractInNotication, err := helper.UInt160FromString(notification.Contract)
	if err != nil {
		return false
	}
	return *contractInNotication == l.evmLayerContract
}

func (l *Relayer) sync(batch *taskBatch) error {
	transactions := []*types.Transaction{}
	if batch.isJoint || len(batch.tasks) > 0 {
		header, err := rpcHeaderToBlockHeader(batch.block.RpcBlockHeader)
		fmt.Println(1.1)
		if err != nil {
			return err
		}
		b, err := blockHeaderToBytes(header)
		if err != nil {
			return fmt.Errorf("can't encode block header: %w", err)
		}
		tx, err := l.invokeSyncObject("syncHeader", b)
		fmt.Println(1.3)
		if err != nil {
			return fmt.Errorf("can't sync object: %w", err)
		}
		transactions = append(transactions, tx)
	}
	var stateroot *mpt.StateRoot
	if len(batch.tasks) > 0 {
		stateroot = l.client.GetStateRoot(uint32(batch.block.Index))
		b, err := staterootToBytes(stateroot)
		if err != nil {
			return fmt.Errorf("can't encode stateroot: %w", err)
		}
		tx, err := l.invokeSyncObject("syncStateRoot", b)
		if err != nil {
			return fmt.Errorf("can't sync object: %w", err)
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
			return errors.New("unkown task")
		}
		txproof, err := proveTx(batch.block, t.TxId())
		if err != nil {
			return fmt.Errorf("can't build tx proof: %w", err)
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

func (l *Relayer) invokeSyncObject(method string, object []byte) (*types.Transaction, error) {
	data, err := l.connector.Abi.Pack(method, object)
	if err != nil {
		return nil, fmt.Errorf("can't pack sync object, method=%s: %w", method, err)
	}
	return l.createEthLayerTransaction(data)
}

func (l *Relayer) invokeStateSync(method string, index uint32, txid *helper.UInt256, txproof []byte, stateproof []byte) (*types.Transaction, error) {
	data, err := l.connector.Abi.Pack(method, index, big.NewInt(0).SetBytes(txid.ToByteArray()), txproof, stateproof)
	if err != nil {
		return nil, err
	}
	return l.createEthLayerTransaction(data)
}

func (l *Relayer) createEthLayerTransaction(data []byte) (*types.Transaction, error) {
	var err error
	chainId := l.client.Eth_ChainId()
	gasPrice := l.client.Eth_GasPrice()
	fmt.Println(l.connector.Address)
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

func (l *Relayer) commitTransactions(transactions []*types.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}
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
