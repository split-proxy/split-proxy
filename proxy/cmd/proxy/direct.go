package main

import (
	"io"
	"net"
)

func handleDirect(
	client net.Conn,
	address string,
	onConnected func() error,
) {
	remoteAddr := client.RemoteAddr().String()

	infof(
		"[DIRECT] connecting remote=%s target=%s",
		remoteAddr,
		address,
	)

	server, resolvedIP, err := dialDirectTarget(address)
	if err != nil {
		warnf(
			"[DIRECT] connection failed remote=%s target=%s: %v",
			remoteAddr,
			address,
			err,
		)
		return
	}
	defer server.Close()

	infof(
		"[DIRECT] connected remote=%s target=%s ip=%s",
		remoteAddr,
		address,
		resolvedIP,
	)

	if onConnected != nil {
		if err := onConnected(); err != nil {
			warnf(
				"[DIRECT] failed to send connection response remote=%s target=%s: %v",
				remoteAddr,
				address,
				err,
			)
			return
		}
	}

	infof(
		"[DIRECT] starting tunnel remote=%s target=%s ip=%s",
		remoteAddr,
		address,
		resolvedIP,
	)

	done := make(chan struct{}, 2)

	go func() {
		_, err := io.Copy(server, client)
		if err != nil {
			debugf(
				"[DIRECT] client->server copy finished remote=%s target=%s: %v",
				remoteAddr,
				address,
				err,
			)
		}

		done <- struct{}{}
	}()

	go func() {
		_, err := io.Copy(client, server)
		if err != nil {
			debugf(
				"[DIRECT] server->client copy finished remote=%s target=%s: %v",
				remoteAddr,
				address,
				err,
			)
		}

		done <- struct{}{}
	}()

	<-done

	infof(
		"[DIRECT] tunnel finished remote=%s target=%s ip=%s",
		remoteAddr,
		address,
		resolvedIP,
	)

	_ = client.Close()
	_ = server.Close()
}
