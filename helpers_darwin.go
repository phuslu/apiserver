// +build darwin

package main

import (
	"net"
	"os"
	"syscall"
)

func RedirectStderrTo(file *os.File) error {
	return syscall.Dup2(int(file.Fd()), 2)
}

func SetBindNoPortSockopts(c syscall.RawConn) error {
	return nil
}

func ReusePortListen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func ReusePortListenUDP(network string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	return net.ListenUDP(network, laddr)
}

func SetProcessName(name string) error {
	return nil
}

func PinToCPU(cpu uint) error {
	return nil
}
