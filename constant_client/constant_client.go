package constantclient

import (
	"github.com/joeqian10/neo3-gogogo/mpt"
	"github.com/joeqian10/neo3-gogogo/rpc/models"
)

type ConstantClient struct {
	seeds []string
}

func New(seeds []string) *ConstantClient {
	return &ConstantClient{
		seeds: seeds,
	}
}

func (c *ConstantClient) GetApplicationLog(txid string) *models.RpcApplicationLog {
	return nil
}

func (c *ConstantClient) GetBlock(index uint32) *models.RpcBlock {
	return nil
}

func (c *ConstantClient) GetStateRoot(index uint32) *mpt.StateRoot {
	return nil
}

func (c *ConstantClient) GetProof(rootHash, contractHash, key string) string {
	return ""
}
