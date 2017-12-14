// +build windows

package main

import (
	"net"
	"os"
	"runtime"
	"syscall"
)

func SetStdHandle(stdhandle int32, handle syscall.Handle) error {
	procSetStdHandle := syscall.MustLoadDLL("kernel32.dll").MustFindProc("SetStdHandle")
	r0, _, e1 := syscall.Syscall(procSetStdHandle.Addr(), 2, uintptr(stdhandle), uintptr(handle), 0)
	if r0 == 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

// https://golang.org/src/internal/syscall/windows/zsyscall_windows.go
func GetCurrentThread() (pseudoHandle syscall.Handle, err error) {
	procGetCurrentThread := syscall.MustLoadDLL("kernel32.dll").MustFindProc("GetCurrentThread")
	r0, _, e1 := syscall.Syscall(procGetCurrentThread.Addr(), 0, 0, 0, 0)
	pseudoHandle = syscall.Handle(r0)
	if pseudoHandle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// https://msdn.microsoft.com/en-us/library/windows/desktop/ms686247(v=vs.85).aspx
func SetThreadAffinityMask(hThread syscall.Handle, dwThreadAffinityMask uint32) error {
	procSetThreadAffinityMask := syscall.MustLoadDLL("kernel32.dll").MustFindProc("procSetThreadAffinityMask")
	r0, _, e1 := syscall.Syscall(procSetThreadAffinityMask.Addr(), 2, uintptr(hThread), uintptr(dwThreadAffinityMask), 0)
	if r0 == 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

func RedirectStderrTo(file *os.File) error {
	err := SetStdHandle(syscall.STD_ERROR_HANDLE, syscall.Handle(file.Fd()))
	if err != nil {
		return err
	}

	os.Stderr = file

	return nil
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

// https://github.com/golang/go/issues/11243#issuecomment-112631423
func PinToCPU(cpu uint) error {
	hThread, err := GetCurrentThread()
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	return SetThreadAffinityMask(hThread, 1<<cpu)
}
