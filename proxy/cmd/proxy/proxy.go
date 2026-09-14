package main

import (
	"bufio"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type BufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *BufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func handleConn(client net.Conn, authStore *AuthStore) {
	defer client.Close()

	remoteAddr := client.RemoteAddr().String()

	infof(
		"[PROXY] client connected remote=%s",
		remoteAddr,
	)

	reader := bufio.NewReader(client)

	_ = client.SetReadDeadline(
		time.Now().Add(ProtocolDetectionTimeout),
	)

	protocol, err := detectProtocol(reader)
	if err != nil {
		warnf(
			"[PROXY] protocol detection failed remote=%s: %v",
			remoteAddr,
			err,
		)

		writeProxyError(client, http.StatusBadRequest)
		return
	}

	infof(
		"[PROXY] protocol detected remote=%s protocol=%s",
		remoteAddr,
		protocol,
	)

	switch protocol {

	case ProtocolHTTP:
		_ = client.SetReadDeadline(time.Time{})

		infof(
			"[PROXY] starting HTTP proxy remote=%s",
			remoteAddr,
		)

		handleHTTPProxy(
			client,
			reader,
			authStore,
		)

		infof(
			"[PROXY] HTTP proxy finished remote=%s",
			remoteAddr,
		)

	case ProtocolSOCKS5:
		_ = client.SetReadDeadline(time.Time{})

		infof(
			"[PROXY] starting SOCKS5 proxy remote=%s",
			remoteAddr,
		)

		handleSOCKS5(
			client,
			reader,
			authStore,
		)

		infof(
			"[PROXY] SOCKS5 proxy finished remote=%s",
			remoteAddr,
		)

	case ProtocolTLS:
		infof(
			"[PROXY] starting TLS handshake remote=%s",
			remoteAddr,
		)

		handleTLS(
			client,
			reader,
			authStore,
		)

	default:
		warnf(
			"[PROXY] unsupported protocol remote=%s protocol=%s",
			remoteAddr,
			protocol,
		)

		writeProxyError(client, http.StatusBadRequest)
	}

	infof(
		"[PROXY] client disconnected remote=%s",
		remoteAddr,
	)
}

func handleTLS(
	client net.Conn,
	reader *bufio.Reader,
	authStore *AuthStore,
) {
	remoteAddr := client.RemoteAddr().String()

	tlsConn := tls.Server(
		&BufferedConn{
			Conn:   client,
			reader: reader,
		},
		tlsConfig,
	)

	if err := tlsConn.Handshake(); err != nil {
		warnf(
			"[PROXY] TLS handshake failed remote=%s: %v",
			remoteAddr,
			err,
		)

		return
	}

	infof(
		"[PROXY] TLS handshake completed remote=%s",
		remoteAddr,
	)

	_ = tlsConn.SetReadDeadline(time.Time{})

	tlsReader := bufio.NewReader(tlsConn)

	protocol, err := detectProtocol(tlsReader)
	if err != nil {
		warnf(
			"[PROXY] TLS protocol detection failed remote=%s: %v",
			remoteAddr,
			err,
		)

		return
	}

	infof(
		"[PROXY] TLS protocol detected remote=%s protocol=%s",
		remoteAddr,
		protocol,
	)

	switch protocol {

	case ProtocolHTTP:
		infof(
			"[PROXY] starting HTTPS proxy remote=%s",
			remoteAddr,
		)

		handleHTTPProxy(
			tlsConn,
			tlsReader,
			authStore,
		)

		infof(
			"[PROXY] HTTPS proxy finished remote=%s",
			remoteAddr,
		)

	case ProtocolSOCKS5:
		infof(
			"[PROXY] starting SOCKS5 over TLS remote=%s",
			remoteAddr,
		)

		handleSOCKS5(
			tlsConn,
			tlsReader,
			authStore,
		)

		infof(
			"[PROXY] SOCKS5 over TLS finished remote=%s",
			remoteAddr,
		)

	default:
		warnf(
			"[PROXY] unsupported protocol over TLS remote=%s protocol=%s",
			remoteAddr,
			protocol,
		)
	}
}
