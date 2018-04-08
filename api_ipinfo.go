package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"
)

type IpinfoLimiterKey struct {
	Token string
}

type IpinfoHandler struct {
	URL          string
	Regex        *regexp.Regexp
	Cache        lrucache.Cache
	CacheTTL     time.Duration
	Singleflight *singleflight.Group
	Transport    *http.Transport
	RateLimit    int
	Config       *Config

	m sync.Map // map[LimiterKey]*rate.Limiter
}

type IpinfoRequest struct {
	IP    string `json:"ip"`
	Token string `json:"token"`
}

type IpinfoResponse struct {
	Error    string `json:"error,omitempty"`
	Location string `json:"location,omitempty"`
	ISP      string `json:"isp,omitempty"`
}

func (h *IpinfoHandler) Error(ctx *fasthttp.RequestCtx, err error) {
	json.NewEncoder(ctx).Encode(IpinfoResponse{
		Error: err.Error(),
	})
}

func (h *IpinfoHandler) Ipinfo(ctx *fasthttp.RequestCtx) {
	glog.S(2).Str("remote_addr", ctx.RemoteAddr().String()).Bytes("method", ctx.Method()).Str("url", ctx.URI().String()).Bytes("user_agent", ctx.UserAgent())

	var req IpinfoRequest

	err := json.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		h.Error(ctx, err)
		return
	}

	limitKey := IpinfoLimiterKey{
		Token: req.Token,
	}

	v, ok := h.m.Load(limitKey)
	if !ok {
		v, _ = h.m.LoadOrStore(limitKey, rate.NewLimiter(rate.Limit(h.RateLimit), h.RateLimit))
	}

	limiter := v.(*rate.Limiter)
	if !limiter.Allow() {
		h.Error(ctx, fmt.Errorf("limitKey=%#v over limit", limitKey))
		return
	}

	var item *IpinfoItem

	key := "ipinfo:" + req.IP
	if v, ok := h.Cache.GetNotStale(key); ok {
		item = v.(*IpinfoItem)
	} else {
		item, err = h.ipinfoSearch(req.IP)
		if err != nil {
			h.Error(ctx, err)
			return
		}

		h.Cache.Set(key, item, time.Now().Add(h.CacheTTL))
	}

	json.NewEncoder(ctx).Encode(IpinfoResponse{
		Error:    "",
		Location: item.Location,
		ISP:      item.ISP,
	})
}

type IpinfoItem struct {
	Location string
	ISP      string
}

func (h *IpinfoHandler) ipinfoSearch(ipStr string) (*IpinfoItem, error) {
	url := strings.Replace(h.URL, "%s", ipStr, 1)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "curl/7.56.0")

	v, err, _ := h.Singleflight.Do(url, func() (interface{}, error) {
		return h.Transport.RoundTrip(req)
	})
	if err != nil {
		return nil, err
	}

	resp := v.(*http.Response)
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	match := h.Regex.FindStringSubmatch(string(data))
	if match == nil {
		return nil, fmt.Errorf("empty")
	}

	item := &IpinfoItem{
		Location: match[1],
		ISP:      match[2],
	}

	glog.Infos().Str("ip", ipStr).Msgf("ipinfoSearch(...) return %+v", item)

	h.Singleflight.Forget(url)

	return item, nil
}
