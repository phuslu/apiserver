package main

import (
	"github.com/aerospike/aerospike-client-go"
	"github.com/phuslu/glog"
	"github.com/valyala/fasthttp"
)

type BidHandler struct {
	AeroSpike *aerospike.Client
	Config    *Config
}

type BidRequest struct {
	Token string `json:"token"`
}

type BidResponse struct {
	Error string      `json:"error,omitempty"`
	Info  interface{} `json:"info"`
}

func (h *BidHandler) Error(ctx *fasthttp.RequestCtx, err error) {
	json.NewEncoder(ctx).Encode(BidResponse{
		Error: err.Error(),
	})
}

func (h *BidHandler) Bid(ctx *fasthttp.RequestCtx) {
	glog.S(2).Str("remote_addr", ctx.RemoteAddr().String()).Bytes("method", ctx.Method()).Str("url", ctx.URI().String()).Bytes("user_agent", ctx.UserAgent())

	var req BidRequest

	err := json.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		h.Error(ctx, err)
		return
	}

	json.NewEncoder(ctx).Encode(BidResponse{
		Error: "",
		Info:  h.AeroSpike.GetNodes(),
	})
}
