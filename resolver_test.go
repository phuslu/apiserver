package main

import (
	"context"
	"testing"

	"github.com/alecthomas/geoip"
	"github.com/cloudflare/golibs/lrucache"
)

func TestRegionResolver(t *testing.T) {
	var pairs = [][2]string{
		{"192.168.1.1", ""},
		{"localhost", ""},
		{"github.com", "US"},
		{"phus.lu", "CN"},
	}

	geo, err := geoip.New()
	if err != nil {
		t.Fatalf("geoip.New() error: %+v", err)
	}

	r := &RegionResolver{
		Resolver: &Resolver{},
		GeoIP:    geo,
		Cache:    lrucache.NewLRUCache(2048),
	}

	for _, pair := range pairs {
		host, country := pair[0], pair[1]
		if c, _ := r.LookupCountry(context.Background(), host); c != country {
			t.Errorf("LookupCountry(%#v) return %#v,  not match %#v", host, c, country)
		}
	}
}
