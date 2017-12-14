package main

import (
	"net/http/pprof"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

var (
	pprofCmdline = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Cmdline)
	pprofProfile = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Profile)
	pprofSymbol  = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Symbol)
	pprofTrace   = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Trace)
	pprofIndex   = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Index)
)

func Pprof(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Content-Type", "text/html")
	switch string(ctx.Path()) {
	case "/debug/pprof/cmdline":
		pprofCmdline(ctx)
	case "/debug/pprof/profile":
		pprofProfile(ctx)
	case "/debug/pprof/symbol":
		pprofSymbol(ctx)
	case "/debug/pprof/trace":
		pprofTrace(ctx)
	default:
		pprofIndex(ctx)
	}
}
