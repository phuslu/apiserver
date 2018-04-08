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
		glog.Warningf("config file path(%#v) not supportted by fsnotify", filename)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatalf("fsnotify.NewWatcher() error: %+v", err)
	}
	defer watcher.Close()

	err = watcher.Add(filename)
	if err != nil {
		glog.Fatalf("watcher.Add(%#v) error: %+v", filename, err)
		return
	}

	glog.Infof("fsnotify add %#v to watch list", filename)

	for {
		select {
		case event := <-watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				glog.Infof("modified config file: %+v", event.Name)
				if c.reload() == nil {
					glog.Infof("new config=%#v", c)
				}
			}
		case err := <-watcher.Errors:
			glog.Errorf("watch config file error: %+v", err)
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
