package constantclient

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joeqian10/neo3-gogogo/crypto"
	"github.com/joeqian10/neo3-gogogo/mpt"
	"github.com/joeqian10/neo3-gogogo/rpc"
	"github.com/joeqian10/neo3-gogogo/rpc/models"
	"github.com/neo-ngd/neo-go/pkg/core/state"
	"github.com/neo-ngd/neo-go/pkg/rpc/client"
	"github.com/neo-ngd/neo-go/pkg/rpc/response"
	"github.com/neo-ngd/neo-go/pkg/rpc/response/result"
)

type ConstantClient struct {
	mainSeeds []string
	sideSeeds []string
	mIndex    int
	sIndex    int
	mClient   *rpc.RpcClient
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
	c.newClient(true)
	c.newClient(false)
	return c
}

func (c *ConstantClient) newClient(m bool) {
	if m {
		c.mClient = rpc.NewClient(c.mainSeeds[c.mIndex])
		c.mIndex = (c.mIndex + 1) % len(c.mainSeeds)
	} else {
		sIndex := c.sIndex
		sclient, err := client.New(context.Background(), c.sideSeeds[sIndex], client.Options{})
		c.sIndex = (c.sIndex + 1) % len(c.sideSeeds)
		for err != nil {
			sclient, err = client.New(context.Background(), c.sideSeeds[c.sIndex], client.Options{})
			c.sIndex = (c.sIndex + 1) % len(c.sideSeeds)
			if c.sIndex == sIndex {
				panic(fmt.Errorf("can't initialize side client"))
			}
		}
		c.sClient = sclient
	}
}

func (c *ConstantClient) ensureRequestMain(doRequest func(*rpc.RpcClient) (*rpc.ErrorResponse, interface{})) interface{} {
	eresp, r := doRequest(c.mClient)
	for eresp.NetError != nil {
		c.newClient(true)
		eresp, r = doRequest(c.mClient)
	}
	if len(eresp.Error.Message) > 0 && eresp.Error.Code != 0 {
		return nil
	}
	return r
}

func (c *ConstantClient) ensureRequestSide(doRequest func(*client.Client) (interface{}, error), resultExpected bool) (interface{}, error) {
	for {
		r, err := doRequest(c.sClient)
		if err != nil {
			respe, ok := err.(*response.Error)
			if resultExpected || !ok {
				c.newClient(false)
				continue
			} else {
				return nil, respe
			}
		}
		return r, nil
	}
}

func (c *ConstantClient) GetApplicationLog(txid string) *models.RpcApplicationLog {
	r := c.ensureRequestMain(func(client *rpc.RpcClient) (*rpc.ErrorResponse, interface{}) {
		resp := client.GetApplicationLog(txid)
		return &resp.ErrorResponse, &resp.Result
	})
	if r == nil {
		return nil
	}
	return r.(*models.RpcApplicationLog)
}

func isBlockUnreached(resp rpc.GetBlockResponse) bool {
	return resp.Error.Code == -100 && resp.Error.Message == "Unknown block"
}

func (c *ConstantClient) GetBlock(index uint32) *models.RpcBlock {
	sidx := strconv.FormatUint(uint64(index), 10)
	r := c.ensureRequestMain(func(client *rpc.RpcClient) (*rpc.ErrorResponse, interface{}) {
		resp := client.GetBlock(sidx)
		return &resp.ErrorResponse, &resp.Result
	})
	if r == nil {
		return nil
	}
	return r.(*models.RpcBlock)
}

func (c *ConstantClient) GetStateRoot(index uint32) *mpt.StateRoot {
	r := c.ensureRequestMain(func(client *rpc.RpcClient) (*rpc.ErrorResponse, interface{}) {
		resp := client.GetStateRoot(index)
		return &resp.ErrorResponse, &resp.Result
	})
	if r == nil {
		return nil
	}
	return r.(*mpt.StateRoot)
}

func (c *ConstantClient) GetProof(rootHash, contractHash, key string) []byte {
	r := c.ensureRequestMain(func(client *rpc.RpcClient) (*rpc.ErrorResponse, interface{}) {
		resp := client.GetProof(rootHash, contractHash, key)
		return &resp.ErrorResponse, &resp.Result
	})
	if r == nil {
		return nil
	}
	s, err := crypto.Base64Decode(*(r.(*string)))
	if err != nil {
		panic(fmt.Errorf("can't base64 decode proof: %w", err))
	}
	return s
}

func (c *ConstantClient) GetNativeContract(name string) *state.NativeContract {
	r, _ := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.GetNativeContracts()
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
	r, _ := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_ChainId()
	}, true)
	return r.(uint64)
}

func (c *ConstantClient) Eth_GasPrice() *big.Int {
	r, _ := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_GasPrice()
	}, true)
	return r.(*big.Int)
}

func (c *ConstantClient) Eth_GetTransactionCount(address common.Address) uint64 {
	r, _ := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_GetTransactionCount(address)
	}, true)
	return r.(uint64)
}

func (c *ConstantClient) Eth_EstimateGas(tx *result.TransactionObject) (uint64, error) {
	r, err := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_EstimateGas(tx)
	}, false)
	if err != nil {
		return 0, err
	}
	return r.(uint64), nil
}

func (c *ConstantClient) Eth_SendRawTransaction(rawTx []byte) (common.Hash, error) {
	r, err := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_SendRawTransaction(rawTx)
	}, false)
	if err != nil {
		return common.Hash{}, err
	}
	return r.(common.Hash), nil
}

func (c *ConstantClient) Eth_GetTransactionByHash(hash common.Hash) *result.TransactionOutputRaw {
	r, _ := c.ensureRequestSide(func(client *client.Client) (interface{}, error) {
		return client.Eth_GetTransactionByHash(hash)
	}, true)
	return r.(*result.TransactionOutputRaw)
}
