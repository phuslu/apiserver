package main

import (
	"fmt"
	"sync"

	"github.com/valyala/fasthttp"
	"golang.org/x/time/rate"
)

type LimiterKey struct {
	ChannelID int
}

type LimiterHandler struct {
	Threshold int

	m sync.Map // map[LimiterKey]*rate.Limiter
}

type LimiterRequest struct {
	ChannelID int `json:"channel_id"`
}

type LimiterResponse struct {
	Status int    `json:"status"`
	Error  string `json:"error,omitempty""`
}

func (h *LimiterHandler) Error(ctx *fasthttp.RequestCtx, err error) {
	json.NewEncoder(ctx).Encode(LookupResponse{
		Status: 204,
		Error:  err.Error(),
	})
}

func (h *LimiterHandler) PubidLimiter(ctx *fasthttp.RequestCtx) {
	var req LimiterRequest

	err := json.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		h.Error(ctx, err)
		return
	}

	key := LimiterKey{
		ChannelID: req.ChannelID,
	}

	v, ok := h.m.Load(key)
	if !ok {
		v, _ = h.m.LoadOrStore(key, rate.NewLimiter(rate.Limit(h.Threshold), h.Threshold))
	}

	limiter := v.(*rate.Limiter)
	if !limiter.Allow() {
		h.Error(ctx, fmt.Errorf("key=%#v over limit", key))
		return
	}

	json.NewEncoder(ctx).Encode(LimiterResponse{
		Status: 200,
	})
}
