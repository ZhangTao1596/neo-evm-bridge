package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joeqian10/neo3-gogogo/helper"
)

type Config struct {
	MainSeeds      []string       `json:"mainSeeds"`
	SideSeeds      []string       `json:"sideSeeds"`
	Start          uint32         `json:"start"`
	End            uint32         `json:"end"`
	ManageContract string         `json:"manageContract"`
	Wallet         string         `json:"wallet"`
	Relayer        common.Address `json:"relayer"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := new(Config)
	err = json.Unmarshal(b, cfg)
	if err != nil {
		return nil, err
	}
	err = cfg.check()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (cfg *Config) check() error {
	if len(cfg.MainSeeds) == 0 {
		return errors.New("missing seeds")
	}
	if len(cfg.SideSeeds) == 0 {
		return errors.New("missing seeds")
	}
	if !strings.HasPrefix(cfg.ManageContract, "0x") {
		return errors.New("invalid manage contract address")
	}
	_, err := helper.UInt160FromString(cfg.ManageContract)
	if err != nil {
		return fmt.Errorf("invalid manage contract address: %w", err)
	}
	return nil
}
