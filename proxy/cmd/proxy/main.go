package main

import (
	"net"
	"time"
)

func main() {
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
				warnf("[PROXY] failed to reload routes: %v", err)
			} else {
				debugf("[PROXY] routes reloaded")
			}
		}
	}()

	if databaseURL == "" {
		errorf("[PROXY] DATABASE_URL is not set")
		return
	}

	infof("[PROXY] initializing auth store")

	authStore, err := NewAuthStore(ctx, databaseURL)
	if err != nil {
		errorf("[PROXY] failed to initialize auth store: %v", err)
		return
	}
	defer authStore.Close()

	infof("[PROXY] starting auth sync")
	authStore.StartSync(ctx)

	infof("[PROXY] proxy listening addr=%s", listenAddr)

	for {
		c, err := ln.Accept()
		if err != nil {
			debugf("[PROXY] accept failed: %v", err)
			continue
		}

		remoteAddr := c.RemoteAddr().String()

		select {
		case connSem <- struct{}{}:
			debugf(
				"[PROXY] connection accepted remote=%s",
				remoteAddr,
			)

			go func() {
				defer func() {
					<-connSem
				}()

				handleConn(c, authStore)
			}()

		default:
			warnf(
				"[PROXY] connection limit reached, rejecting remote=%s",
				remoteAddr,
			)

			_ = c.Close()
		}
	}
}
