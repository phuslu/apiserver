package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/buaazp/fasthttprouter"
	"github.com/cloudflare/golibs/lrucache"
	"github.com/json-iterator/go"
	"github.com/phuslu/glog"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/singleflight"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var (
	version = "r9999"
)

func main() {
	var err error

	glog.Single = true
	rand.Seed(time.Now().UnixNano())
	OLDPWD, _ := os.Getwd()

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println(version)
		return
	}

	if !HasString(os.Args, "-log_dir") {
		flag.Set("logtostderr", "true")
	}

	flag.Parse()

	config, err := NewConfig(flag.Arg(0))
	if err != nil {
		glog.Fatalf("NewConfig(%#v) error: %+v", flag.Arg(0), err)
	}

	// see http.DefaultTransport
	dialer := &TCPDialer{
		Resolver: &Resolver{
			Resolver: &net.Resolver{PreferGo: true},
			DNSCache: lrucache.NewLRUCache(8 * 1024),
			DNSTTL:   10 * time.Minute,
		},
		KeepAlive:             30 * time.Second,
		Timeout:               30 * time.Second,
		Level:                 1,
		PreferIPv6:            false,
		TLSClientSessionCache: tls.NewLRUClientSessionCache(2048),
	}

	// see http.DefaultTransport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			ClientSessionCache: tls.NewLRUClientSessionCache(2048),
		},
		Dial:                  dialer.Dial,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		Proxy:                 http.ProxyFromEnvironment,
	}

	ipinfo := &IpinfoHandler{
		URL:          config.Ipinfo.Url,
		Regex:        regexp.MustCompile(config.Ipinfo.Regex),
		CacheTTL:     time.Duration(config.Ipinfo.CacheTtl) * time.Second,
		Cache:        lrucache.NewLRUCache(10000),
		Singleflight: &singleflight.Group{},
		Transport:    transport,
	}

	googleplay := &LookupHandler{
		SearchURL:    config.Googleplay.SearchUrl,
		SearchRegex:  regexp.MustCompile(config.Googleplay.SearchRegex),
		SearchTTL:    time.Duration(config.Googleplay.SearchTtl) * time.Second,
		SearchCache:  lrucache.NewLRUCache(10000),
		Singleflight: &singleflight.Group{},
		Transport:    transport,
	}

	router := fasthttprouter.New()
	router.GET("/", Index)
	router.GET("/metrics", Metrics)
	router.GET("/debug/pprof/*profile", Pprof)
	router.GET("/ipinfo/:ip", ipinfo.Ipinfo)
	router.POST("/lookup-title", googleplay.LookupTitle)
	router.POST("/lookup-pkgname", googleplay.LookupPackageName)

	ln, err := ReusePortListen("tcp", config.Default.ListenAddr)
	if err != nil {
		glog.Fatalf("TLS Listen(%s) error: %s", config.Default.ListenAddr, err)
	}

	glog.Infof("apiserver %s ListenAndServe on %s\n", version, ln.Addr().String())
	go fasthttp.Serve(ln, router.Handler)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	switch <-c {
	case syscall.SIGHUP:
	default:
		glog.Infof("apiserver server closed.")
		os.Exit(0)
	}

	glog.Infof("apiserver flush logs")
	glog.Flush()

	glog.Infof("apiserver start new child server")
	exe, err := os.Executable()
	if err != nil {
		glog.Fatalf("os.Executable() error: %+v", exe)
	}

	_, err = os.StartProcess(exe, os.Args, &os.ProcAttr{
		Dir:   OLDPWD,
		Env:   os.Environ(),
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		glog.Fatalf("os.StartProcess(%+v, %+v) error: %+v", exe, os.Args, err)
	}

	glog.Warningf("apiserver start graceful shutdown...")
	SetProcessName("apiserver: (graceful shutdown)")

	timeout := 5 * time.Minute
	if config.Default.GracefulTimeout > 0 {
		timeout = time.Duration(config.Default.GracefulTimeout) * time.Second
	}

	if err := ln.Close(); err != nil {
		glog.Errorf("%T.Shutdown() error: %+v", ln, err)
	}

	time.Sleep(timeout)

	glog.Infof("apiserver server shutdown.")
}
