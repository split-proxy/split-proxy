package main

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"
)

const directDialTimeout = 10 * time.Second

func resolveDirectTarget(address string) ([]netip.Addr, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid target %q: %w", address, err)
	}

	host = strings.TrimSpace(host)

	if host == "" {
		return nil, fmt.Errorf("empty target host")
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()

		if err := validateDirectTargetIP(ip); err != nil {
			return nil, err
		}

		return []netip.Addr{ip}, nil
	}

	ips, ok := dnsCache.lookup(host)
	if !ok {
		return nil, fmt.Errorf("DNS resolution failed for %q", host)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("DNS returned no addresses for %q", host)
	}

	allowed := make([]netip.Addr, 0, len(ips))

	for _, ip := range ips {
		ip = ip.Unmap()

		if err := validateDirectTargetIP(ip); err != nil {
			debugf(
				"[DIRECT] target IP rejected host=%s ip=%s: %v",
				host,
				ip,
				err,
			)

			continue
		}

		allowed = append(allowed, ip)
	}

	if len(allowed) == 0 {
		return nil, fmt.Errorf(
			"all resolved addresses for %q are blocked by DIRECT target policy",
			host,
		)
	}

	return allowed, nil
}

func dialDirectTarget(address string) (net.Conn, netip.Addr, error) {
	ips, err := resolveDirectTarget(address)
	if err != nil {
		return nil, netip.Addr{}, err
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, netip.Addr{}, fmt.Errorf(
			"invalid target %q: %w",
			address,
			err,
		)
	}

	var lastErr error

	for _, ip := range ips {
		target := net.JoinHostPort(ip.String(), port)

		infof(
			"[DIRECT] dialing resolved target host=%s ip=%s port=%s",
			host,
			ip,
			port,
		)

		conn, err := net.DialTimeout(
			"tcp",
			target,
			directDialTimeout,
		)
		if err == nil {
			return conn, ip, nil
		}

		lastErr = err

		debugf(
			"[DIRECT] resolved target connection failed host=%s ip=%s port=%s: %v",
			host,
			ip,
			port,
			err,
		)
	}

	return nil, netip.Addr{}, fmt.Errorf(
		"failed to connect to any allowed address for %q: %w",
		address,
		lastErr,
	)
}
