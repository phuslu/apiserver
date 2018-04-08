// +build linux

package main

import (
	"bytes"
	"io"
	"net"
	"os"
	"reflect"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	SO_REUSEPORT            = 15
	TCP_FASTOPEN            = 23
	IP_BIND_ADDRESS_NO_PORT = 24
)

type Announcer struct {
	ReusePort   bool
	FastOpen    bool
	DeferAccept bool
}

func (an Announcer) Listen(network, address string) (*net.TCPListener, error) {
	control := func(network string, address net.Addr, conn syscall.RawConn) error {
		return conn.Control(func(fd uintptr) {
			if an.ReusePort {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, SO_REUSEPORT, 1)
			}
			if an.FastOpen {
				syscall.SetsockoptInt(int(fd), syscall.SOL_TCP, TCP_FASTOPEN, 16*1024)
			}
			if an.DeferAccept {
				syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_DEFER_ACCEPT, 1)
			}
		})
	}

	tl, err := net.ListenControl(network, address, control)
	if err != nil {
		return nil, err
	}

	return tl.(*net.TCPListener), nil
}

func (an Announcer) ListenPacket(network, address string) (net.PacketConn, error) {
	control := func(network string, address net.Addr, conn syscall.RawConn) error {
		return conn.Control(func(fd uintptr) {
			if an.ReusePort {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, SO_REUSEPORT, 1)
			}
		})
	}

	laddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	return net.ListenUDPControl(network, laddr, control)
}

type DailerController struct {
	BindAddressNoPort bool
}

func (dc DailerController) Control(network string, addr net.Addr, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {
		if dc.BindAddressNoPort {
			syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, IP_BIND_ADDRESS_NO_PORT, 1)
		}
	})
}

func RedirectStderrTo(file *os.File) error {
	return syscall.Dup3(int(file.Fd()), 2, 0)
}

func SetProcessName(name string) error {
	argv0str := (*reflect.StringHeader)(unsafe.Pointer(&os.Args[0]))
	argv0 := (*[1 << 30]byte)(unsafe.Pointer(argv0str.Data))[:len(name)+1]

	n := copy(argv0, name+string(0))
	if n < len(argv0) {
		argv0[n] = 0
	}
	return nil
}

// https://github.com/golang/go/issues/11243#issuecomment-112631423
func PinToCPU(cpu int) error {
	runtime.LockOSThread()

	var mask unix.CPUSet
	mask.Set(cpu)
	return unix.SchedSetaffinity(0, &mask)
}

func ReadHTTPHeader(tc *net.TCPConn) ([]byte, *net.TCPConn, error) {
	f, err := tc.File()
	if err != nil {
		return nil, tc, err
	}

	b := make([]byte, os.Getpagesize())
	n, _, err := syscall.Recvfrom(int(f.Fd()), b, syscall.MSG_PEEK)
	if err != nil {
		return nil, tc, err
	}

	if n == 0 {
		return nil, tc, io.EOF
	}

	if b[0] < 'A' || b[0] > 'Z' {
		return nil, tc, io.EOF
	}

	n = bytes.Index(b, []byte{'\r', '\n', '\r', '\n'})
	if n < 0 {
		return nil, tc, io.EOF
	}

	b = b[:n+4]
	n, err = tc.Read(b)

	return b, tc, err
}
