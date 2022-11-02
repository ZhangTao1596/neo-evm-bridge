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
	"golang.org/x/exp/maps"

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
	DepositPrefix                           = 0x01
	ValidatorsKey                           = 0x03
	StateValidatorRole                      = 4
	BlockTimeSeconds                        = 15
	MaxStateRootTryCount                    = 1000
	MintThreshold                           = 100000000
	RoleManagementContract                  = "0x49cf4e5378ffcd4dec034fd98a174c5491e395e2"
	ConnectorContractName                   = "Connector"
	ConnectorSyncHeader                     = "syncHeader"
	ConnectorSyncStateRoot                  = "syncStateRoot"
	ConnectorSyncValidators                 = "syncValidators"
	ConnectorSyncStateRootValidatorsAddress = "syncStateRootValidatorsAddress"
	ConnectorRequestMint                    = "requestMint"
	ConnectorAlreadySyncedError             = "already synced"
)

type Relayer struct {
	cfg                           *config.Config
	lastHeader                    *models.RpcBlockHeader
	lastStateRoot                 *mpt.StateRoot
	roleManagementContractAddress *helper.UInt160
	client                        *constantclient.ConstantClient
	connector                     *state.NativeContract
	account                       *wallet.Account
}

func NewRelayer(cfg *config.Config, acc *wallet.Account) (*Relayer, error) {
	roleManagement, err := helper.UInt160FromString(RoleManagementContract)
	if err != nil {
		return nil, err
	}
	client := constantclient.New(cfg.MainSeeds, cfg.SideSeeds)
	connector := client.GetNativeContract(ConnectorContractName)
	if connector == nil {
		return nil, errors.New("can't get connector contract")
	}
	return &Relayer{
		cfg:                           cfg,
		roleManagementContractAddress: roleManagement,
		client:                        client,
		connector:                     connector,
		account:                       acc,
	}, nil
}

func (l *Relayer) Run() {
	for i := l.cfg.Start; i < l.cfg.End; {
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
							if amount < MintThreshold {
								log.Printf("threshold unreached, id=%d, from=%s, amount=%d, to=%s\n", requestId, from, amount, to)
								continue
							}
							batch.addTask(depositTask{
								txid:      tx.Hash,
								requestId: requestId,
							})
						} else {
							isDesignate, pks, err := l.isDesignateValidatorsEvent(&notification)
							if err != nil {
								panic(err)
							}
							if isDesignate {
								log.Printf("validators designate event, pks=%s\n", pks)
								batch.addTask(validatorsDesignateTask{
									txid: tx.Hash,
								})
							} else {
								isStateValidatorsDesignate, index, err := l.isStateValidatorsDesignatedEvent(&notification)
								if err != nil {
									panic(err)
								}
								if isStateValidatorsDesignate {
									log.Printf("state validators designate event, index=%d\n", index)
									batch.addTask(stateValidatorsChangeTask{
										txid:  tx.Hash,
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
		l.lastHeader = &block.RpcBlockHeader
		i++
	}
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
	if !l.isManageContract(notification) || notification.EventName != "OnDeposited" {
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
	if !l.isManageContract(notification) || notification.EventName != "OnValidatorsChanged" {
		isDesignated = false
		return
	}
	if notification.State.Type != vm.Array.String() {
		err = errors.New("invalid designated event type")
		return
	}
	notification.State.Convert()
	args := notification.State.Value.([]models.InvokeStack)
	if len(args) != 1 {
		err = errors.New("invalid validators change arguments count")
		return
	}
	arr := args[0].Value.([]models.InvokeStack)
	pks = make([]crypto.ECPoint, len(arr))
	for i, p := range arr {
		if p.Type != vm.ByteString.String() {
			err = errors.New("invalid ecpoint type in validators change event")
			return
		}
		pkb, e := crypto.Base64Decode(p.Value.(string))
		if e != nil {
			err = fmt.Errorf("can't parse ecpoint base64: %w", e)
			return
		}
		pt, e := crypto.NewECPointFromBytes(pkb)
		if err != nil {
			err = fmt.Errorf("can't parse ecpoint: %w", e)
			return
		}
		pks[i] = *pt
	}
	return true, pks, nil
}

func (l *Relayer) isManageContract(notification *models.RpcNotification) bool {
	return notification.Contract == l.cfg.ManageContract
}

func (l *Relayer) sync(batch *taskBatch) error {
	transactions := []*types.Transaction{}
	if batch.isJoint || len(batch.tasks) > 0 {
		header, err := rpcHeaderToBlockHeader(batch.block.RpcBlockHeader)
		if err != nil {
			return err
		}
		b, err := blockHeaderToBytes(header)
		if err != nil {
			return fmt.Errorf("can't encode block header: %w", err)
		}
		tx, err := l.invokeSyncObject(ConnectorSyncHeader, b)
		if err != nil {
			if strings.Contains(err.Error(), ConnectorAlreadySyncedError) {
				log.Println("skip synced header")
			} else {
				return fmt.Errorf("can't %s, header=%s, h=%s,: %w", ConnectorSyncHeader, batch.block.Hash, header.Hash(), err)
			}
		} else {
			log.Printf("%s tx, txid=%s\n", ConnectorSyncHeader, batch.block.Hash)
			transactions = append(transactions, tx)
		}

	}
	var stateroot *mpt.StateRoot
	if len(batch.tasks) > 0 {
		if l.lastStateRoot != nil && l.lastStateRoot.Index >= batch.Index() {
			stateroot = l.lastStateRoot
		} else {
			for stateIndex := batch.Index(); stateIndex < batch.Index()+MaxStateRootTryCount; {
				stateroot = l.client.GetStateRoot(stateIndex)
				if stateroot == nil {
					return errors.New("can't get state root")
				}
				if len(stateroot.Witnesses) == 0 {
					log.Printf("unverified state root, index=%d", batch.Index())
					continue
				}
				l.lastStateRoot = stateroot
				b, err := staterootToBytes(stateroot)
				if err != nil {
					return fmt.Errorf("can't encode stateroot: %w", err)
				}
				tx, err := l.invokeSyncObject(ConnectorSyncStateRoot, b)
				if err != nil {
					if strings.Contains(err.Error(), ConnectorAlreadySyncedError) {
						log.Println("skip synced state root")
					} else {
						return fmt.Errorf("can't sync state root: %w", err)
					}
				} else {
					log.Printf("%s tx, txid=%s\n", ConnectorSyncStateRoot, tx.Hash())
					transactions = append(transactions, tx)
				}
			}
			if stateroot == nil {
				return errors.New("can't get verified state root")
			}
		}
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
			method = ConnectorRequestMint
			key = append([]byte{DepositPrefix}, big.NewInt(int64(v.requestId)).Bytes()...)
		case validatorsDesignateTask:
			method = ConnectorSyncValidators
			key = []byte{ValidatorsKey}
		case stateValidatorsChangeTask:
			method = ConnectorSyncStateRootValidatorsAddress
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
		stateproof := l.client.GetProof(stateroot.RootHash, l.cfg.ManageContract, crypto.Base64Encode(key))
		tx, err := l.invokeStateSync(method, batch.Index(), t.TxId(), txproof, stateroot.Index, stateproof)
		if err != nil {
			if strings.Contains(err.Error(), "already synced") {
				log.Printf("%s skip synced\n", method)
				continue
			}
			if method == ConnectorSyncValidators && strings.Contains(err.Error(), "synced validators outdated") {
				log.Printf("%s skip synced validators", method)
				continue
			}
			if method == ConnectorRequestMint && strings.Contains(err.Error(), "already minted") {
				log.Printf("%s skip synced mint", method)
				continue
			}
			return err
		}
		log.Printf("%s tx, txid=%s\n", method, tx.Hash())
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

func (l *Relayer) invokeStateSync(method string, index uint32, txid string, txproof []byte, rootIndex uint32, stateproof []byte) (*types.Transaction, error) {
	h, err := helper.UInt256FromString(txid)
	if err != nil {
		return nil, fmt.Errorf("can't parse txid: %w", err)
	}
	data, err := l.connector.Abi.Pack(method, index, big.NewInt(0).SetBytes(common.BytesToHash(h.ToByteArray()).Bytes()), txproof, rootIndex, stateproof)
	if err != nil {
		return nil, err
	}
	return l.createEthLayerTransaction(data)
}

func (l *Relayer) createEthLayerTransaction(data []byte) (*types.Transaction, error) {
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
			log.Printf("txes committed: %s\n", maps.Keys(hashes))
			return nil
		}
		retry--
	}
	return fmt.Errorf("can't commit transactions: [%s]", strings.Join(appending, ","))
}

type taskBatch struct {
	block   *models.RpcBlock
	isJoint bool
	tasks   []task
}

func (b taskBatch) Index() uint32 {
	return uint32(b.block.Index)
}

func (b *taskBatch) addTask(t task) {
	b.tasks = append(b.tasks, t)
}

type task interface {
	TxId() string
}

type depositTask struct {
	txid      string
	requestId uint64
}

func (t depositTask) TxId() string {
	return t.txid
}

type validatorsDesignateTask struct {
	txid string
}

func (t validatorsDesignateTask) TxId() string {
	return t.txid
}

type stateValidatorsChangeTask struct {
	txid  string
	index uint32
}

func (t stateValidatorsChangeTask) TxId() string {
	return t.txid
}
