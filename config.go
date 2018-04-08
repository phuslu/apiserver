package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/naoina/toml"
)

type Config struct {
	uri string

	Default struct {
		ListenAddr      string
		TcpFastopen     bool
		GracefulTimeout int
	}
	Ipinfo struct {
		Url       string
		Regex     string
		CacheTtl  int
		Ratelimit int
	}
}

func (c *Config) Reload() error {
	tomlData, err := ioutil.ReadFile(c.uri)
	if err != nil {
		return err
	}

	if err = toml.Unmarshal(tomlData, &c); err != nil {
		return fmt.Errorf("toml.Decode(%s) error: %+v", tomlData, err)
	}

	return nil
}

func NewConfig(filename string) (*Config, error) {
	if filename == "" {
		env := os.Getenv("GOLANG_ENV")
		if env == "" {
			env = "development"
		}
		filename = env + ".toml"
	}

	c := &Config{uri: filename}
	if err := c.Reload(); err != nil {
		return nil, fmt.Errorf("load config from %#v error: %+v", filename, err)
	}

	return c, nil
}
