package main

import (
	"fmt"

	"github.com/valyala/fasthttp"
)

func Index(ctx *fasthttp.RequestCtx) {
	host := ctx.Host()
	fmt.Fprintf(ctx, `Ipinfo lookup:

Usage:
    curl -v http://%s/ipinfo/127.0.0.1
    curl -v -d '{"title": "WhatsApp Messenger", "geo": "IN"}' http://%s/lookup-title
    curl -v -d '{"pkg_name": "com.whatsapp", "geo": "IN"}' http://%s/lookup-pkgname

`, host, host, host)
}
