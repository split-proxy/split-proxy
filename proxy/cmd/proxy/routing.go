package main

import (
	"os"
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type CIDRNode struct {
	children [2]*CIDRNode

	route    Route
	hasRoute bool
}

type CIDRTrie struct {
	ipv4 *CIDRNode
	ipv6 *CIDRNode
}

func (t *CIDRTrie) insert(prefix netip.Prefix, route Route) {
	if t == nil || !prefix.IsValid() {
		return
	}

	prefix = prefix.Masked()

	addr := prefix.Addr()

	var (
		root **CIDRNode
		raw  []byte
		bits int
	)

	if addr.Is4() {
		root = &t.ipv4

		v := addr.As4()
		raw = v[:]

		bits = prefix.Bits()
		if bits < 0 || bits > 32 {
			return
		}
	} else if addr.Is6() {
		root = &t.ipv6

		v := addr.As16()
		raw = v[:]

		bits = prefix.Bits()
		if bits < 0 || bits > 128 {
			return
		}
	} else {
		return
	}

	if *root == nil {
		*root = &CIDRNode{}
	}

	node := *root

	for i := 0; i < bits; i++ {
		bit := (raw[i/8] >> uint(7-(i%8))) & 1

		child := node.children[bit]

		if child == nil {
			child = &CIDRNode{}
			node.children[bit] = child
		}

		node = child
	}

	node.route = route
	node.hasRoute = true
}

func (t *CIDRTrie) lookup(addr netip.Addr) (Route, int, bool) {
	if t == nil || !addr.IsValid() {
		return "", 0, false
	}

	addr = addr.Unmap()

	var (
		node    *CIDRNode
		raw     []byte
		maxBits int
	)

	if addr.Is4() {
		node = t.ipv4

		v := addr.As4()
		raw = v[:]

		maxBits = 32
	} else if addr.Is6() {
		node = t.ipv6

		v := addr.As16()
		raw = v[:]

		maxBits = 128
	} else {
		return RouteDirect, 0, false
	}

	if node == nil {
		return RouteDirect, 0, false
	}

	var (
		bestRoute Route
		bestBits  int
		found     bool
	)

	if node.hasRoute {
		bestRoute = node.route
		bestBits = 0
		found = true
	}

	for i := 0; i < maxBits; i++ {
		bit := (raw[i/8] >> uint(7-(i%8))) & 1

		node = node.children[bit]
		if node == nil {
			break
		}

		if node.hasRoute {
			bestRoute = node.route
			bestBits = i + 1
			found = true
		}
	}

	if !found {
		return RouteDirect, 0, false
	}

	return bestRoute, bestBits, true
}

type DomainNode struct {
	children map[string]*DomainNode

	wildcardRoute    Route
	hasWildcardRoute bool
}

type DomainTrie struct {
	root *DomainNode
}

func newDomainTrie() *DomainTrie {
	return &DomainTrie{
		root: &DomainNode{
			children: make(map[string]*DomainNode),
		},
	}
}

func (t *DomainTrie) insertWildcard(pattern string, route Route) {
	if t == nil || t.root == nil {
		return
	}

	if !strings.HasPrefix(pattern, "*.") {
		return
	}

	suffix := pattern[2:]
	if suffix == "" {
		return
	}

	labels := strings.Split(suffix, ".")
	node := t.root

	for i := len(labels) - 1; i >= 0; i-- {
		label := labels[i]

		if label == "" {
			return
		}

		if node.children == nil {
			node.children = make(map[string]*DomainNode)
		}

		child := node.children[label]

		if child == nil {
			child = &DomainNode{
				children: make(map[string]*DomainNode),
			}

			node.children[label] = child
		}

		node = child
	}

	node.wildcardRoute = route
	node.hasWildcardRoute = true
}

func (t *DomainTrie) lookupWildcard(host string) (Route, bool) {
	if t == nil || t.root == nil || host == "" {
		return "", false
	}

	labels := strings.Split(host, ".")
	node := t.root

	var (
		bestRoute Route
		found     bool
	)

	matchedSuffixLabels := 0

	for i := len(labels) - 1; i >= 0; i-- {
		label := labels[i]

		if label == "" {
			return "", false
		}

		child := node.children[label]
		if child == nil {
			break
		}

		node = child
		matchedSuffixLabels++

		if node.hasWildcardRoute && matchedSuffixLabels < len(labels) {
			bestRoute = node.wildcardRoute
			found = true
		}
	}

	if !found {
		return "", false
	}

	return bestRoute, true
}

const (
	dnsCacheTTL        = 60 * time.Second
	dnsNegativeTTL     = 10 * time.Second
	dnsCacheMaxEntries = 100_000
	dnsCleanupInterval = 30 * time.Second
	dnsLookupTimeout   = 5 * time.Second
)

type dnsCacheEntry struct {
	ips       []netip.Addr
	expiresAt time.Time
	ok        bool
}

type DNSCache struct {
	mu sync.RWMutex

	entries map[string]dnsCacheEntry

	maxEntries  int
	ttl         time.Duration
	negativeTTL time.Duration

	group singleflight.Group

	stopOnce sync.Once
	stop     chan struct{}
}

func newDNSCache(
	maxEntries int,
	ttl time.Duration,
	negativeTTL time.Duration,
) *DNSCache {
	if maxEntries < 1 {
		maxEntries = 1
	}

	if ttl <= 0 {
		ttl = dnsCacheTTL
	}

	if negativeTTL <= 0 {
		negativeTTL = dnsNegativeTTL
	}

	c := &DNSCache{
		entries:    make(map[string]dnsCacheEntry),
		maxEntries: maxEntries,
		ttl:        ttl,
		negativeTTL: negativeTTL,
		stop:       make(chan struct{}),
	}

	go c.cleanupLoop()

	return c
}

func (c *DNSCache) normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))

	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}

	return host
}

func (c *DNSCache) get(host string) ([]netip.Addr, bool, bool) {
	if c == nil {
		return nil, false, false
	}

	host = c.normalizeHost(host)

	if host == "" {
		return nil, false, false
	}

	now := time.Now()

	c.mu.RLock()
	entry, ok := c.entries[host]
	c.mu.RUnlock()

	if !ok {
		return nil, false, false
	}

	if now.Before(entry.expiresAt) {
		return entry.ips, entry.ok, true
	}

	return nil, false, false
}

func (c *DNSCache) set(
	host string,
	ips []netip.Addr,
	ok bool,
) {
	if c == nil {
		return
	}

	host = c.normalizeHost(host)

	if host == "" {
		return
	}

	ttl := c.ttl

	if !ok {
		ttl = c.negativeTTL
	}

	copiedIPs := make([]netip.Addr, len(ips))
	copy(copiedIPs, ips)

	entry := dnsCacheEntry{
		ips:       copiedIPs,
		expiresAt: time.Now().Add(ttl),
		ok:        ok,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[host]; exists {
		c.entries[host] = entry
		return
	}

	if len(c.entries) >= c.maxEntries {
		c.removeExpiredLocked()
	}

	if len(c.entries) >= c.maxEntries {
		for key := range c.entries {
			delete(c.entries, key)
			break
		}
	}

	c.entries[host] = entry
}

func (c *DNSCache) removeExpiredLocked() {
	now := time.Now()

	for host, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, host)
		}
	}
}

func (c *DNSCache) cleanupLoop() {
	ticker := time.NewTicker(dnsCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()

			before := len(c.entries)
			c.removeExpiredLocked()
			removed := before - len(c.entries)

			c.mu.Unlock()

			if removed > 0 {
				debugf(
					"[ROUTE] DNS cache cleanup removed=%d remaining=%d",
					removed,
					before-removed,
				)
			}

		case <-c.stop:
			return
		}
	}
}

func (c *DNSCache) close() {
	if c == nil {
		return
	}

	c.stopOnce.Do(func() {
		close(c.stop)
	})
}

func (c *DNSCache) lookup(host string) ([]netip.Addr, bool) {
	if c == nil {
		return nil, false
	}

	host = c.normalizeHost(host)

	if host == "" {
		return nil, false
	}

	if ips, ok, found := c.get(host); found {
		debugf(
			"[ROUTE] DNS cache hit host=%s ips=%d ok=%t",
			host,
			len(ips),
			ok,
		)

		return ips, ok
	}

	debugf(
		"[ROUTE] DNS cache miss host=%s",
		host,
	)

	value, err, shared := c.group.Do(host, func() (any, error) {
		if ips, ok, found := c.get(host); found {
			debugf(
				"[ROUTE] DNS cache filled while waiting host=%s",
				host,
			)

			return dnsCacheEntry{
				ips: ips,
				ok:  ok,
			}, nil
		}

		ctx, cancel := context.WithTimeout(
			context.Background(),
			dnsLookupTimeout,
		)
		defer cancel()

		debugf(
			"[ROUTE] DNS lookup started host=%s",
			host,
		)

		start := time.Now()

		ips, err := net.DefaultResolver.LookupNetIP(
			ctx,
			"ip",
			host,
		)

		if err != nil {
			c.set(host, nil, false)

			warnf(
				"[ROUTE] DNS lookup failed host=%s duration=%s: %v",
				host,
				time.Since(start),
				err,
			)

			return dnsCacheEntry{
				ok: false,
			}, nil
		}

		c.set(host, ips, true)

		debugf(
			"[ROUTE] DNS lookup completed host=%s ips=%d duration=%s",
			host,
			len(ips),
			time.Since(start),
		)

		return dnsCacheEntry{
			ips: ips,
			ok:  true,
		}, nil
	})

	if err != nil {
		warnf(
			"[ROUTE] DNS singleflight failed host=%s: %v",
			host,
			err,
		)

		return nil, false
	}

	if shared {
		debugf(
			"[ROUTE] DNS lookup result shared host=%s",
			host,
		)
	}

	entry, ok := value.(dnsCacheEntry)
	if !ok {
		warnf(
			"[ROUTE] invalid DNS cache result host=%s",
			host,
		)

		return nil, false
	}

	return entry.ips, entry.ok
}

type RouteCache struct {
	Domains         map[string]Route
	WildcardDomains *DomainTrie
	CIDRs           *CIDRTrie
	DNS             *DNSCache
	DefaultRoute    Route
}

var (
	routes atomic.Value

	dnsCache = newDNSCache(
		dnsCacheMaxEntries,
		dnsCacheTTL,
		dnsNegativeTTL,
	)
)

func resolveCIDRRoute(host string, cache RouteCache) (Route, bool) {
	if cache.CIDRs == nil || cache.DNS == nil {
		return "", false
	}

	ips, ok := cache.DNS.lookup(host)
	if !ok {
		debugf(
			"[ROUTE] CIDR lookup skipped because DNS failed host=%s",
			host,
		)

		return "", false
	}

	var (
		bestRoute Route
		bestBits  int
		found     bool
	)

	for _, addr := range ips {
		route, bits, ok := cache.CIDRs.lookup(addr)
		if !ok {
			continue
		}

		debugf(
			"[ROUTE] CIDR match host=%s ip=%s route=%s prefix=%d",
			host,
			addr,
			route,
			bits,
		)

		if !found || bits > bestBits {
			bestRoute = route
			bestBits = bits
			found = true
		}
	}

	if !found {
		return "", false
	}

	return bestRoute, true
}

func ResolveRoute(host string) Route {
	host = strings.ToLower(strings.TrimSpace(host))

	if host == "" {
		warnf("[ROUTE] empty host, using direct route")
		return RouteDirect
	}

	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}

	value := routes.Load()
	if value == nil {
		warnf(
			"[ROUTE] route cache is empty host=%s, using direct route",
			host,
		)

		return RouteDirect
	}

	cache := value.(RouteCache)

	if route, ok := cache.Domains[host]; ok {
		infof(
			"[ROUTE] exact domain match host=%s route=%s",
			host,
			route,
		)

		return route
	}

	if route, ok := cache.WildcardDomains.lookupWildcard(host); ok {
		infof(
			"[ROUTE] wildcard domain match host=%s route=%s",
			host,
			route,
		)

		return route
	}

	if addr, err := netip.ParseAddr(host); err == nil {
		route, bits, found := cache.CIDRs.lookup(addr)

		if found {
			infof(
				"[ROUTE] direct IP CIDR match host=%s route=%s prefix=%d",
				host,
				route,
				bits,
			)

			return route
		}

		debugf(
			"[ROUTE] direct IP has no CIDR match host=%s",
			host,
		)

		return cache.DefaultRoute
	}

	if route, ok := resolveCIDRRoute(host, cache); ok {
		infof(
			"[ROUTE] DNS CIDR match host=%s route=%s",
			host,
			route,
		)

		return route
	}

	infof(
		"[ROUTE] no specific match host=%s using default route=%s",
		host,
		cache.DefaultRoute,
	)

	return cache.DefaultRoute
}

func loadRoutes() error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	infof("[ROUTE] loading routes from Redis")

	defaultRoute := RouteDirect

	redisDefault, err := rdb.Get(
		ctx,
		DefaultRouteRedisKey,
	).Result()
	if err != nil && err != redis.Nil {
		warnf(
			"[ROUTE] failed to load default route from Redis: %v",
			err,
		)

		return err
	}

	redisDefault = strings.TrimSpace(redisDefault)

	if redisDefault != "" {
		defaultRoute = Route(redisDefault)
	}

	infof(
		"[ROUTE] default route=%s",
		defaultRoute,
	)

	domains, err := rdb.HGetAll(
		ctx,
		DomainsRedisKey,
	).Result()
	if err != nil {
		warnf(
			"[ROUTE] failed to load domains from Redis: %v",
			err,
		)

		return err
	}

	domainMap := make(map[string]Route, len(domains))
	wildcardTrie := newDomainTrie()

	exactDomainCount := 0
	wildcardDomainCount := 0

	for pattern, route := range domains {
		pattern = strings.ToLower(strings.TrimSpace(pattern))

		if pattern == "" {
			continue
		}

		routeValue := Route(strings.TrimSpace(route))

		if routeValue == "" {
			continue
		}

		if strings.HasPrefix(pattern, "*.") {
			wildcardTrie.insertWildcard(
				pattern,
				routeValue,
			)

			wildcardDomainCount++
			continue
		}

		domainMap[pattern] = routeValue
		exactDomainCount++
	}

	infof(
		"[ROUTE] loaded domains exact=%d wildcard=%d",
		exactDomainCount,
		wildcardDomainCount,
	)

	cidrs, err := rdb.HGetAll(
		ctx,
		CidrsRedisKey,
	).Result()
	if err != nil {
		warnf(
			"[ROUTE] failed to load CIDRs from Redis: %v",
			err,
		)

		return err
	}

	cidrTrie := &CIDRTrie{}
	cidrCount := 0
	invalidCIDRCount := 0

	for cidr, route := range cidrs {
		cidr = strings.TrimSpace(cidr)

		if cidr == "" {
			continue
		}

		routeValue := Route(strings.TrimSpace(route))

		if routeValue == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			warnf(
				"[ROUTE] invalid CIDR %q: %v",
				cidr,
				err,
			)

			invalidCIDRCount++
			continue
		}

		prefix = prefix.Masked()

		addr := prefix.Addr()

		if addr.Is6() {
			unmapped := addr.Unmap()

			if unmapped.Is4() {
				bits := prefix.Bits()

				if bits >= 96 {
					prefix = netip.PrefixFrom(
						unmapped,
						bits-96,
					).Masked()
				}
			}
		}

		cidrTrie.insert(
			prefix,
			routeValue,
		)

		cidrCount++
	}

	infof(
		"[ROUTE] loaded CIDRs valid=%d invalid=%d",
		cidrCount,
		invalidCIDRCount,
	)

	newCache := RouteCache{
		Domains:         domainMap,
		WildcardDomains: wildcardTrie,
		CIDRs:           cidrTrie,
		DNS:             dnsCache,
		DefaultRoute:    defaultRoute,
	}

	routes.Store(newCache)

	infof(
		"[ROUTE] routes loaded successfully exact=%d wildcard=%d cidrs=%d default=%s",
		exactDomainCount,
		wildcardDomainCount,
		cidrCount,
		defaultRoute,
	)

	return nil
}

func init() {
	if RouteDirect == "" {
		errorf("DEFAULT_GROUP_NAME environment variable is required")
		os.Exit(1)
	}

	routes.Store(RouteCache{
		Domains:         make(map[string]Route),
		WildcardDomains: newDomainTrie(),
		CIDRs:            &CIDRTrie{},
		DNS:             dnsCache,
		DefaultRoute:    RouteDirect,
	})
}
