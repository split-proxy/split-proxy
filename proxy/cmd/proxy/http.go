package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

func writeProxyError(client net.Conn, status int) {
	statusText := http.StatusText(status)
	if statusText == "" {
		statusText = "Error"
	}

	warnf(
		"[HTTP] sending proxy error status=%d text=%s remote=%s",
		status,
		statusText,
		client.RemoteAddr(),
	)

	response := fmt.Sprintf(
		"HTTP/1.1 %d %s\r\n"+
			"Connection: close\r\n"+
			"Content-Length: 0\r\n"+
			"\r\n",
		status,
		statusText,
	)

	_, _ = client.Write([]byte(response))
}

func writeProxyAuthRequired(client net.Conn) {
	warnf(
		"[HTTP] proxy authentication required remote=%s",
		client.RemoteAddr(),
	)

	_, _ = client.Write([]byte(
		"HTTP/1.1 407 Proxy Authentication Required\r\n" +
			"Proxy-Authenticate: Basic realm=\"Proxy\"\r\n" +
			"Connection: close\r\n" +
			"Content-Length: 0\r\n" +
			"\r\n",
	))
}

func parseConnectTarget(req *http.Request) (string, string, error) {
	target := req.RequestURI

	if target == "" {
		target = req.Host
	}

	if target == "" {
		return "", "", fmt.Errorf("empty CONNECT target")
	}

	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return "", "", fmt.Errorf(
			"invalid CONNECT target %q: %w",
			target,
			err,
		)
	}

	if host == "" {
		return "", "", fmt.Errorf("empty CONNECT host")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return "", "", fmt.Errorf(
			"invalid CONNECT port %q",
			port,
		)
	}

	return host, port, nil
}

func handleHTTPProxy(
	client net.Conn,
	reader *bufio.Reader,
	authStore *AuthStore,
) {
	remoteAddr := client.RemoteAddr().String()

	req, err := http.ReadRequest(reader)
	if err != nil {
		warnf(
			"[HTTP] request parse failed remote=%s: %v",
			remoteAddr,
			err,
		)

		writeProxyError(client, http.StatusBadRequest)
		return
	}

	_ = client.SetReadDeadline(time.Time{})
	defer req.Body.Close()

	infof(
		"[HTTP] request received remote=%s method=%s host=%s",
		remoteAddr,
		req.Method,
		req.Host,
	)

	if !authStore.CheckAuth(req) {
		warnf(
			"[HTTP] authentication failed remote=%s method=%s host=%s",
			remoteAddr,
			req.Method,
			req.Host,
		)

		writeProxyAuthRequired(client)
		return
	}

	infof(
		"[HTTP] authentication successful remote=%s",
		remoteAddr,
	)

	switch req.Method {

	case http.MethodConnect:
		host, port, err := parseConnectTarget(req)
		if err != nil {
			warnf(
				"[HTTP] invalid CONNECT target remote=%s host=%s: %v",
				remoteAddr,
				req.Host,
				err,
			)

			writeProxyError(client, http.StatusBadRequest)
			return
		}

		address := net.JoinHostPort(host, port)

		route := ResolveRoute(host)

		infof(
			"[HTTP] CONNECT remote=%s target=%s route=%s",
			remoteAddr,
			address,
			route,
		)

		if route == RouteDirect {
			infof(
				"[HTTP] CONNECT using direct route remote=%s target=%s",
				remoteAddr,
				address,
			)

			handleDirect(
				&BufferedConn{
					Conn:   client,
					reader: reader,
				},
				address,
				func() error {
					infof(
						"[HTTP] CONNECT established remote=%s target=%s",
						remoteAddr,
						address,
					)

					_, err := client.Write([]byte(
						"HTTP/1.1 200 Connection Established\r\n\r\n",
					))

					return err
				},
			)

			infof(
				"[HTTP] direct CONNECT finished remote=%s target=%s",
				remoteAddr,
				address,
			)

			return
		}

		infof(
			"[HTTP] CONNECT using broker route remote=%s target=%s route=%s",
			remoteAddr,
			address,
			route,
		)

		handleBroker(
			client,
			reader,
			req.Method,
			address,
			route,
			func() error {
				infof(
					"[HTTP] CONNECT established remote=%s target=%s route=%s",
					remoteAddr,
					address,
					route,
				)

				_, err := client.Write([]byte(
					"HTTP/1.1 200 Connection Established\r\n\r\n",
				))

				if err != nil {
					warnf(
						"[HTTP] failed to send CONNECT response remote=%s target=%s: %v",
						remoteAddr,
						address,
						err,
					)
				}

				return err
			},
		)

		infof(
			"[HTTP] broker CONNECT finished remote=%s target=%s route=%s",
			remoteAddr,
			address,
			route,
		)

	default:
		warnf(
			"[HTTP] unsupported method remote=%s method=%s host=%s",
			remoteAddr,
			req.Method,
			req.Host,
		)

		writeProxyError(client, http.StatusNotImplemented)
		return
	}
}
