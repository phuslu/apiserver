package main

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
	"github.com/valyala/fasthttp"
	"golang.org/x/sync/singleflight"
)

type LookupHandler struct {
	SearchURL    string
	SearchRegex  *regexp.Regexp
	SearchTTL    time.Duration
	SearchCache  lrucache.Cache
	Singleflight *singleflight.Group
	Transport    *http.Transport
}

type LookupRequest struct {
	PackageName string `json:"pkg_name"`
	Title       string `json:"title"`
	GEO         string `json:"geo"`
}

type LookupResponse struct {
	Status      int    `json:"status"`
	Error       string `json:"error,omitempty""`
	PackageName string `json:"pkg_name,omitempty"`
	Title       string `json:"title,omitempty"`
	GEO         string `json:"geo,omitempty"`
}

func (h *LookupHandler) Error(ctx *fasthttp.RequestCtx, err error) {
	json.NewEncoder(ctx).Encode(LookupResponse{
		Status: 204,
		Error:  err.Error(),
	})
}

func (h *LookupHandler) LookupTitle(ctx *fasthttp.RequestCtx) {
	if glog.V(2) {
		glog.Infof("%s \"%s %s\" \"%s\"", ctx.RemoteAddr(), ctx.Method(), ctx.URI(), ctx.UserAgent())
	}

	var req LookupRequest
	var pkgName string

	err := json.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		h.Error(ctx, err)
		return
	}

	key := "title:" + req.Title + ":" + req.GEO
	if v, ok := h.SearchCache.GetNotStale(key); ok {
		pkgName = v.(string)
	} else {
		items, err := h.googleplaySearch(url.PathEscape(req.Title), req.GEO)
		if err != nil {
			h.Error(ctx, err)
			return
		}

		for _, item := range items {
			if item.Title == req.Title {
				pkgName = item.PackageName
				h.SearchCache.Set(key, pkgName, time.Now().Add(h.SearchTTL))
				break
			}
		}
	}

	status := 200
	if pkgName == "" {
		status = 204
	}

	json.NewEncoder(ctx).Encode(LookupResponse{
		Status:      status,
		PackageName: pkgName,
	})
}

func (h *LookupHandler) LookupPackageName(ctx *fasthttp.RequestCtx) {
	if glog.V(2) {
		glog.Infof("%s \"%s %s\" \"%s\"", ctx.RemoteAddr(), ctx.Method(), ctx.URI(), ctx.UserAgent())
	}

	var req LookupRequest
	var title string

	err := json.Unmarshal(ctx.PostBody(), &req)
	if err != nil {
		h.Error(ctx, err)
		return
	}

	key := "pkgname:" + req.PackageName + ":" + req.GEO
	if v, ok := h.SearchCache.GetNotStale(key); ok {
		title = v.(string)
	} else {
		items, err := h.googleplaySearch(req.PackageName, req.GEO)
		if err != nil {
			h.Error(ctx, err)
			return
		}

		for _, item := range items {
			if item.PackageName == req.PackageName {
				title = item.Title
				h.SearchCache.Set(key, title, time.Now().Add(h.SearchTTL))
				break
			}
		}
	}

	status := 200
	if title == "" {
		status = 204
	}

	json.NewEncoder(ctx).Encode(LookupResponse{
		Status: status,
		Title:  title,
	})
}

type GoogleplaySearchItem struct {
	PackageName string
	Title       string
}

func (h *LookupHandler) googleplaySearch(query, lang string) ([]GoogleplaySearchItem, error) {
	if v, ok := h.SearchCache.GetNotStale(query + lang); ok {
		return v.([]GoogleplaySearchItem), nil
	}

	url := strings.Replace(h.SearchURL, "%s", query, 1)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept-Language", strings.ToLower(lang)+";q=0.9,en-US;q=0.8,en;q=0.7")

	v, err, _ := h.Singleflight.Do(url+lang, func() (interface{}, error) {
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

	matches := h.SearchRegex.FindAllStringSubmatch(string(data), -1)

	items := make([]GoogleplaySearchItem, 0)
	for _, group := range matches {
		name, title := group[1], group[2]
		items = append(items, GoogleplaySearchItem{name, title})
	}

	glog.Infof("googleplaySearch(%#v, %#v) return %d items", query, lang, len(items))

	h.SearchCache.Set(query+lang, items, time.Now().Add(h.SearchTTL))
	h.Singleflight.Forget(url + lang)

	return items, nil
}
