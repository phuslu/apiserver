package main

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	BUFSZ = 128 * 1024
)

var (
	bufpool = sync.Pool{
		New: func() interface{} {
			return make([]byte, BUFSZ)
		},
	}
)

func IOCopy(dst io.Writer, src io.Reader) (written int64, err error) {
	// If the reader has a WriteTo method, use it to do the copy.
	// Avoids an allocation and a copy.
	if wt, ok := src.(io.WriterTo); ok {
		return wt.WriteTo(dst)
	}
	// Similarly, if the writer has a ReadFrom method, use it to do the copy.
	if rt, ok := dst.(io.ReaderFrom); ok {
		return rt.ReadFrom(src)
	}
	buf := bufpool.Get().([]byte)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	bufpool.Put(buf)
	return written, err
}

type FlushWriter struct {
	w io.Writer
}

func (fw FlushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

type TCPListener struct {
	*net.TCPListener
	SkipHTTPHeader  bool
	KeepAlivePeriod time.Duration
	ReadBufferSize  int
	WriteBufferSize int
}

func (ln TCPListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	if ln.KeepAlivePeriod > 0 {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(ln.KeepAlivePeriod)
	}
	if ln.ReadBufferSize > 0 {
		tc.SetReadBuffer(ln.ReadBufferSize)
	}
	if ln.WriteBufferSize > 0 {
		tc.SetWriteBuffer(ln.WriteBufferSize)
	}
	if ln.SkipHTTPHeader {
		ReadHTTPHeader(tc)
	}
	return tc, nil
}

type ConnWithData struct {
	net.Conn
	Data []byte
}

func (c *ConnWithData) Read(b []byte) (int, error) {
	if c.Data == nil {
		return c.Conn.Read(b)
	}

	n := copy(b, c.Data)
	if n < len(c.Data) {
		c.Data = c.Data[n:]
	} else {
		c.Data = nil
	}

	return n, nil
}

type ConnWithBuffers struct {
	net.Conn
	Buffers net.Buffers
}

func (c *ConnWithBuffers) Read(b []byte) (int, error) {
	if c.Buffers == nil {
		return c.Conn.Read(b)
	}

	var total int
	for {
		n := copy(b, c.Buffers[0])
		total += n

		if n < len(c.Buffers[0]) {
			// b is full
			c.Buffers[0] = c.Buffers[0][n:]
			break
		}

		c.Buffers = c.Buffers[1:]
		if len(c.Buffers) == 0 {
			c.Buffers = nil
			break
		}

		b = b[n:]
		if len(b) == 0 {
			break
		}
	}

	return total, nil
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func MaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func MinInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func MaxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func MinDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func HasString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func RemoveString(ss []string, s string) []string {
	for i, v := range ss {
		if v == s {
			return append(ss[:i], ss[i+1:]...)
		}
	}
	return ss
}

func StringSet(ss []string, toupper bool) (m map[string]struct{}) {
	if len(ss) == 0 {
		return nil
	}

	m = make(map[string]struct{})
	for _, s := range ss {
		if toupper {
			m[strings.ToUpper(s)] = struct{}{}
		} else {
			m[s] = struct{}{}
		}
	}
	return
}

func LookupEcdsaCiphers(clientHello *tls.ClientHelloInfo) uint16 {
	for _, cipher := range clientHello.CipherSuites {
		switch cipher {
		case tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA:
			return cipher
		case tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA:
			return 0
		}
	}
	return 0
}

func HasTLS13Support(clientHello *tls.ClientHelloInfo) bool {
	for _, ver := range clientHello.SupportedVersions {
		switch ver {
		case tls.VersionTLS12, tls.VersionTLS11, tls.VersionTLS10, tls.VersionSSL30:
			return false
		case tls.VersionTLS13, 0x7f17:
			return true
		}
	}
	return false
}

func IsTLSGreaseCode(c uint16) bool {
	return c&0x0f0f == 0x0a0a && c&0xff == c>>8
}

var (
	ja3pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 512)
		},
	}
)

func Ja3Hash(clientHello *tls.ClientHelloInfo) (digest [16]byte) {
	buf := bufpool.Get().([]byte)[:0]

	// versions
	for i, v := range clientHello.SupportedVersions {
		if IsTLSGreaseCode(v) {
			continue
		}
		buf = strconv.AppendInt(buf, int64(v), 10)
		if i == len(clientHello.SupportedVersions)-1 {
			buf = append(buf, ',')
		} else {
			buf = append(buf, '-')
		}
	}
	// cipersuites
	for i, v := range clientHello.CipherSuites {
		if IsTLSGreaseCode(v) {
			continue
		}
		buf = strconv.AppendInt(buf, int64(v), 10)
		if i == len(clientHello.CipherSuites)-1 {
			buf = append(buf, ',')
		} else {
			buf = append(buf, '-')
		}
	}
	// extensions
	for i, v := range clientHello.Extensions {
		if IsTLSGreaseCode(v) {
			continue
		}
		buf = strconv.AppendInt(buf, int64(v), 10)
		if i == len(clientHello.Extensions)-1 {
			buf = append(buf, ',')
		} else {
			buf = append(buf, '-')
		}
	}
	// curves
	for i, v := range clientHello.SupportedCurves {
		if IsTLSGreaseCode(uint16(v)) {
			continue
		}
		buf = strconv.AppendInt(buf, int64(v), 10)
		if i == len(clientHello.SupportedCurves)-1 {
			buf = append(buf, ',')
		} else {
			buf = append(buf, '-')
		}
	}
	// curve points
	for i, v := range clientHello.SupportedPoints {
		buf = strconv.AppendInt(buf, int64(v), 10)
		if i != len(clientHello.SupportedPoints)-1 {
			buf = append(buf, '-')
		}
	}

	digest = md5.Sum(buf)
	bufpool.Put(buf)

	return
}

func GetPreferedLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return nil, err
	}

	s, _, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return nil, err
	}

	return net.ParseIP(s), nil
}

func IsTimeout(err error) bool {
	switch err {
	case nil:
		return false
	case context.Canceled:
		return true
	}

	if terr, ok := err.(interface {
		Timeout() bool
	}); ok {
		return terr.Timeout()
	}

	return false
}

func StartWatchDog() {
	if os.Getenv("watchdog") != "1" {
		return
	}

	executable, _ := os.Executable()
	os.Chdir(filepath.Dir(executable))

	deepcopy := func(s []string) []string { return strings.Split(strings.Join(s, "\x00"), "\x00") }
	osArgs := deepcopy(os.Args)
	osEnviron := deepcopy(RemoveString(os.Environ(), "watchdog=1"))

	var child *os.Process
	var watchdog func()
	watchdog = func() {
		p, err := os.StartProcess(executable, osArgs, &os.ProcAttr{
			Dir:   ".",
			Env:   osEnviron,
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		})
		if err != nil {
			panic("os.StartProcess error: " + err.Error())
		}

		if child != nil {
			child.Signal(syscall.SIGHUP)
		}

		child = p

		SetProcessName(filepath.Base(executable) + ": master process " + executable)

		ps, err := p.Wait()
		if ps != nil && !ps.Success() {
			go watchdog()
		}

		return
	}

	go watchdog()

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	signal.Notify(c, syscall.SIGTERM)

	for {
		switch sig := <-c; sig {
		case syscall.SIGHUP:
			go watchdog()
		case syscall.SIGTERM:
			child.Signal(sig)
			os.Exit(0)
		}
	}
}
