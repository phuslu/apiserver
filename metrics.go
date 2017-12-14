package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

type MetricsFooKey struct {
	Key1 string
	Key2 string
}

var (
	MetricsFooCounter sync.Map // map[MetricsFooKey]*int64
)

func Metrics(ctx *fasthttp.RequestCtx) {
	var w io.Writer = ctx
	if ctx.Request.Header.HasAcceptEncoding("gzip") {
		gz := gzip.NewWriter(ctx)
		defer gz.Close()
		w = gz
		ctx.Response.Header.Set("Content-Encoding", "gzip")
	}

	ctx.Response.Header.Set("Content-Type", "text/plain")
	ctx.SetStatusCode(fasthttp.StatusOK)

	io.WriteString(w, "# HELP apiserver_foo_count transmit bytes\n")
	io.WriteString(w, "# TYPE apiserver_foo_count gauge\n")
	MetricsFooCounter.Range(func(key, value interface{}) bool {
		k := key.(MetricsFooKey)
		v := atomic.LoadInt64(value.(*int64))
		fmt.Fprintf(w, "apiserver_foo_count{key1=\"%s\",key2=\"%s\"} %d\n", k.Key1, k.Key2, v)
		return true
	})
}
