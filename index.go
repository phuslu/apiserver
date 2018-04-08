package main

import (
	"fmt"

	"github.com/valyala/fasthttp"
)

func Index(ctx *fasthttp.RequestCtx) {
	host := ctx.Host()
	fmt.Fprintf(ctx, `Ipinfo lookup:

Usage:
    curl -v -d '{"ip": "1.1.1.1", "token": "42"}' http://%s/ipinfo

`, host)
}
