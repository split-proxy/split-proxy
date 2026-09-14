package main

import (
	"fmt"
	"net/netip"
)

func validateDirectTargetIP(addr netip.Addr) error {
	if !addr.IsValid() {
		return fmt.Errorf("invalid IP address")
	}

	addr = addr.Unmap()

	if addr.IsLoopback() {
		return fmt.Errorf("loopback address %s is not allowed", addr)
	}

	if addr.IsPrivate() {
		return fmt.Errorf("private address %s is not allowed", addr)
	}

	if addr.IsLinkLocalUnicast() {
		return fmt.Errorf("link-local address %s is not allowed", addr)
	}

	if addr.IsUnspecified() {
		return fmt.Errorf("unspecified address %s is not allowed", addr)
	}

	if addr.IsMulticast() {
		return fmt.Errorf("multicast address %s is not allowed", addr)
	}

	return nil
}
