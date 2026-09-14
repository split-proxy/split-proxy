package main

import (
	"net/netip"
	"testing"
	"time"
)

func mustBuildCIDRTrie(cidrs map[string]Route) *CIDRTrie {
	trie := &CIDRTrie{}

	for cidr, route := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			panic(err)
		}

		trie.insert(prefix.Masked(), route)
	}

	return trie
}

func mustBuildWildcardTrie(routes map[string]Route) *DomainTrie {
	trie := newDomainTrie()

	for pattern, route := range routes {
		trie.insertWildcard(pattern, route)
	}

	return trie
}

func mustBuildDNSCache(entries map[string][]string) *DNSCache {
	cache := newDNSCache(
		1000,
		time.Minute,
		time.Second,
	)

	for host, ipStrings := range entries {
		ips := make([]netip.Addr, 0, len(ipStrings))

		for _, ipString := range ipStrings {
			addr, err := netip.ParseAddr(ipString)
			if err != nil {
				panic(err)
			}

			ips = append(ips, addr)
		}

		cache.set(host, ips, true)
	}

	return cache
}

func TestResolveRoute(t *testing.T) {
	dns := mustBuildDNSCache(map[string][]string{
		"unknown.example.net": {
			"192.168.100.10",
		},
	})
	defer dns.close()

	routes.Store(RouteCache{
		Domains: map[string]Route{
			"example.com": "exact",
		},

		WildcardDomains: mustBuildWildcardTrie(map[string]Route{
			"*.example.org":     "wildcard",
			"*.api.example.com": "specific-wildcard",
		}),

		CIDRs: mustBuildCIDRTrie(map[string]Route{
			"10.0.0.0/8":   "network-10",
			"10.10.0.0/16": "network-10-10",
		}),

		DNS:          dns,
		DefaultRoute: "default",
	})

	tests := []struct {
		name string
		host string
		want Route
	}{
		{
			name: "exact domain",
			host: "example.com",
			want: "exact",
		},
		{
			name: "exact domain case insensitive",
			host: "EXAMPLE.COM",
			want: "exact",
		},
		{
			name: "exact domain trims spaces",
			host: "  example.com  ",
			want: "exact",
		},
		{
			name: "exact domain trims trailing dot",
			host: "example.com.",
			want: "exact",
		},
		{
			name: "wildcard domain",
			host: "foo.example.org",
			want: "wildcard",
		},
		{
			name: "wildcard does not match base domain",
			host: "example.org",
			want: "default",
		},
		{
			name: "specific wildcard",
			host: "foo.api.example.com",
			want: "specific-wildcard",
		},
		{
			name: "nested specific wildcard",
			host: "foo.bar.api.example.com",
			want: "specific-wildcard",
		},
		{
			name: "ipv4 cidr",
			host: "10.20.30.40",
			want: "network-10",
		},
		{
			name: "more specific ipv4 cidr",
			host: "10.10.20.30",
			want: "network-10-10",
		},
		{
			name: "unknown domain uses default route",
			host: "unknown.example.com",
			want: "default",
		},
		{
				name: "empty host uses RouteDirect",
				host: "",
				want: RouteDirect,
		},
		{
			name: "dns result without matching cidr uses default",
			host: "unknown.example.net",
			want: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveRoute(tt.host)

			if got != tt.want {
				t.Errorf(
					"ResolveRoute(%q) = %q, want %q",
					tt.host,
					got,
					tt.want,
				)
			}
		})
	}
}

func TestResolveCIDRRoute(t *testing.T) {
	dns := mustBuildDNSCache(map[string][]string{
		"network-10.example": {
			"10.20.30.40",
		},
		"network-10-10.example": {
			"10.10.20.30",
		},
		"outside.example": {
			"192.168.1.1",
		},
	})
	defer dns.close()

	cache := RouteCache{
		CIDRs: mustBuildCIDRTrie(map[string]Route{
			"10.0.0.0/8":   "network-10",
			"10.10.0.0/16": "network-10-10",
		}),
		DNS: dns,
	}

	tests := []struct {
		name string
		host string
		want Route
		ok   bool
	}{
		{
			name: "matches cidr",
			host: "network-10.example",
			want: "network-10",
			ok:   true,
		},
		{
			name: "most specific cidr wins",
			host: "network-10-10.example",
			want: "network-10-10",
			ok:   true,
		},
		{
			name: "outside cidr",
			host: "outside.example",
			want: Route(""),
			ok:   false,
		},
		{
			name: "unknown hostname",
			host: "does-not-exist.example",
			want: Route(""),
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoute, gotOK := resolveCIDRRoute(tt.host, cache)

			if gotRoute != tt.want {
				t.Errorf(
					"resolveCIDRRoute(%q) route = %q, want %q",
					tt.host,
					gotRoute,
					tt.want,
				)
			}

			if gotOK != tt.ok {
				t.Errorf(
					"resolveCIDRRoute(%q) ok = %v, want %v",
					tt.host,
					gotOK,
					tt.ok,
				)
			}
		})
	}
}

func TestCIDRTrieLongestPrefixMatch(t *testing.T) {
	trie := mustBuildCIDRTrie(map[string]Route{
		"0.0.0.0/0":       "default",
		"10.0.0.0/8":      "network-10",
		"10.10.0.0/16":    "network-10-10",
		"10.10.20.0/24":   "network-10-10-20",
		"10.10.20.128/25": "network-10-10-20-128",
	})

	tests := []struct {
		ip   string
		want Route
		bits int
	}{
		{
			ip:   "8.8.8.8",
			want: "default",
			bits: 0,
		},
		{
			ip:   "10.20.30.40",
			want: "network-10",
			bits: 8,
		},
		{
			ip:   "10.10.30.40",
			want: "network-10-10",
			bits: 16,
		},
		{
			ip:   "10.10.20.40",
			want: "network-10-10-20",
			bits: 24,
		},
		{
			ip:   "10.10.20.200",
			want: "network-10-10-20-128",
			bits: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr, err := netip.ParseAddr(tt.ip)
			if err != nil {
				t.Fatalf("invalid IP %q: %v", tt.ip, err)
			}

			gotRoute, gotBits, ok := trie.lookup(addr)

			if !ok {
				t.Fatalf("lookup(%q) returned no route", tt.ip)
			}

			if gotRoute != tt.want {
				t.Errorf(
					"lookup(%q) route = %q, want %q",
					tt.ip,
					gotRoute,
					tt.want,
				)
			}

			if gotBits != tt.bits {
				t.Errorf(
					"lookup(%q) bits = %d, want %d",
					tt.ip,
					gotBits,
					tt.bits,
				)
			}
		})
	}
}

func TestCIDRTrieIPv6(t *testing.T) {
	trie := mustBuildCIDRTrie(map[string]Route{
		"::/0":                "ipv6-default",
		"2001:db8::/32":       "ipv6-doc",
		"2001:db8:1234::/48": "ipv6-specific",
	})

	tests := []struct {
		ip   string
		want Route
		bits int
	}{
		{
			ip:   "2001:db8:1234::1",
			want: "ipv6-specific",
			bits: 48,
		},
		{
			ip:   "2001:db8:ffff::1",
			want: "ipv6-doc",
			bits: 32,
		},
		{
			ip:   "2001:4860:4860::8888",
			want: "ipv6-default",
			bits: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			addr, err := netip.ParseAddr(tt.ip)
			if err != nil {
				t.Fatalf("invalid IP %q: %v", tt.ip, err)
			}

			gotRoute, gotBits, ok := trie.lookup(addr)

			if !ok {
				t.Fatalf("lookup(%q) returned no route", tt.ip)
			}

			if gotRoute != tt.want {
				t.Errorf(
					"lookup(%q) route = %q, want %q",
					tt.ip,
					gotRoute,
					tt.want,
				)
			}

			if gotBits != tt.bits {
				t.Errorf(
					"lookup(%q) bits = %d, want %d",
					tt.ip,
					gotBits,
					tt.bits,
				)
			}
		})
	}
}

func TestDomainTrieWildcard(t *testing.T) {
	trie := mustBuildWildcardTrie(map[string]Route{
		"*.example.com":     "example",
		"*.api.example.com": "api",
	})

	tests := []struct {
		host string
		want Route
		ok   bool
	}{
		{
			host: "example.com",
			want: Route(""),
			ok:   false,
		},
		{
			host: "foo.example.com",
			want: "example",
			ok:   true,
		},
		{
			host: "foo.api.example.com",
			want: "api",
			ok:   true,
		},
		{
			host: "foo.bar.api.example.com",
			want: "api",
			ok:   true,
		},
		{
			host: "example.net",
			want: Route(""),
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			gotRoute, gotOK := trie.lookupWildcard(tt.host)

			if gotRoute != tt.want {
				t.Errorf(
					"lookupWildcard(%q) route = %q, want %q",
					tt.host,
					gotRoute,
					tt.want,
				)
			}

			if gotOK != tt.ok {
				t.Errorf(
					"lookupWildcard(%q) ok = %v, want %v",
					tt.host,
					gotOK,
					tt.ok,
				)
			}
		})
	}
}

func TestDNSCache(t *testing.T) {
	cache := newDNSCache(
		100,
		time.Minute,
		time.Second,
	)
	defer cache.close()

	ips := []netip.Addr{
		netip.MustParseAddr("10.10.10.10"),
		netip.MustParseAddr("2001:db8::1"),
	}

	cache.set("Example.COM.", ips, true)

	got, ok := cache.lookup("example.com")

	if !ok {
		t.Fatal("expected DNS cache hit")
	}

	if len(got) != 2 {
		t.Fatalf("got %d IPs, want 2", len(got))
	}

	if got[0] != ips[0] {
		t.Errorf("got first IP %v, want %v", got[0], ips[0])
	}

	if got[1] != ips[1] {
		t.Errorf("got second IP %v, want %v", got[1], ips[1])
	}
}

func TestDNSCacheNegative(t *testing.T) {
	cache := newDNSCache(
		100,
		time.Minute,
		time.Second,
	)
	defer cache.close()

	cache.set(
		"does-not-exist.example",
		nil,
		false,
	)

	ips, ok := cache.lookup("does-not-exist.example")

	if ok {
		t.Fatal("expected negative DNS cache result")
	}

	if len(ips) != 0 {
		t.Fatalf("expected no IPs, got %v", ips)
	}
}
