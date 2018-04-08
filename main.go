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
	"sync"
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
	StartWatchDog()

	var err error

	glog.DailyRolling = true
	glog.Version = version
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Println(version)
		return
	}

	var validate bool
	flag.BoolVar(&validate, "validate", false, "parse the apiserver toml and exit")

	if !HasString(os.Args, "-log_dir") {
		flag.Set("logtostderr", "true")
	}

	flag.Parse()

	config, err := NewConfig(flag.Arg(0))
	if err != nil {
		glog.Fatals().Err(err).Str("filename", flag.Arg(0)).Msg("NewConfig(..) error")
	}
	go config.Watcher()

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
		RateLimit:    config.Ipinfo.Ratelimit,
		Config:       config,
	}

	router := fasthttprouter.New()
	router.GET("/", Index)
	router.GET("/metrics", Metrics)
	router.GET("/debug/pprof/*profile", Pprof)
	router.POST("/ipinfo", ipinfo.Ipinfo)

	an := Announcer{
		FastOpen:    config.Default.TcpFastopen,
		ReusePort:   true,
		DeferAccept: true,
	}

	ln, err := an.Listen("tcp", config.Default.ListenAddr)
	if err != nil {
		glog.Fatals().Err(err).Str("listen_addr", config.Default.ListenAddr).Msg("TLS Listen(...) error")
	}

	server := &fasthttp.Server{
		Handler: router.Handler,
		Name:    "apiserver",
	}

	glog.Infos().Str("version", version).Str("listen_addr", ln.Addr().String()).Msg("apiserver ListenAndServe")
	go server.Serve(ln)

	glog.Flush()

	if validate {
		os.Exit(0)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM)
	signal.Notify(c, syscall.SIGINT)
	signal.Notify(c, syscall.SIGHUP)

	switch <-c {
	case syscall.SIGTERM, syscall.SIGINT:
		glog.Infos().Msg("apiserver flush logs and exit.")
		glog.Flush()
		os.Exit(0)
	}

	glog.Warnings().Msg("apiserver start graceful shutdown...")
	glog.Flush()

	SetProcessName("apiserver: (graceful shutdown)")

	timeout := 5 * time.Minute
	if config.reload() == nil {
		if config.Default.GracefulTimeout > 0 {
			timeout = time.Duration(config.Default.GracefulTimeout) * time.Second
		}
	}

	var wg sync.WaitGroup
	go func(server *fasthttp.Server, ln net.Listener) {
		wg.Add(1)
		defer wg.Done()

		ln.Close()

		time.Sleep(timeout)
	}(server, ln)
	wg.Wait()

	glog.Infos().Msg("apiserver server shutdown")
	glog.Flush()
}
