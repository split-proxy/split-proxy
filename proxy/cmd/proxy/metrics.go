package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	acceptedConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxy_connections_accepted_total", Help: "Total number of TCP connections successfully accepted.",
	})
	rejectedConnections = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "proxy_connections_rejected_total", Help: "Total connections rejected due to the connection limit.",
	})
	activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxy_active_connections", Help: "Currently active client TCP connection handlers.",
	})
	activeUDPSessions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxy_active_udp_sessions", Help: "Currently active SOCKS5 UDP ASSOCIATE sessions.",
	})
	authFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_auth_failures_total", Help: "Failed authentication attempts.",
	}, []string{"protocol"})
	connectionErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_connection_errors_total", Help: "Connection and data transfer errors.",
	}, []string{"stage", "path"})
	socketOpenErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_socket_open_errors_total", Help: "Socket operation errors.",
	}, []string{"operation"})
	syncErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_sync_errors_total", Help: "Failed synchronization operations.",
	}, []string{"kind"})
	targetConnectDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "proxy_target_connect_duration_seconds", Help: "Time required to establish a target connection.",
	}, []string{"path"})
	dependencyUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "proxy_dependency_up", Help: "Whether a dependency is reachable: 1=up, 0=down.",
	}, []string{"component"})
	dependencyOperationDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "proxy_dependency_operation_duration_seconds", Help: "Duration of dependency operations in seconds.",
	}, []string{"component", "operation"})
	lastRoutesSyncSuccessTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxy_last_successful_routes_sync_timestamp_seconds", Help: "Unix timestamp of the last successful routing configuration load.",
	})
	lastUsersSyncSuccessTimestamp = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "proxy_last_successful_user_sync_timestamp_seconds", Help: "Unix timestamp of the last successful user synchronization.",
	})
	trafficBytes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "proxy_traffic_bytes_total", Help: "Payload bytes transferred through the proxy, grouped by authenticated user.",
	}, []string{"username", "direction", "path"})
	userActiveConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "proxy_user_active_connections", Help: "Active authenticated connections grouped by username and client IP.",
	}, []string{"username", "client_ip"})
)

func initMetrics() {
	prometheus.MustRegister(
		acceptedConnections, rejectedConnections, activeConnections,
		activeUDPSessions, authFailures, connectionErrors, socketOpenErrors,
		syncErrors, targetConnectDuration, dependencyUp,
		dependencyOperationDuration, lastRoutesSyncSuccessTimestamp,
		lastUsersSyncSuccessTimestamp, trafficBytes, userActiveConnections,
	)
}

func startMetrics() {
	addr := os.Getenv("METRICS_LISTEN_IP") + ":" + os.Getenv("METRICS_LISTEN_PORT")
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(addr, mux); err != nil {
			errorf("[METRICS] HTTP server failed addr=%s: %v", addr, err)
		}
	}()
}

func clientIP(conn net.Conn) string {
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return conn.RemoteAddr().String()
	}
	return host
}

func trackAuthenticatedUser(username string, conn net.Conn) func() {
	ip := clientIP(conn)
	userActiveConnections.WithLabelValues(username, ip).Inc()
	return func() {
		userActiveConnections.WithLabelValues(username, ip).Dec()
	}
}

func recordTraffic(username, direction, path string, n int) {
	if n <= 0 {
		return
	}
	if username == "" {
		username = "unknown"
	}
	trafficBytes.WithLabelValues(username, direction, path).Add(float64(n))
}

type metricWriteConn struct {
	net.Conn
	username  string
	direction string
	path      string
}

func (c *metricWriteConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	recordTraffic(c.username, c.direction, c.path, n)
	return n, err
}

func startDependencyMetrics(ctx context.Context, authStore *AuthStore) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	check := func(component, operation string, fn func() error) {
		started := time.Now()
		err := fn()
		dependencyOperationDuration.WithLabelValues(component, operation).Observe(time.Since(started).Seconds())
		if err != nil {
			dependencyUp.WithLabelValues(component).Set(0)
		} else {
			dependencyUp.WithLabelValues(component).Set(1)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check("postgres", "ping", func() error {
				pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				return authStore.pool.Ping(pingCtx)
			})
			check("redis", "ping", func() error {
				pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				return rdb.Ping(pingCtx).Err()
			})
		}
	}
}

func metricsEnabled() bool {
	return os.Getenv("METRICS_ENABLED") != "false"
}
