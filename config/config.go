package config

import (
	"encoding/json"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

type Config struct {
	MainSeeds    []string       `json:"mainSeeds"`
	SideSeeds    []string       `json:"sideSeeds"`
	Start        uint32         `json:"start"`
	MainContract string         `json:"ainContract"`
	Wallet       string         `json:"wallet"`
	Relayer      common.Address `json:"relayer"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := new(Config)
	err = json.Unmarshal(b, c)
	return c, err
}
