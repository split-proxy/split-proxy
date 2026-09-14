package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UDPDirectSession struct {
	conn   *net.UDPConn
	target string

	mu     sync.Mutex
	closed bool
}

func (s *UDPDirectSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true

	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *UDPDirectSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

type UDPBrokerSession struct {
	broker *BrokerSession
	target string

	mu     sync.Mutex
	closed bool
}

func (s *UDPBrokerSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	s.closed = true

	if s.broker != nil {
		closeBrokerSession(s.broker)
	}
}

func (s *UDPBrokerSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func handleSOCKS5UDPAssociate(client net.Conn) {
	remoteAddr := client.RemoteAddr().String()

	infof(
		"[SOCKS5] UDP ASSOCIATE started remote=%s",
		remoteAddr,
	)

	// 1. Создаём постоянный UDP relay socket.

	udpConn, _, err := listenUDPInRange(
		UDPRelayMinPort,
		UDPRelayMaxPort,
	)
	if err != nil {
		warnf(
			"[SOCKS5] UDP relay listen failed remote=%s: %v",
			remoteAddr,
			err,
		)
		return
	}
	defer udpConn.Close()

	localAddr := udpConn.LocalAddr().(*net.UDPAddr)

	infof(
		"[SOCKS5] UDP relay listening remote=%s relay=%s",
		remoteAddr,
		localAddr,
	)

	// 2. Адрес, который отдаём SOCKS5 клиенту.

	relayAddr := &net.UDPAddr{
		IP:   net.ParseIP(UDPRelayHost),
		Port: localAddr.Port,
	}

	if err := writeSOCKS5UDPAssociateSuccess(
		client,
		relayAddr,
	); err != nil {
		warnf(
			"[SOCKS5] UDP ASSOCIATE response failed remote=%s relay=%s: %v",
			remoteAddr,
			relayAddr,
			err,
		)
		return
	}

	infof(
		"[SOCKS5] UDP ASSOCIATE established remote=%s relay=%s",
		remoteAddr,
		relayAddr,
	)

	// 3. Адрес SOCKS5 клиента.
	//
	// Первый UDP-пакет привязывает ASSOCIATE session к source IP:port.
	// После этого UDP-пакеты от другого source не принимаются.
	//
	// Это намеренно IP:port binding, а не только IP binding:
	//
	//   first:
	//     203.0.113.25:52143 -> bind
	//
	//   subsequent:
	//     203.0.113.25:52143 -> ACCEPT
	//     203.0.113.25:52144 -> REJECT
	//     198.51.100.10:12345 -> REJECT
	//
	// При смене сети (например Wi-Fi -> mobile) IP изменится,
	// поэтому клиент должен создать новый UDP ASSOCIATE.

	var clientUDPAddr *net.UDPAddr
	var clientAddrMu sync.RWMutex

	bindClientUDPAddr := func(addr *net.UDPAddr) bool {
		if addr == nil {
			return false
		}

		clientAddrMu.Lock()
		defer clientAddrMu.Unlock()

		// Первый UDP-пакет привязывает session к client IP:port.

		if clientUDPAddr == nil {
			clientUDPAddr = &net.UDPAddr{
				IP:   append(net.IP(nil), addr.IP...),
				Port: addr.Port,
				Zone: addr.Zone,
			}

			infof(
				"[SOCKS5] UDP client bound remote=%s client=%s",
				remoteAddr,
				clientUDPAddr,
			)

			return true
		}

		// После binding разрешаем только тот же IP:port.
		//
		// Source port важен: два клиента за одним NAT могут иметь
		// один публичный IP, но разные UDP source ports.

		if !clientUDPAddr.IP.Equal(addr.IP) ||
			clientUDPAddr.Port != addr.Port ||
			clientUDPAddr.Zone != addr.Zone {
			warnf(
				"[SOCKS5] UDP packet rejected: client binding mismatch remote=%s expected=%s actual=%s",
				remoteAddr,
				clientUDPAddr,
				addr,
			)

			return false
		}

		return true
	}

	getClientUDPAddr := func() *net.UDPAddr {
		clientAddrMu.RLock()
		defer clientAddrMu.RUnlock()

		if clientUDPAddr == nil {
			return nil
		}

		return &net.UDPAddr{
			IP:   append(net.IP(nil), clientUDPAddr.IP...),
			Port: clientUDPAddr.Port,
			Zone: clientUDPAddr.Zone,
		}
	}

	// 4. UDP sessions.
	//
	// Одна broker session = один route + target.
	// Одна direct session = один concrete IP + port.
	//
	// Для DIRECT hostname сначала разрешается DNS:
	//
	// hostname -> DNS cache -> allowed IPs -> concrete IP:port
	//
	// После этого session работает только с concrete IP:port.

	var sessionsMu sync.Mutex

	brokerSessions := make(map[string]*UDPBrokerSession)
	directSessions := make(map[string]*UDPDirectSession)

	getSessionKey := func(route Route, target string) string {
		return string(route) + "|" + target
	}

	closeAllSessions := func() {
		sessionsMu.Lock()
		defer sessionsMu.Unlock()

		for key, session := range brokerSessions {
			session.close()
			delete(brokerSessions, key)
		}

		for key, session := range directSessions {
			session.close()
			delete(directSessions, key)
		}

		debugf(
			"[SOCKS5] UDP sessions closed remote=%s",
			remoteAddr,
		)
	}

	defer closeAllSessions()

	// 5. DNS + DIRECT target policy.
	//
	// Вариант 2:
	//
	// Если hostname имеет несколько IP:
	//
	//     IP1 private       -> reject
	//     IP2 public        -> allow
	//     IP3 public        -> allow
	//
	// Используются только разрешённые адреса.

	resolveDirectUDPTargets := func(
		host string,
	) ([]netip.Addr, error) {
		host = strings.TrimSpace(host)

		if host == "" {
			return nil, fmt.Errorf(
				"empty UDP target host",
			)
		}

		// Literal IP.
		//
		// DNS не нужен.

		if ip, err := netip.ParseAddr(host); err == nil {
			ip = ip.Unmap()

			if err := validateDirectTargetIP(ip); err != nil {
				return nil, err
			}

			return []netip.Addr{ip}, nil
		}

		// Hostname.
		//
		// Используем существующий DNS cache.
		// Это тот же cache, который используется routing.go.

		ips, ok := dnsCache.lookup(host)
		if !ok {
			return nil, fmt.Errorf(
				"DNS resolution failed for %q",
				host,
			)
		}

		if len(ips) == 0 {
			return nil, fmt.Errorf(
				"DNS returned no addresses for %q",
				host,
			)
		}

		allowed := make(
			[]netip.Addr,
			0,
			len(ips),
		)

		for _, ip := range ips {
			ip = ip.Unmap()

			if err := validateDirectTargetIP(ip); err != nil {
				debugf(
					"[UDP DIRECT] target IP rejected host=%s ip=%s: %v",
					host,
					ip,
					err,
				)
				continue
			}

			allowed = append(
				allowed,
				ip,
			)
		}

		if len(allowed) == 0 {
			return nil, fmt.Errorf(
				"all resolved addresses for %q are blocked by DIRECT target policy",
				host,
			)
		}

		return allowed, nil
	}

	// 6. Создание DIRECT UDP session.
	//
	// ВАЖНО:
	//
	// target здесь обязан быть concrete IP:port.
	// DNS lookup внутри этой функции отсутствует.

	getOrCreateDirectSession := func(
		target string,
	) (*UDPDirectSession, error) {
		key := target

		sessionsMu.Lock()

		if session, ok := directSessions[key]; ok {
			if !session.isClosed() {
				sessionsMu.Unlock()

				debugf(
					"[UDP] reusing direct session remote=%s target=%s",
					remoteAddr,
					target,
				)

				return session, nil
			}

			delete(directSessions, key)
		}

		sessionsMu.Unlock()

		infof(
			"[UDP] opening direct session remote=%s target=%s",
			remoteAddr,
			target,
		)

		host, portString, err := net.SplitHostPort(target)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid UDP target %s: %w",
				target,
				err,
			)
		}

		// После DNS resolution здесь должен быть именно IP.

		targetIP, err := netip.ParseAddr(host)
		if err != nil {
			return nil, fmt.Errorf(
				"direct UDP target is not a concrete IP %q",
				host,
			)
		}

		targetIP = targetIP.Unmap()

		// Повторная проверка является намеренной.
		//
		// Даже если target пришёл из resolveDirectUDPTargets(),
		// session creation не должна позволять создать socket
		// на запрещённом адресе.

		if err := validateDirectTargetIP(targetIP); err != nil {
			return nil, err
		}

		port, err := strconv.Atoi(portString)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf(
				"invalid UDP target port %q",
				portString,
			)
		}

		targetAddr := &net.UDPAddr{
			IP:   net.IP(targetIP.AsSlice()),
			Port: port,
		}

		// Здесь используется concrete IP.
		//
		// net.DialUDP() не выполняет дополнительный DNS lookup.

		conn, err := net.DialUDP(
			"udp",
			nil,
			targetAddr,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"direct UDP connect %s: %w",
				target,
				err,
			)
		}

		session := &UDPDirectSession{
			conn:   conn,
			target: target,
		}

		// Защита от duplicate session.

		sessionsMu.Lock()

		if existing, ok := directSessions[key]; ok {
			if !existing.isClosed() {
				sessionsMu.Unlock()

				_ = conn.Close()

				debugf(
					"[UDP] duplicate direct session, reusing existing remote=%s target=%s",
					remoteAddr,
					target,
				)

				return existing, nil
			}

			delete(directSessions, key)
		}

		directSessions[key] = session

		sessionsMu.Unlock()

		infof(
			"[UDP] direct session opened remote=%s target=%s local=%s",
			remoteAddr,
			target,
			conn.LocalAddr(),
		)

		// Читаем destination -> proxy.

		go func() {
			defer func() {
				session.close()

				sessionsMu.Lock()

				if current, ok := directSessions[key]; ok &&
					current == session {
					delete(directSessions, key)
				}

				sessionsMu.Unlock()

				infof(
					"[UDP] direct session closed remote=%s target=%s",
					remoteAddr,
					target,
				)
			}()

			buffer := make([]byte, 64*1024)

			for {
				n, src, err := conn.ReadFromUDP(buffer)
				if err != nil {
					if !session.isClosed() {
						warnf(
							"[UDP] direct read failed remote=%s target=%s: %v",
							remoteAddr,
							target,
							err,
						)
					}

					return
				}

				if n == 0 {
					continue
				}

				dst := getClientUDPAddr()
				if dst == nil {
					debugf(
						"[UDP] direct response received before client address was known remote=%s target=%s",
						remoteAddr,
						target,
					)
					continue
				}

				socksResponse := buildSOCKS5UDPResponse(
					src,
					buffer[:n],
				)

				if len(socksResponse) == 0 {
					warnf(
						"[UDP] failed to build direct response remote=%s target=%s source=%s",
						remoteAddr,
						target,
						src,
					)
					continue
				}

				if _, err := udpConn.WriteToUDP(
					socksResponse,
					dst,
				); err != nil {
					warnf(
						"[UDP] direct response to client failed remote=%s target=%s client=%s: %v",
						remoteAddr,
						target,
						dst,
						err,
					)
					return
				}

				debugf(
					"[UDP] direct -> client remote=%s target=%s source=%s client=%s bytes=%d",
					remoteAddr,
					target,
					src,
					dst,
					n,
				)
			}
		}()

		return session, nil
	}

	sendDirectUDP := func(
		session *UDPDirectSession,
		payload []byte,
	) error {
		session.mu.Lock()
		defer session.mu.Unlock()

		if session.closed {
			return fmt.Errorf(
				"direct UDP session is closed",
			)
		}

		if _, err := session.conn.Write(payload); err != nil {
			return err
		}

		return nil
	}

	// 7. Создание BROKER UDP session.

	getOrCreateBrokerSession := func(
		route Route,
		target string,
	) (*UDPBrokerSession, error) {
		key := getSessionKey(route, target)

		sessionsMu.Lock()

		if session, ok := brokerSessions[key]; ok {
			if !session.isClosed() {
				sessionsMu.Unlock()

				debugf(
					"[UDP] reusing broker session remote=%s route=%s target=%s",
					remoteAddr,
					route,
					target,
				)

				return session, nil
			}

			delete(brokerSessions, key)
		}

		sessionsMu.Unlock()

		infof(
			"[UDP] opening broker session remote=%s route=%s target=%s",
			remoteAddr,
			route,
			target,
		)

		brokerSession, err := handleUDPBroker(
			route,
			target,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"UDP broker connect failed: %w",
				err,
			)
		}

		session := &UDPBrokerSession{
			broker: brokerSession,
			target: target,
		}

		// Защита от duplicate session.

		sessionsMu.Lock()

		if existing, ok := brokerSessions[key]; ok {
			if !existing.isClosed() {
				sessionsMu.Unlock()

				closeBrokerSession(brokerSession)

				debugf(
					"[UDP] duplicate broker session, reusing existing remote=%s route=%s target=%s",
					remoteAddr,
					route,
					target,
				)

				return existing, nil
			}

			delete(brokerSessions, key)
		}

		brokerSessions[key] = session

		sessionsMu.Unlock()

		infof(
			"[UDP] broker session opened remote=%s route=%s target=%s",
			remoteAddr,
			route,
			target,
		)

		// Читаем broker -> proxy.

		go func() {
			defer func() {
				session.close()

				sessionsMu.Lock()

				if current, ok := brokerSessions[key]; ok &&
					current == session {
					delete(brokerSessions, key)
				}

				sessionsMu.Unlock()

				infof(
					"[UDP] broker session closed remote=%s route=%s target=%s",
					remoteAddr,
					route,
					target,
				)
			}()

			for {
				select {
				case <-session.broker.done:
					if !session.isClosed() {
						warnf(
							"[UDP] broker session closed remote=%s route=%s target=%s",
							remoteAddr,
							route,
							target,
						)
					}
					return

				case payload, ok := <-session.broker.data:
					if !ok {
						return
					}

					debugf(
						"[UDP] broker -> proxy remote=%s route=%s target=%s bytes=%d",
						remoteAddr,
						route,
						session.target,
						len(payload),
					)

					if len(payload) == 0 {
						continue
					}

					dst := getClientUDPAddr()
					if dst == nil {
						debugf(
							"[UDP] broker response received before client address was known remote=%s target=%s",
							remoteAddr,
							session.target,
						)
						continue
					}

					socksResponse := buildSOCKS5UDPResponseFromTarget(
						session.target,
						payload,
					)

					if len(socksResponse) == 0 {
						warnf(
							"[UDP] failed to build broker response remote=%s target=%s",
							remoteAddr,
							session.target,
						)
						continue
					}

					if _, err := udpConn.WriteToUDP(
						socksResponse,
						dst,
					); err != nil {
						warnf(
							"[UDP] broker response to client failed remote=%s target=%s client=%s: %v",
							remoteAddr,
							session.target,
							dst,
							err,
						)
						return
					}

					debugf(
						"[UDP] broker -> client remote=%s target=%s client=%s bytes=%d",
						remoteAddr,
						session.target,
						dst,
						len(payload),
					)
				}
			}
		}()

		return session, nil
	}

	// 8. Следим за TCP control connection.

	tcpDone := make(chan struct{})

	go func() {
		defer close(tcpDone)

		buf := make([]byte, 1)

		for {
			_, err := client.Read(buf)
			if err != nil {
				debugf(
					"[SOCKS5] UDP control connection closed remote=%s: %v",
					remoteAddr,
					err,
				)
				return
			}
		}
	}()

	// 9. UDP relay loop.

	buffer := make([]byte, 64*1024)

	for {
		select {
		case <-tcpDone:
			infof(
				"[SOCKS5] UDP ASSOCIATE closed remote=%s",
				remoteAddr,
			)
			return

		default:
		}

		_ = udpConn.SetReadDeadline(
			time.Now().Add(1 * time.Second),
		)

		n, src, err := udpConn.ReadFromUDP(buffer)

		if err != nil {
			if netErr, ok := err.(net.Error); ok &&
				netErr.Timeout() {
				continue
			}

			warnf(
				"[UDP] relay read failed remote=%s: %v",
				remoteAddr,
				err,
			)
			return
		}

		if n == 0 {
			continue
		}

		packet, err := parseSOCKS5UDPRequest(buffer[:n])
		if err != nil {
			warnf(
				"[SOCKS5] invalid UDP packet remote=%s source=%s: %v",
				remoteAddr,
				src,
				err,
			)
			continue
		}

		// Первый UDP packet привязывает session к source IP:port.
		// Последующие packets от другого source отклоняются.

		if !bindClientUDPAddr(src) {
			continue
		}

		target := net.JoinHostPort(
			packet.Host,
			strconv.Itoa(int(packet.Port)),
		)

		debugf(
			"[SOCKS5] UDP request remote=%s source=%s target=%s bytes=%d",
			remoteAddr,
			src,
			target,
			len(packet.Payload),
		)

		// Определяем route.
		//
		// ВАЖНО:
		//
		// ResolveRoute() используется для routing decision.
		// DIRECT target policy применяется только после определения
		// RouteDirect.

		route := ResolveRoute(packet.Host)

		infof(
			"[SOCKS5] UDP route selected remote=%s route=%s target=%s",
			remoteAddr,
			route,
			target,
		)

		// ============================================================
		// DIRECT UDP
		// ============================================================

		if route == RouteDirect {
			// Вариант 2:
			//
			// hostname
			//     ↓
			// DNS cache
			//     ↓
			// [IP1, IP2, IP3]
			//     ↓
			// validateDirectTargetIP()
			//     ↓
			// [allowed IP1, allowed IP3]
			//     ↓
			// concrete UDP sessions
			//
			// Private/internal адреса здесь запрещены.
			// Worker/Broker route это ограничение не получает.

			allowedIPs, err := resolveDirectUDPTargets(
				packet.Host,
			)
			if err != nil {
				warnf(
					"[UDP DIRECT] target rejected remote=%s target=%s: %v",
					remoteAddr,
					target,
					err,
				)
				continue
			}

			sent := false
			var lastErr error

			for _, ip := range allowedIPs {
				concreteTarget := net.JoinHostPort(
					ip.String(),
					strconv.Itoa(int(packet.Port)),
				)

				session, err := getOrCreateDirectSession(
					concreteTarget,
				)
				if err != nil {
					lastErr = err

					debugf(
						"[UDP DIRECT] session failed remote=%s host=%s ip=%s port=%d: %v",
						remoteAddr,
						packet.Host,
						ip,
						packet.Port,
						err,
					)

					continue
				}

				if err := sendDirectUDP(
					session,
					packet.Payload,
				); err != nil {
					lastErr = err

					warnf(
						"[UDP] direct send failed remote=%s target=%s: %v",
						remoteAddr,
						concreteTarget,
						err,
					)

					session.close()

					key := concreteTarget

					sessionsMu.Lock()

					if current, ok := directSessions[key]; ok &&
						current == session {
						delete(directSessions, key)
					}

					sessionsMu.Unlock()

					continue
				}

				infof(
					"[UDP DIRECT] payload sent remote=%s host=%s ip=%s port=%d bytes=%d",
					remoteAddr,
					packet.Host,
					ip,
					packet.Port,
					len(packet.Payload),
				)

				sent = true
				break
			}

			if !sent && lastErr != nil {
				warnf(
					"[UDP DIRECT] failed to send to any allowed address remote=%s target=%s: %v",
					remoteAddr,
					target,
					lastErr,
				)
			}

			continue
		}

		// ============================================================
		// BROKER UDP
		// ============================================================
		//
		// НЕ вызываем validateDirectTargetIP().
		//
		// Worker является доверенной точкой выхода и должен иметь
		// возможность обращаться к private/internal targets.

		session, err := getOrCreateBrokerSession(
			route,
			target,
		)
		if err != nil {
			warnf(
				"[UDP] broker session failed remote=%s route=%s target=%s: %v",
				remoteAddr,
				route,
				target,
				err,
			)
			continue
		}

		session.mu.Lock()

		if session.closed {
			session.mu.Unlock()

			debugf(
				"[UDP] broker session already closed remote=%s route=%s target=%s",
				remoteAddr,
				route,
				target,
			)
			continue
		}

		err = session.broker.send(
			CmdUDPData,
			packet.Payload,
		)

		session.mu.Unlock()

		if err != nil {
			warnf(
				"[UDP] broker send failed remote=%s route=%s target=%s: %v",
				remoteAddr,
				route,
				target,
				err,
			)

			session.close()

			key := getSessionKey(route, target)

			sessionsMu.Lock()

			if current, ok := brokerSessions[key]; ok &&
				current == session {
				delete(brokerSessions, key)
			}

			sessionsMu.Unlock()

			continue
		}

		debugf(
			"[UDP] payload sent via broker remote=%s route=%s target=%s bytes=%d",
			remoteAddr,
			route,
			target,
			len(packet.Payload),
		)
	}
}

// UDP LISTENER

func listenUDPInRange(
	minPort,
	maxPort int,
) (*net.UDPConn, int, error) {
	for port := minPort; port <= maxPort; port++ {
		addr := &net.UDPAddr{
			IP:   net.IPv4zero,
			Port: port,
		}

		conn, err := net.ListenUDP(
			"udp4",
			addr,
		)
		if err == nil {
			debugf(
				"[UDP] relay socket opened port=%d",
				port,
			)
			return conn, port, nil
		}
	}

	return nil, 0, fmt.Errorf(
		"no free UDP port in range %d-%d",
		minPort,
		maxPort,
	)
}

// SOCKS5 UDP RESPONSE

func buildSOCKS5UDPResponseFromTarget(
	target string,
	payload []byte,
) []byte {
	host, portString, err := net.SplitHostPort(target)
	if err != nil {
		return nil
	}

	port, err := strconv.Atoi(portString)
	if err != nil || port < 0 || port > 65535 {
		return nil
	}

	ip := net.ParseIP(host)

	// IPv4.

	if ip4 := ip.To4(); ip4 != nil {
		result := make(
			[]byte,
			0,
			10+len(payload),
		)

		result = append(
			result,
			0x00, // RSV
			0x00, // RSV
			0x00, // FRAG
			0x01, // ATYP IPv4
		)

		result = append(
			result,
			ip4[0],
			ip4[1],
			ip4[2],
			ip4[3],
		)

		result = append(
			result,
			byte(port>>8),
			byte(port),
		)

		result = append(
			result,
			payload...,
		)

		return result
	}

	// IPv6.

	if ip6 := ip.To16(); ip6 != nil {
		result := make(
			[]byte,
			0,
			22+len(payload),
		)

		result = append(
			result,
			0x00, // RSV
			0x00, // RSV
			0x00, // FRAG
			0x04, // ATYP IPv6
		)

		result = append(
			result,
			ip6...,
		)

		result = append(
			result,
			byte(port>>8),
			byte(port),
		)

		result = append(
			result,
			payload...,
		)

		return result
	}

	// DOMAIN.

	if len(host) > 255 {
		return nil
	}

	result := make(
		[]byte,
		0,
		7+len(host)+len(payload),
	)

	result = append(
		result,
		0x00, // RSV
		0x00, // RSV
		0x00, // FRAG
		0x03, // ATYP DOMAIN
		byte(len(host)),
	)

	result = append(
		result,
		host...,
	)

	result = append(
		result,
		byte(port>>8),
		byte(port),
	)

	result = append(
		result,
		payload...,
	)

	return result
}

// SOCKS5 UDP response for direct routing.

func buildSOCKS5UDPResponse(
	addr *net.UDPAddr,
	payload []byte,
) []byte {
	if addr == nil {
		return nil
	}

	// IPv4.

	if ip4 := addr.IP.To4(); ip4 != nil {
		result := make(
			[]byte,
			0,
			10+len(payload),
		)

		result = append(
			result,
			0x00, // RSV
			0x00, // RSV
			0x00, // FRAG
			0x01, // ATYP IPv4
		)

		result = append(
			result,
			ip4[0],
			ip4[1],
			ip4[2],
			ip4[3],
		)

		result = append(
			result,
			byte(addr.Port>>8),
			byte(addr.Port),
		)

		result = append(
			result,
			payload...,
		)

		return result
	}

	// IPv6.

	if ip6 := addr.IP.To16(); ip6 != nil {
		result := make(
			[]byte,
			0,
			22+len(payload),
		)

		result = append(
			result,
			0x00, // RSV
			0x00, // RSV
			0x00, // FRAG
			0x04, // ATYP IPv6
		)

		result = append(
			result,
			ip6...,
		)

		result = append(
			result,
			byte(addr.Port>>8),
			byte(addr.Port),
		)

		result = append(
			result,
			payload...,
		)

		return result
	}

	return nil
}

// SOCKS5 UDP ASSOCIATE RESPONSE

func writeSOCKS5UDPAssociateSuccess(
	client net.Conn,
	addr *net.UDPAddr,
) error {
	ip := addr.IP.To4()

	if ip == nil {
		return fmt.Errorf(
			"only IPv4 supported for UDP reply",
		)
	}

	response := []byte{
		0x05,
		0x00,
		0x00,
		0x01,

		ip[0],
		ip[1],
		ip[2],
		ip[3],

		byte(addr.Port >> 8),
		byte(addr.Port),
	}

	_, err := client.Write(response)

	return err
}

// SOCKS5 UDP REQUEST PARSER

func parseSOCKS5UDPRequest(
	data []byte,
) (*SOCKS5UDPPacket, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf(
			"SOCKS5 UDP packet too short",
		)
	}

	// RSV.

	if data[0] != 0x00 || data[1] != 0x00 {
		return nil, fmt.Errorf(
			"invalid SOCKS5 UDP RSV",
		)
	}

	// FRAG.

	if data[2] != 0x00 {
		return nil, fmt.Errorf(
			"UDP fragmentation is not supported",
		)
	}

	packet := &SOCKS5UDPPacket{
		ATYP: data[3],
	}

	offset := 4

	switch packet.ATYP {
	case 0x01:
		// IPv4.

		if len(data) < offset+4 {
			return nil, fmt.Errorf(
				"invalid IPv4 address",
			)
		}

		packet.Host = net.IP(
			data[offset : offset+4],
		).String()

		offset += 4

	case 0x03:
		// DOMAIN.

		if len(data) < offset+1 {
			return nil, fmt.Errorf(
				"missing domain length",
			)
		}

		length := int(data[offset])
		offset++

		if len(data) < offset+length {
			return nil, fmt.Errorf(
				"invalid domain",
			)
		}

		packet.Host = string(
			data[offset : offset+length],
		)

		offset += length

	case 0x04:
		// IPv6.

		if len(data) < offset+16 {
			return nil, fmt.Errorf(
				"invalid IPv6 address",
			)
		}

		packet.Host = net.IP(
			data[offset : offset+16],
		).String()

		offset += 16

	default:
		return nil, fmt.Errorf(
			"unsupported ATYP: %d",
			packet.ATYP,
		)
	}

	// PORT.

	if len(data) < offset+2 {
		return nil, fmt.Errorf(
		"missing port",
		)
	}

	packet.Port = binary.BigEndian.Uint16(
		data[offset : offset+2],
	)

	offset += 2

	// PAYLOAD.

	packet.Payload = data[offset:]

	return packet, nil
}
