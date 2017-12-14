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
	Googleplay struct {
		SearchUrl   string
		SearchRegex string
		SearchTtl   int
	}
	Ipinfo struct {
		Url      string
		Regex    string
		CacheTtl int
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

	return &config, nil
}
