package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/naoina/toml"
)

type Config struct {
	Default struct {
		ListenAddr      string
		GracefulTimeout int
	}
	Ipinfo struct {
		Url      string
		Regex    string
		CacheTtl int
	}
	Limiter struct {
		Threshold int
	}
}

func NewConfig(filename string) (*Config, error) {
	if filename == "" {
		env := os.Getenv("GOLANG_ENV")
		if env == "" {
			env = "development"
		}
		filename = env + ".toml"
	}

	tomlData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadFile(%+v) error: %+v", filename, err)
	}

	var config Config
	if err = toml.Unmarshal(tomlData, &config); err != nil {
		return nil, fmt.Errorf("toml.Decode(%s) error: %+v", tomlData, err)
	}

	if config.Limiter.Threshold == 0 {
		config.Limiter.Threshold = 10000
	}

	return &config, nil
}
