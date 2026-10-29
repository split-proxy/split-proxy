package main

import (
	"io"
	"net"
	"time"
)

func handleDirect(
	client net.Conn,
	address string,
	username string,
	onConnected func() error,
) {
	remoteAddr := client.RemoteAddr().String()

	infof(
		"[DIRECT] connecting remote=%s target=%s",
		remoteAddr,
		address,
	)

	dialStarted := time.Now()
	server, resolvedIP, err := dialDirectTarget(address)
	targetConnectDuration.WithLabelValues("direct").Observe(time.Since(dialStarted).Seconds())

	if err != nil {
		connectionErrors.
			WithLabelValues("target_connect", "direct").
			Inc()
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
		_, err := io.Copy(
			&metricWriteConn{
				Conn:      server,
				username:  username,
				direction: "client_to_target",
				path:      "direct",
			},
			client,
		)
		if err != nil {
			connectionErrors.
				WithLabelValues("transfer_read", "direct").
				Inc()
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
		_, err := io.Copy(
			&metricWriteConn{
				Conn:      client,
				username:  username,
				direction: "target_to_client",
				path:      "direct",
			},
			server,
		)
		if err != nil {
			connectionErrors.
				WithLabelValues("transfer_read", "direct").
				Inc()
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
