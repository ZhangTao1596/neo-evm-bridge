package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MainSeeds    []string `json:"mainSeeds"`
	SideSeeds    []string `json:"sideSeeds"`
	Start        int      `json:"start"`
	ForceStart   bool     `json:"forceStart"`
	MainContract string   `json:"MainContract"`
	SideContract string   `json:"SideContract"`
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
