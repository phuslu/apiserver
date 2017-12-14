package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

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

func IsPollutionIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	switch ip4[0] {
	case 1:
		switch ip4[1] {
		case 1:
			return ip4[2] == 1 && ip4[3] == 1 // 1.1.1.1
		case 2:
			return ip4[2] == 3 && ip4[3] == 4 // 1.2.3.4
		}
	case 2:
		return ip4[1] == 1 && ip4[2] == 1 && ip4[3] == 2 // 2.1.1.2
	case 4:
		return ip4[1] == 36 && ip4[2] == 66 && ip4[3] == 178 // 4.36.66.178
	case 8:
		return ip4[1] == 7 && ip4[2] == 198 && ip4[3] == 45 // 8.7.198.45
	case 10:
		return ip4[1] == 10 && ip4[2] == 10 && ip4[3] == 10 // 10.10.10.10
	case 20:
		return ip4[1] == 20 && ip4[2] == 20 && ip4[3] == 20 // 20.20.20.20
	case 23:
		return ip4[1] == 89 && ip4[2] == 5 && ip4[3] == 60 // 23.89.5.60
	case 31:
		switch ip4[1] {
		case 13:
			switch ip4[2] {
			case 66:
				return ip4[3] == 1 // 31.13.66.1
			case 68:
				return ip4[3] == 22 // 31.13.68.22
			case 69:
				return ip4[3] == 86 // 31.13.69.86
			case 74:
				return ip4[3] == 40 // 31.13.74.40
			}
		}
	case 37:
		return ip4[1] == 61 && ip4[2] == 54 && ip4[3] == 158 // 37.61.54.158
	case 42:
		return ip4[1] == 123 && ip4[2] == 125 && ip4[3] == 237 // 42.123.125.237
	case 46:
		return ip4[1] == 82 && ip4[2] == 174 && ip4[3] == 68 // 46.82.174.68
	case 49:
		return ip4[1] == 2 && ip4[2] == 123 && ip4[3] == 56 // 49.2.123.56
	case 54:
		return ip4[1] == 76 && ip4[2] == 135 && ip4[3] == 1 // 54.76.135.1
	case 59:
		return ip4[1] == 24 && ip4[2] == 3 && ip4[3] == 173 // 59.24.3.173
	case 60:
		return ip4[1] == 19 && ip4[2] == 29 && ip4[3] == 22 // 60.19.29.22
	case 61:
		switch ip4[1] {
		case 131:
			switch ip4[2] {
			case 208:
				switch ip4[3] {
				case 210, 211: // 61.131.208.210, 61.131.208.211
					return true
				}
			}
		}
	case 64:
		switch ip4[1] {
		case 33:
			switch ip4[2] {
			case 88:
				return ip4[3] == 161 // 64.33.88.161
			case 99:
				return ip4[3] == 47 // 64.33.99.47
			}
		case 66:
			return ip4[2] == 163 && ip4[3] == 251 // 64.66.163.251
		}
	case 65:
		switch ip4[1] {
		case 104:
			return ip4[2] == 202 && ip4[3] == 252 // 65.104.202.252
		case 160:
			return ip4[2] == 219 && ip4[3] == 113 // 65.160.219.113
		}
	case 66:
		return ip4[1] == 45 && ip4[2] == 252 && ip4[3] == 237 // 66.45.252.237
	case 69:
		return ip4[1] == 171 && ip4[2] == 247 && ip4[3] == 20 // 69.171.247.20
	case 72:
		switch ip4[1] {
		case 14:
			switch ip4[2] {
			case 205:
				switch ip4[3] {
				case 99, 104:
					return true // 72.14.205.99, 72.14.205.104
				}
			}
		}
	case 74:
		switch ip4[1] {
		case 125:
			switch ip4[2] {
			case 31:
				return ip4[3] == 113 // 74.125.31.113
			case 39:
				return ip4[3] == 102 || ip4[3] == 113 // 74.125.39.102, 74.125.39.113
			case 127:
				return ip4[3] == 102 || ip4[3] == 113 // 74.125.127.102, 74.125.127.113
			case 130:
				return ip4[3] == 47 // 74.125.130.47
			case 155:
				return ip4[3] == 102 // 74.125.155.102
			}
		}
	case 77:
		return ip4[1] == 4 && ip4[2] == 7 && ip4[3] == 92 // 77.4.7.92
	case 78:
		return ip4[1] == 16 && ip4[2] == 49 && ip4[3] == 15 // 78.16.49.15
	case 92:
		return ip4[1] == 242 && ip4[2] == 144 && ip4[3] == 2 //92.242.144.2
	case 93:
		return ip4[1] == 46 && ip4[2] == 8 && ip4[3] == 89 // 93.46.8.89
	case 108:
		return ip4[1] == 160 && ip4[2] == 166 && ip4[3] == 92 // 108.160.166.92
	case 110:
		return ip4[1] == 249 && ip4[2] == 209 && ip4[3] == 42 // 110.249.209.42
	case 118:
		return ip4[1] == 5 && ip4[2] == 49 && ip4[3] == 6 // 118.5.49.6
	case 120:
		return ip4[1] == 192 && ip4[2] == 83 && ip4[3] == 163 // 120.192.83.163
	case 123:
		switch ip4[1] {
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
	case 128:
		return ip4[1] == 121 && ip4[2] == 126 && ip4[3] == 139 // 128.121.126.139
	case 159:
		return ip4[1] == 106 && ip4[2] == 121 && ip4[3] == 75 // 159.106.121.75
	case 169:
		return ip4[1] == 132 && ip4[2] == 13 && ip4[3] == 103 // 169.132.13.103
	case 183:
		return ip4[1] == 221 && ip4[2] == 250 && ip4[3] == 11 // 183.221.250.11
	case 185:
		return ip4[1] == 85 && ip4[2] == 13 && ip4[3] == 155 // 185.85.13.155
	case 188:
		return ip4[1] == 5 && ip4[2] == 4 && ip4[3] == 96 // 188.5.4.96
	case 189:
		return ip4[1] == 163 && ip4[2] == 17 && ip4[3] == 5 // 189.163.17.5
	case 192:
		return ip4[1] == 67 && ip4[2] == 198 && ip4[3] == 6 // 192.67.198.6
	case 197:
		return ip4[1] == 4 && ip4[2] == 4 && ip4[3] == 12 // 197.4.4.12
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
	case 203:
		switch ip4[1] {
		case 98:
			return ip4[2] == 7 && ip4[3] == 65 // 203.98.7.65
		case 161:
			return ip4[2] == 230 && ip4[3] == 171 // 203.161.230.171
		}
	case 207:
		return ip4[1] == 12 && ip4[2] == 88 && ip4[3] == 98 // 207.12.88.98
	case 208:
		return ip4[1] == 56 && ip4[2] == 31 && ip4[3] == 43 // 208.56.31.43
	case 209:
		switch ip4[1] {
		case 36:
			return ip4[2] == 73 && ip4[3] == 33 // 209.36.73.33
		case 85:
			return ip4[2] == 229 && ip4[3] == 138 // 209.85.229.138
		case 145:
			return ip4[2] == 54 && ip4[3] == 50 // 209.145.54.50
		case 220:
			return ip4[2] == 30 && ip4[3] == 174 // 209.220.30.174
		}
	case 210:
		return ip4[1] == 242 && ip4[2] == 125 && ip4[3] == 20 // 210.242.125.20
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
	case 213:
		return ip4[1] == 169 && ip4[2] == 251 && ip4[3] == 35 // 213.169.251.35
	case 216:
		switch ip4[1] {
		case 221:
			return ip4[2] == 188 && ip4[3] == 182 // 216.221.188.182
		case 234:
			return ip4[2] == 179 && ip4[3] == 13 // 216.234.179.13
		}
	case 218:
		return ip4[1] == 93 && ip4[2] == 250 && ip4[3] == 18 // 218.93.250.18
	case 220:
		switch ip4[1] {
		case 165:
			switch ip4[2] {
			case 8:
				switch ip4[3] {
				case 172, 174: // 220.165.8.172 220.165.8.174
					return true
				}
			}
		case 250:
			return ip4[2] == 64 && ip4[3] == 20 // 220.250.64.20
		}
	case 221:
		return ip4[1] == 179 && ip4[2] == 46 && ip4[3] == 190 // 221.179.46.190
	case 243:
		return ip4[1] == 185 && ip4[2] == 187 && ip4[3] == 39 // 243.185.187.39
	case 249:
		return ip4[1] == 129 && ip4[2] == 46 && ip4[3] == 48 // 249.129.46.48
	case 253:
		return ip4[1] == 157 && ip4[2] == 14 && ip4[3] == 165 // 253.157.14.165
	case 255:
		return ip4[1] == 255 && ip4[2] == 255 && ip4[3] == 255 // 255.255.255.255
	}
	return false
}
