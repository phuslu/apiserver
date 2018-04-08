package main

import (
	"crypto/tls"
	"encoding/hex"
	"testing"
)

func TestJash3HashDummy(t *testing.T) {
	clientHello := &tls.ClientHelloInfo{}
	t.Logf("Ja3Hash Dummy: %x", Ja3Hash(clientHello))
}

func TestJash3HashTLS13(t *testing.T) {
	clientHello := &tls.ClientHelloInfo{
		SupportedVersions: []uint16{0x0301},
		CipherSuites:      []uint16{0x2f, 0x35, 0x5, 0xa, 0xc009, 0xc00a, 0xc013, 0xc014, 0x32, 0x38, 0x13, 0x4},
		Extensions:        []uint16{0x0, 0xa, 0xb},
		SupportedCurves:   []tls.CurveID{0x17, 0x18, 0x19},
		SupportedPoints:   []uint8{0x0},
	}
	b := Ja3Hash(clientHello)
	digest := hex.EncodeToString(b[:])
	if digest != "ada70206e40642a3e4461f35503241d5" {
		t.Errorf("Ja3Hash Chrome: %x mismatch", b)
	} else {
		t.Logf("Ja3Hash Chrome: %x", b)
	}
}
