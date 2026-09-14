package main

import (
	"bufio"
	"fmt"
)

type Protocol int

const (
	ProtocolUnknown Protocol = iota
	ProtocolHTTP
	ProtocolSOCKS5
	ProtocolTLS
)

func detectProtocol(reader *bufio.Reader) (Protocol, error) {
	// SOCKS5
	b, err := reader.Peek(1)
	if err != nil {
		return ProtocolUnknown, fmt.Errorf("peek protocol: %w", err)
	}

	if b[0] == 0x05 {
		return ProtocolSOCKS5, nil
	}

	// TLS ClientHello
	tlsHeader, err := reader.Peek(3)
	if err == nil &&
		tlsHeader[0] == 0x16 &&
		tlsHeader[1] == 0x03 {
		return ProtocolTLS, nil
	}

	// HTTP CONNECT
	httpHeader, err := reader.Peek(len("CONNECT"))
	if err == nil && string(httpHeader) == "CONNECT" {
		return ProtocolHTTP, nil
	}

	return ProtocolUnknown, fmt.Errorf("unknown proxy protocol")
}
