package main

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"time"
)

type TCPDialer struct {
	Resolver  *Resolver
	Control   net.ControlFunc
	LocalAddr *net.TCPAddr

	KeepAlive time.Duration
	Timeout   time.Duration
	Level     int

	RejectIntranet bool
	PreferIPv6     bool

	TLSClientSessionCache tls.ClientSessionCache
}

func (d *TCPDialer) Dial(network, address string) (net.Conn, error) {
	return d.dialContext(context.Background(), network, address, nil)
}

func (d *TCPDialer) DialTLS(network, address string, tlsConfig *tls.Config) (net.Conn, error) {
	return d.dialContext(context.Background(), network, address, tlsConfig)
}

func (d *TCPDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialContext(ctx, network, address, nil)
}

func (d *TCPDialer) dialContext(ctx context.Context, network, address string, tlsConfig *tls.Config) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	ips, err := d.Resolver.LookupIP(ctx, host)
	if err != nil {
		return nil, err
	}

	switch len(ips) {
	case 0:
		return nil, net.InvalidAddrError("Invaid DNS Record: " + address)
	case 1:
		break
	default:
		if !d.PreferIPv6 {
			if ips[0].To4() == nil {
				pos := len(ips) - 1
				if ips[pos].To4() != nil {
					ips[0], ips[pos] = ips[pos], ips[0]
				}
			}
		} else {
			if ips[0].To4() != nil {
				pos := len(ips) - 1
				if ips[pos].To4() == nil {
					ips[0], ips[pos] = ips[pos], ips[0]
				}
			}
		}
	}

	if d.RejectIntranet && IsReservedIP(ips[0]) {
		return nil, net.InvalidAddrError("Intranet address is rejected: " + ips[0].String())
	}

	port, _ := strconv.Atoi(portStr)

	if d.Timeout > 0 {
		deadline := time.Now().Add(d.Timeout)
		if d, ok := ctx.Deadline(); ok && deadline.After(d) {
			deadline = d
		}

		subCtx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = subCtx
	}

	switch d.Level {
	case 0, 1:
		return d.dialSerial(ctx, network, host, ips, port, tlsConfig)
	default:
		if len(ips) == 1 {
			ips = append(ips, ips[0])
		}
		return d.dialParallel(ctx, network, host, ips, port, tlsConfig)
	}
}

func (d *TCPDialer) dialSerial(ctx context.Context, network, hostname string, ips []net.IP, port int, tlsConfig *tls.Config) (conn net.Conn, err error) {
	for i, ip := range ips {
		raddr := &net.TCPAddr{IP: ip, Port: port}
		conn, err = net.DialTCPContext(ctx, network, d.LocalAddr, raddr, d.Control)
		if err != nil {
			if i < len(ips)-1 {
				continue
			} else {
				return nil, err
			}
		}

		if tlsConfig == nil {
			return conn, nil
		}

		tlsConn := tls.Client(conn, tlsConfig)
		err = tlsConn.Handshake()
		if err != nil {
			if i < len(ips)-1 {
				continue
			} else {
				return nil, err
			}
		}

		return tlsConn, nil
	}
	return nil, err
}

func (d *TCPDialer) dialParallel(ctx context.Context, network, hostname string, ips []net.IP, port int, tlsConfig *tls.Config) (net.Conn, error) {
	type dialResult struct {
		Conn net.Conn
		Err  error
	}

	level := len(ips)
	if level > d.Level {
		level = d.Level
		ips = ips[:level]
	}

	lane := make(chan dialResult, level)
	for i := 0; i < level; i++ {
		go func(ip net.IP, port int, tlsConfig *tls.Config) {
			raddr := &net.TCPAddr{IP: ip, Port: port}
			conn, err := net.DialTCPContext(ctx, network, d.LocalAddr, raddr, d.Control)
			if err != nil {
				lane <- dialResult{nil, err}
				return
			}

			if d.KeepAlive > 0 {
				conn.SetKeepAlive(true)
				conn.SetKeepAlivePeriod(d.KeepAlive)
			}

			if tlsConfig == nil {
				lane <- dialResult{conn, nil}
				return
			}

			tlsConn := tls.Client(conn, tlsConfig)
			err = tlsConn.Handshake()

			if err != nil {
				lane <- dialResult{nil, err}
				return
			}

			lane <- dialResult{tlsConn, nil}
		}(ips[i], port, tlsConfig)
	}

	var r dialResult
	for j := 0; j < level; j++ {
		r = <-lane
		if r.Err == nil {
			go func(count int) {
				var r1 dialResult
				for ; count > 0; count-- {
					r1 = <-lane
					if r1.Conn != nil {
						r1.Conn.Close()
					}
				}
			}(level - 1 - j)
			return r.Conn, nil
		}
	}

	return nil, r.Err
}
