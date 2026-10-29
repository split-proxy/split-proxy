package main

import (
	"net"
	"net/url"
	"time"
)

func main() {
	if metricsEnabled() {
		initMetrics()
		startMetrics()
		infof("[METRICS] enabled")
	} else {
		infof("[METRICS] disabled")
	}
	ln, err := net.Listen("tcp4", listenAddr)
	if err != nil {
		errorf("[PROXY] failed to listen addr=%s: %v", listenAddr, err)
		return
	}
	defer ln.Close()

	infof("[PROXY] listener started addr=%s", listenAddr)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if err := loadRoutes(); err != nil {
				syncErrors.WithLabelValues("routes").Inc()
				warnf("[PROXY] failed to reload routes: %v", err)
			} else {
				debugf("[PROXY] routes reloaded")
			}
		}
	}()

	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(dbHost, dbPort),
		Path:   dbName,
	}
	u.User = url.UserPassword(dbUser, dbPassword)

	authStore, err := NewAuthStore(ctx, u.String())
	if err != nil {
		errorf("[PROXY] failed to initialize auth store: %v", err)
		return
	}
	defer authStore.Close()
	go startDependencyMetrics(ctx, authStore)

	infof("[PROXY] starting auth sync")
	authStore.StartSync(ctx)

	infof("[PROXY] proxy listening addr=%s", listenAddr)

	for {
		c, err := ln.Accept()
		if err != nil {
			socketOpenErrors.WithLabelValues("accept").Inc()
			debugf("[PROXY] accept failed: %v", err)
			continue
		}

		acceptedConnections.Inc()

		remoteAddr := c.RemoteAddr().String()

		select {
		case connSem <- struct{}{}:
			activeConnections.Inc()
			debugf(
				"[PROXY] connection accepted remote=%s",
				remoteAddr,
			)

			go func() {
				defer func() {
					activeConnections.Dec()
					<-connSem
				}()

				handleConn(c, authStore)
			}()

		default:
			rejectedConnections.Inc()
			warnf(
				"[PROXY] connection limit reached, rejecting remote=%s",
				remoteAddr,
			)

			_ = c.Close()
		}
	}
}
