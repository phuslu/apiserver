// +build !linux
// +build !windows

package main

import (
	"errors"
	"net"
	"os"
	"syscall"
)

type Announcer struct {
	ReusePort   bool
	FastOpen    bool
	DeferAccept bool
}

func (an Announcer) Listen(network, address string) (*net.TCPListener, error) {
	laddr, err := net.ResolveTCPAddr(network, address)
	if err != nil {
		return nil, err
	}
	return net.ListenTCP(network, laddr)
}

func (an Announcer) ListenPacket(network, address string) (net.PacketConn, error) {
	laddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}

	return net.ListenUDP(network, laddr)
}

type DailerController struct {
	BindAddressNoPort bool
}

func (dc DailerController) Control(network string, addr net.Addr, c syscall.RawConn) error {
	return nil
}

func RedirectStderrTo(file *os.File) error {
	return syscall.Dup2(int(file.Fd()), 2)
}

func SetProcessName(name string) error {
	return nil
}

func PinToCPU(cpu uint) error {
	return nil
}

func ReadHTTPHeader(conn *net.TCPConn) ([]byte, *net.TCPConn, error) {
	return nil, conn, errors.New("not implemented")
}
