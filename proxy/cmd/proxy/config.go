package main

import (
	"crypto/tls"
	"context"
	"net"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	tlsCertFile = "/app/certs/server.crt"
	tlsKeyFile  = "/app/certs/server.key"

	ProtocolDetectionTimeout = 10 * time.Second

	UDPRelayMinPort = 55000
	UDPRelayMaxPort = 60000

	AuthSyncInterval = 10 * time.Second
	AuthReloadTimeout = 5 * time.Second
)

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

var logLevel = LogLevelInfo

func initLogger() {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		logLevel = LogLevelDebug

	case "warn", "warning":
		logLevel = LogLevelWarn

	case "error":
		logLevel = LogLevelError

	default:
		logLevel = LogLevelWarn
	}
}

func debugf(format string, args ...any) {
	if logLevel >= LogLevelDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func infof(format string, args ...any) {
	if logLevel >= LogLevelInfo {
		log.Printf("[INFO] "+format, args...)
	}
}

func warnf(format string, args ...any) {
	if logLevel >= LogLevelWarn {
		log.Printf("[WARN] "+format, args...)
	}
}

func errorf(format string, args ...any) {
	if logLevel >= LogLevelError {
		log.Printf("[ERROR] "+format, args...)
	}
}

type Route string

var (
	brokerWS = ""

	rdb *redis.Client

	ctx = context.Background()

	proxyToken = os.Getenv("PROXY_TOKEN")

	listenAddr = net.JoinHostPort(
    os.Getenv("PROXY_LISTEN_IP"),
    os.Getenv("PROXY_LISTEN_PORT"),
	)

	maxConnections = getEnvInt("MAX_CONNECTIONS", 1000)
	connSem = make(chan struct{}, maxConnections)

	UDPRelayHost = os.Getenv("UDP_RELAY_HOST")

	dbHost = os.Getenv("POSTGRES_HOST")
	dbPort = os.Getenv("POSTGRES_PORT")
	dbName = os.Getenv("POSTGRES_DB")
	dbUser = os.Getenv("POSTGRES_USER")
	dbPassword = os.Getenv("POSTGRES_PASSWORD")

	RouteDirect Route = Route(os.Getenv("DEFAULT_GROUP_NAME"))

	DomainsRedisKey      = os.Getenv("DOMAINS_REDIS_KEY")
	CidrsRedisKey        = os.Getenv("CIDRS_REDIS_KEY")
	DefaultRouteRedisKey = os.Getenv("DEFAULT_ROUTE_REDIS_KEY")

	tlsConfig *tls.Config
)

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		log.Printf("[WARN] invalid %s=%q, using default=%d", key, value, defaultValue)
		return defaultValue
	}

	return n
}

func init() {
	initLogger()

	if RouteDirect == "" {
		log.Fatal("DEFAULT_GROUP_NAME environment variable is required")
	}

	brokerWS = "ws://" + os.Getenv("BROKER_CONNECT_HOST") + ":" +  os.Getenv("BROKER_PORT") +
		"/ws/proxy/" +
		os.Getenv("RANDOM_ENDPOINT_SECRET")

	db := 0
	if value := os.Getenv("REDIS_DB"); value != "" {
			var err error
			db, err = strconv.Atoi(value)
			if err != nil {
					errorf("invalid REDIS_DB: %v", err)
					os.Exit(1)
			}
	}

	opts := &redis.Options{
			Addr:     net.JoinHostPort(os.Getenv("REDIS_HOST"), os.Getenv("REDIS_PORT")),
			Password: os.Getenv("REDIS_PASSWORD"),
			DB:       db,
	}

	rdb = redis.NewClient(opts)

	if err := loadRoutes(); err != nil {
		errorf("failed to load routes: %v", err)
		os.Exit(1)
	}

	if err := loadTLSConfig(); err != nil {
		errorf("failed to load TLS config: %v", err)
		os.Exit(1)
	}
}

func loadTLSConfig() error {
	certFile := os.Getenv("TLS_CERT_FILE")
	if certFile == "" {
		certFile = tlsCertFile
	}

	keyFile := os.Getenv("TLS_KEY_FILE")
	if keyFile == "" {
		keyFile = tlsKeyFile
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	return nil
}
