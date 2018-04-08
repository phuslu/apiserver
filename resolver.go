package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/alecthomas/geoip"
	"github.com/cloudflare/golibs/lrucache"
	"github.com/phuslu/glog"
)

type Resolver struct {
	*net.Resolver

	DNSCache lrucache.Cache
	DNSTTL   time.Duration

	static map[string][]net.IP
}

func (r *Resolver) LookupIP(ctx context.Context, name string) ([]net.IP, error) {
	if ips, _ := r.static[name]; len(ips) > 0 {
		return ips, nil
	}
	return r.lookupIP(ctx, name)
}

func (r *Resolver) Forget(name string) {
	r.DNSCache.Del(name)
}

func (r *Resolver) AddStaticHosts(reader io.Reader) error {
	if r.static == nil {
		r.static = make(map[string][]net.IP)
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		s := strings.TrimSpace(scanner.Text())
		if pos := strings.Index(s, "#"); pos >= 0 {
			s = s[:pos]
		}
		if s == "" {
			continue
		}

		words := strings.Fields(s)
		if len(words) < 2 {
			continue
		}

		ip := net.ParseIP(strings.TrimSpace(words[0]))
		if ip == nil {
			continue
		}

		for _, name := range words[1:] {
			name := strings.TrimSpace(name)
			r.static[name] = []net.IP{ip}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func (r *Resolver) AddStaticRecord(name string, ips ...net.IP) {
	if r.static == nil {
		r.static = make(map[string][]net.IP)
	}
	r.static[name] = ips
}

func (r *Resolver) IsStaticRecord(name string) bool {
	_, ok := r.static[name]
	return ok
}

func (r *Resolver) lookupIP(ctx context.Context, name string) ([]net.IP, error) {
	if r.DNSCache != nil {
		if v, ok := r.DNSCache.GetNotStale(name); ok {
			switch v.(type) {
			case []net.IP:
				return v.([]net.IP), nil
			case string:
				return r.lookupIP(ctx, v.(string))
			default:
				return nil, fmt.Errorf("LookupIP: cannot convert %T(%+v) to []net.IP", v, v)
			}
		}
	}

	if ip := net.ParseIP(name); ip != nil {
		return []net.IP{ip}, nil
	}

	addrs, err := r.Resolver.LookupIPAddr(ctx, name)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, len(addrs))
	for i, ia := range addrs {
		ips[i] = ia.IP
	}

	if r.DNSTTL > 0 && r.DNSCache != nil && len(ips) > 0 {
		r.DNSCache.Set(name, ips, time.Now().Add(r.DNSTTL))
	}

	glog.V(2).Infof("lookupIP(%#v) return %+v", name, ips)
	return ips, nil
}

// see https://en.wikipedia.org/wiki/Reserved_IP_addresses
func IsReservedIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		switch ip4[0] {
		case 10:
			return true
		case 100:
			return ip4[1] >= 64 && ip4[1] <= 127
		case 127:
			return true
		case 169:
			return ip4[1] == 254
		case 172:
			return ip4[1] >= 16 && ip4[1] <= 31
		case 192:
			switch ip4[1] {
			case 0:
				switch ip4[2] {
				case 0, 2:
					return true
				}
			case 18, 19:
				return true
			case 51:
				return ip4[2] == 100
			case 88:
				return ip4[2] == 99
			case 168:
				return true
			}
		case 203:
			return ip4[1] == 0 && ip4[2] == 113
		case 224:
			return true
		case 240:
			return true
		}
	}
	return false
}

func IsPoisonousChinaIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	switch ip4[0] {
	case 42:
		return ip4[1] == 123 && ip4[2] == 125 && ip4[3] == 237 // 42.123.125.237
	case 60:
		return ip4[1] == 19 && ip4[2] == 29 && ip4[3] == 22 // 60.19.29.22
	case 61:
		switch ip4[1] {
		case 54:
			return ip4[2] == 28 && ip4[3] == 6 // 61.54.28.6
		case 131:
			switch ip4[2] {
			case 208:
				switch ip4[3] {
				case 210, 211: // 61.131.208.210, 61.131.208.211
					return true
				}
			}
		}
	case 110:
		return ip4[1] == 249 && ip4[2] == 209 && ip4[3] == 42 // 110.249.209.42
	case 113:
		return ip4[1] == 11 && ip4[2] == 194 && ip4[3] == 190 // 113.11.194.190
	case 120:
		return ip4[1] == 192 && ip4[2] == 83 && ip4[3] == 163 // 120.192.83.163
	case 123:
		switch ip4[1] {
		case 126:
			return ip4[2] == 249 && ip4[3] == 238 // 123.126.249.238
		case 129:
			switch ip4[2] {
			case 254:
				switch ip4[3] {
				case 12, 13, 14, 15: // 123.129.254.12, 123.129.254.13, 123.129.254.14, 123.129.254.15
					return true
				}
			}
		}
	case 125:
		return ip4[1] == 211 && ip4[2] == 213 && ip4[3] == 132 // 125.211.213.132
	case 183:
		return ip4[1] == 221 && ip4[2] == 250 && ip4[3] == 11 // 183.221.250.11
	case 202:
		switch ip4[1] {
		case 98:
			switch ip4[2] {
			case 24:
				switch ip4[3] {
				case 122, 124, 125: // 202.98.24.122, 202.98.24.124, 202.98.24.125
					return true
				}
			}
		case 106:
			return ip4[2] == 1 && ip4[3] == 2 // 202.106.1.2
		case 181:
			return ip4[2] == 7 && ip4[3] == 85 // 202.181.7.85
		}
	case 211:
		switch ip4[1] {
		case 138:
			switch ip4[2] {
			case 34:
				return ip4[3] == 204 // 211.138.34.204
			case 74:
				return ip4[3] == 132 // 211.138.74.132
			}
		case 94:
			return ip4[2] == 66 && ip4[3] == 147 // 211.94.66.147
		case 98:
			switch ip4[2] {
			case 70:
				switch ip4[3] {
				case 195, 226, 227: // 211.98.70.195, 211.98.70.225, 211.98.70.227
					return true
				}
			case 71:
				return ip4[3] == 195 // 211.98.71.195
			}
		}
	case 218:
		return ip4[1] == 93 && ip4[2] == 250 && ip4[3] == 18 // 218.93.250.18
	case 220:
		switch ip4[1] {
		case 165:
			switch ip4[2] {
			case 8:
				switch ip4[3] {
				case 172, 174: // 220.165.8.172, 220.165.8.174
					return true
				}
			}
		case 250:
			return ip4[2] == 64 && ip4[3] == 20 // 220.250.64.20
		}
	case 221:
		switch ip4[1] {
		case 8:
			return ip4[2] == 69 && ip4[3] == 27 // 221.8.69.27
		case 179:
			return ip4[2] == 46 && ip4[3] == 190 // 221.179.46.190
		}
	}
	return false
}

type RegionResolver struct {
	Resolver *Resolver
	GeoIP    *geoip.GeoIP
	Cache    lrucache.Cache
}

func (r *RegionResolver) LookupCountry(ctx context.Context, host string) (string, error) {
	if v, ok := r.Cache.GetNotStale(host); ok {
		return v.(string), nil
	}

	// IPv6 address
	if host[0] == '[' {
		return "ZZ", nil
	}

	ips, err := r.Resolver.LookupIP(ctx, host)
	if err != nil {
		return "ZZ", err
	}
	if len(ips) == 0 {
		return "ZZ", nil
	}

	ip := ips[0]

	if IsReservedIP(ip) {
		r.Cache.Set(host, "", time.Now().Add(7*24*time.Hour))
		return "", nil
	}

	if IsPoisonousChinaIP(ip) {
		r.Cache.Set(host, "ZZ", time.Now().Add(7*24*time.Hour))
		return "ZZ", nil
	}

	country := r.GeoIP.Lookup(ip)
	if country == nil || country.Short == "" {
		r.Cache.Set(host, "ZZ", time.Now().Add(10*time.Minute))
		return "ZZ", nil
	}

	r.Cache.Set(host, country.Short, time.Now().Add(1*time.Hour))

	return country.Short, nil
}
