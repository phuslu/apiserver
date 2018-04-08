package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/naoina/toml"
	"github.com/phuslu/glog"
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
	Bid struct {
		AerospikeHost string
		AerospikePort int
	}
}

func (c *Config) reload() error {
	tomlData, err := ioutil.ReadFile(c.uri)
	if err != nil {
		return err
	}

	if err = toml.Unmarshal(tomlData, &c); err != nil {
		return fmt.Errorf("toml.Decode(%s) error: %+v", tomlData, err)
	}

	return nil
}

func (c *Config) Watcher() {
	filename := c.uri
	if strings.Contains(filename, "://") {
		glog.Warnings().Str("filename", filename).Msg("config file path not supportted by fsnotify")
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatals().Err(err).Msg("fsnotify.NewWatcher() error")
	}
	defer watcher.Close()

	err = watcher.Add(filename)
	if err != nil {
		glog.Fatals().Err(err).Str("filename", filename).Msg("watcher.Add(...) error: %+v")
		return
	}

	glog.Infos().Str("filename", filename).Msg("fsnotify add file to watch list")

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				glog.Infos().Str("filename", filename).Str("event_name", event.Name).Msg("modified config file")
				if c.reload() == nil {
					glog.Infos().Str("filename", filename).Msgf("%#v", c)
				}
			}
		case err := <-watcher.Errors:
			glog.Errors().Err(err).Msg("watch config file error")
		}
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

	c := &Config{uri: filename}
	if err := c.reload(); err != nil {
		return nil, fmt.Errorf("load config from %#v error: %+v", filename, err)
	}

	return c, nil
}
