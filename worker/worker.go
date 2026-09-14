package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

const (
	MaxPacketData      = 32 * 1024
	SessionWriteQueue  = 128
	BrokerWriteQueue   = 8192
	BrokerReadTimeout  = 60 * time.Second
	BrokerWriteTimeout = 10 * time.Second
	TargetDialTimeout  = 10 * time.Second
	TargetWriteTimeout = 30 * time.Second

	// Reconnect backoff.
	BrokerReconnectInitial = 500 * time.Millisecond
	BrokerReconnectMax     = 30 * time.Second
	BrokerReconnectJitter  = 0.25
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

type Session struct {
	id   string
	mode string

	connMu sync.RWMutex
	conn   net.Conn

	writeCh chan []byte
	done    chan struct{}

	closeOnce sync.Once
}

func newSession(id, mode string) *Session {
	return &Session{
		id:      id,
		mode:    mode,
		writeCh: make(chan []byte, SessionWriteQueue),
		done:    make(chan struct{}),
	}
}

func (s *Session) setConn(conn net.Conn) bool {
	if conn == nil {
		return false
	}

	s.connMu.Lock()
	defer s.connMu.Unlock()

	select {
	case <-s.done:
		return false
	default:
	}

	s.conn = conn
	return true
}

func (s *Session) getConn() net.Conn {
	s.connMu.RLock()
	defer s.connMu.RUnlock()

	return s.conn
}

func (s *Session) close() {
	s.closeOnce.Do(func() {
		close(s.done)

		s.connMu.Lock()
		conn := s.conn
		s.conn = nil
		s.connMu.Unlock()

		if conn != nil {
			_ = conn.Close()
		}
	})
}

type WorkerHello struct {
	WorkerGUID string `json:"workerGUID"`
	Token      string `json:"token"`
}

type brokerWriterMessage struct {
	messageType int
	data        []byte
}

var brokerAddr string

var (
	sessions sync.Map // map[string]*Session
)

const (
	CmdData       byte = 0
	CmdConnect    byte = 1
	CmdConnected  byte = 2
	CmdClose      byte = 3
	CmdError      byte = 4
	CmdUDPConnect byte = 5
	CmdUDPData    byte = 6
)

func main() {
	initLogger()

	workerGUID := os.Getenv("WORKER_GUID")
	if workerGUID == "" {
		log.Fatal("[WORKER] WORKER_GUID is empty")
	}

	brokerBase := os.Getenv("BROKER_ADDR")
	if brokerBase == "" {
		log.Fatal("[WORKER] BROKER_ADDR is empty")
	}

	secret := os.Getenv("RANDOM_ENDPOINT_SECRET")
	if secret == "" {
		log.Fatal("[WORKER] RANDOM_ENDPOINT_SECRET is empty")
	}

	brokerAddr = strings.TrimRight(brokerBase, "/") +
		"/ws/worker/" +
		secret

	infof(
		"[WORKER] starting broker=%s workerGUID=%s",
		brokerAddr,
		workerGUID,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	backoff := BrokerReconnectInitial

	for {
		if ctx.Err() != nil {
			break
		}

		err := runBrokerConnection(ctx, workerGUID)

		if ctx.Err() != nil {
			break
		}

		if err != nil {
			warnf(
				"[WORKER] broker connection ended: %v",
				err,
			)
		}

		delay := reconnectDelay(backoff)

		infof(
			"[WORKER] reconnecting to broker in %s",
			delay,
		)

		if !sleepWithContext(ctx, delay) {
			break
		}

		backoff = nextReconnectBackoff(backoff)
	}

	infof("[WORKER] shutdown requested")

	closeAllSessions()

	infof("[WORKER] shutdown complete")
}

func sleepWithContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false

	case <-timer.C:
		return true
	}
}

func closeAllSessions() {
	sessions.Range(func(key, value any) bool {
		session, ok := value.(*Session)
		if ok {
			session.close()
		}

		sessions.Delete(key)
		return true
	})
}

func nextReconnectBackoff(current time.Duration) time.Duration {
	if current >= BrokerReconnectMax {
		return BrokerReconnectMax
	}

	next := current * 2

	if next > BrokerReconnectMax {
		return BrokerReconnectMax
	}

	return next
}

func reconnectDelay(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}

	// Jitter is ±25%.
	//
	// base = 1s
	// result ∈ [750ms, 1250ms]
	//
	// This prevents multiple workers from reconnecting
	// at exactly the same time after a Broker outage.
	jitter := float64(base) * BrokerReconnectJitter

	minDelay := float64(base) - jitter
	maxDelay := float64(base) + jitter

	if minDelay < 0 {
		minDelay = 0
	}

	delay := minDelay + rand.Float64()*(maxDelay-minDelay)

	// BrokerReconnectMax is the absolute upper limit.
	if delay > float64(BrokerReconnectMax) {
		delay = float64(BrokerReconnectMax)
	}

	return time.Duration(delay)
}

func runBrokerConnection(
	ctx context.Context,
	workerGUID string,
) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	infof("[WORKER] connecting to broker")

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   64 * 1024,
		WriteBufferSize:  64 * 1024,
	}

	ws, _, err := dialer.DialContext(
		ctx,
		brokerAddr,
		nil,
	)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fmt.Errorf("broker dial: %w", err)
	}

	hello, err := json.Marshal(
		WorkerHello{
			WorkerGUID: workerGUID,
			Token:      os.Getenv("WORKER_TOKEN"),
		},
	)
	if err != nil {
		_ = ws.Close()
		return fmt.Errorf("marshal worker hello: %w", err)
	}

	if err := ws.WriteMessage(
		websocket.TextMessage,
		hello,
	); err != nil {
		_ = ws.Close()

		if ctx.Err() != nil {
			return ctx.Err()
		}

		return fmt.Errorf("send worker hello: %w", err)
	}

	return handleBrokerConnection(ctx, ws)
}

func handleBrokerConnection(
	ctx context.Context,
	ws *websocket.Conn,
) error {
	defer ws.Close()

	send := make(chan brokerWriterMessage, BrokerWriteQueue)
	done := make(chan struct{})

	var closeOnce sync.Once

	closeDone := func() {
		closeOnce.Do(func() {
			close(done)
		})
	}

	// Shutdown context должен немедленно закрыть WS.
	go func() {
		select {
		case <-ctx.Done():
			closeDone()
			_ = ws.Close()

		case <-done:
		}
	}()

	defer func() {
		closeDone()

		sessions.Range(func(key, value any) bool {
			session, ok := value.(*Session)
			if ok {
				session.close()
			}

			sessions.Delete(key)
			return true
		})
	}()

	_ = ws.SetReadDeadline(
		time.Now().Add(BrokerReadTimeout),
	)

	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(
			time.Now().Add(BrokerReadTimeout),
		)
	})

	go brokerWriter(
		ws,
		send,
		done,
		closeDone,
	)

	for {
		messageType, msg, err := ws.ReadMessage()
		if err != nil {
			closeDone()

			if ctx.Err() != nil {
				return ctx.Err()
			}

			return fmt.Errorf(
				"broker read: %w",
				err,
			)
		}

		if ctx.Err() != nil {
			closeDone()
			return ctx.Err()
		}

		if messageType != websocket.BinaryMessage {
			debugf(
				"[WORKER] ignoring broker message type=%d bytes=%d",
				messageType,
				len(msg),
			)
			continue
		}

		if len(msg) < 37 {
			warnf(
				"[WORKER] invalid broker packet bytes=%d",
				len(msg),
			)
			continue
		}

		id := string(msg[:36])
		cmd := msg[36]
		payload := msg[37:]

		debugf(
			"[WORKER] <- broker session=%s cmd=%d bytes=%d",
			id,
			cmd,
			len(payload),
		)

		switch cmd {

		case CmdConnect:
			if ctx.Err() != nil {
				continue
			}

			go openTCP(
				ctx,
				done,
				send,
				id,
				string(payload),
			)

		case CmdUDPConnect:
			if ctx.Err() != nil {
				continue
			}

			go openUDP(
				ctx,
				done,
				send,
				id,
				string(payload),
			)

		case CmdData:
			enqueueSessionWrite(
				done,
				id,
				payload,
			)

		case CmdUDPData:
			enqueueSessionWrite(
				done,
				id,
				payload,
			)

		case CmdClose:
			closeSession(
				done,
				send,
				id,
			)

		default:
			warnf(
				"[WORKER] unknown broker command id=%s cmd=%d",
				id,
				cmd,
			)
		}
	}
}

func brokerWriter(
	ws *websocket.Conn,
	send <-chan brokerWriterMessage,
	done <-chan struct{},
	closeDone func(),
) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {

		case <-done:
			return

		case msg := <-send:
			if err := ws.SetWriteDeadline(
				time.Now().Add(BrokerWriteTimeout),
			); err != nil {
				closeDone()
				_ = ws.Close()
				return
			}

			if err := ws.WriteMessage(
				msg.messageType,
				msg.data,
			); err != nil {
				warnf(
					"[WORKER] broker write failed: %v",
					err,
				)

				closeDone()
				_ = ws.Close()
				return
			}

		case <-ticker.C:
			if err := ws.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(BrokerWriteTimeout),
			); err != nil {
				warnf(
					"[WORKER] broker ping failed: %v",
					err,
				)

				closeDone()
				_ = ws.Close()
				return
			}
		}
	}
}

func openTCP(
	ctx context.Context,
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
	addr string,
) {
	if ctx.Err() != nil {
		return
	}

	if _, exists := sessions.Load(id); exists {
		return
	}

	session := newSession(id, "tcp")

	actual, loaded := sessions.LoadOrStore(
		id,
		session,
	)

	if loaded {
		_ = actual
		return
	}

	if ctx.Err() != nil {
		session.close()
		sessions.Delete(id)
		return
	}

	debugf(
		"[WORKER] opening TCP session=%s target=%s",
		id,
		addr,
	)

	conn, err := net.DialTimeout(
		"tcp",
		addr,
		TargetDialTimeout,
	)

	if err != nil {
		sessions.Delete(id)

		if ctx.Err() == nil {
			debugf(
				"[WORKER] TCP connect failed session=%s target=%s: %v",
				id,
				addr,
				err,
			)

			sendPacket(
				done,
				send,
				id,
				CmdClose,
				nil,
			)
		}

		return
	}

	if ctx.Err() != nil {
		_ = conn.Close()
		session.close()
		sessions.Delete(id)
		return
	}

	if !session.setConn(conn) {
		_ = conn.Close()
		sessions.Delete(id)
		return
	}

	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetReadBuffer(1024 * 1024)
		_ = tcp.SetWriteBuffer(1024 * 1024)
	}

	go sessionWriter(
		session,
		done,
		id,
	)

	go remoteReader(
		done,
		send,
		id,
		session,
	)

	if !sendPacket(
		done,
		send,
		id,
		CmdConnected,
		nil,
	) {
		session.close()
		sessions.Delete(id)
		return
	}

	infof(
		"[WORKER] TCP connected session=%s target=%s",
		id,
		addr,
	)
}

func openUDP(
	ctx context.Context,
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
	addr string,
) {
	if ctx.Err() != nil {
		return
	}

	if _, exists := sessions.Load(id); exists {
		return
	}

	session := newSession(id, "udp")

	if _, loaded := sessions.LoadOrStore(
		id,
		session,
	); loaded {
		return
	}

	if ctx.Err() != nil {
		session.close()
		sessions.Delete(id)
		return
	}

	debugf(
		"[WORKER] opening UDP session=%s target=%s",
		id,
		addr,
	)

	conn, err := net.DialTimeout(
		"udp",
		addr,
		TargetDialTimeout,
	)

	if err != nil {
		sessions.Delete(id)

		if ctx.Err() == nil {
			debugf(
				"[WORKER] UDP connect failed session=%s target=%s: %v",
				id,
				addr,
				err,
			)

			sendPacket(
				done,
				send,
				id,
				CmdClose,
				nil,
			)
		}

		return
	}

	if ctx.Err() != nil {
		_ = conn.Close()
		session.close()
		sessions.Delete(id)
		return
	}

	if !session.setConn(conn) {
		_ = conn.Close()
		sessions.Delete(id)
		return
	}

	go sessionWriter(
		session,
		done,
		id,
	)

	go remoteUDPReader(
		done,
		send,
		id,
		session,
	)

	if !sendPacket(
		done,
		send,
		id,
		CmdConnected,
		nil,
	) {
		session.close()
		sessions.Delete(id)
		return
	}

	infof(
		"[WORKER] UDP connected session=%s target=%s",
		id,
		addr,
	)
}

func enqueueSessionWrite(
	done <-chan struct{},
	id string,
	data []byte,
) {
	value, ok := sessions.Load(id)
	if !ok {
		debugf(
			"[WORKER] data for unknown session=%s",
			id,
		)
		return
	}

	session := value.(*Session)

	payload := append([]byte(nil), data...)

	select {
	case <-done:
		return

	case <-session.done:
		return

	case session.writeCh <- payload:
		// queued

	default:
		warnf(
			"[WORKER] session write queue full session=%s",
			id,
		)

		closeSession(
			done,
			nil,
			id,
		)
	}
}

func sessionWriter(
	session *Session,
	done <-chan struct{},
	id string,
) {
	for {
		select {

		case <-done:
			return

		case <-session.done:
			return

		case data := <-session.writeCh:
			conn := session.getConn()
			if conn == nil {
				return
			}

			if err := conn.SetWriteDeadline(
				time.Now().Add(TargetWriteTimeout),
			); err != nil {
				closeSession(
					done,
					nil,
					id,
				)
				return
			}

			for len(data) > 0 {
				n, err := conn.Write(data)
				if err != nil {
					warnf(
						"[WORKER] target write failed session=%s: %v",
						id,
						err,
					)

					closeSession(
						done,
						nil,
						id,
					)

					return
				}

				if n <= 0 {
					closeSession(
						done,
						nil,
						id,
					)
					return
				}

				data = data[n:]
			}
		}
	}
}

func remoteReader(
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
	session *Session,
) {
	conn := session.getConn()
	if conn == nil {
		return
	}

	defer func() {
		session.close()
		sessions.Delete(id)

		sendPacket(
			done,
			send,
			id,
			CmdClose,
			nil,
		)
	}()

	buf := make([]byte, 64*1024)

	for {
		n, err := conn.Read(buf)

		if n > 0 {
			data := buf[:n]

			for len(data) > 0 {
				size := len(data)

				if size > MaxPacketData {
					size = MaxPacketData
				}

				if !sendPacket(
					done,
					send,
					id,
					CmdData,
					data[:size],
				) {
					return
				}

				data = data[size:]
			}
		}

		if err != nil {
			return
		}

		select {
		case <-done:
			return
		case <-session.done:
			return
		default:
		}
	}
}

func remoteUDPReader(
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
	session *Session,
) {
	conn := session.getConn()
	if conn == nil {
		return
	}

	defer func() {
		session.close()
		sessions.Delete(id)

		sendPacket(
			done,
			send,
			id,
			CmdClose,
			nil,
		)
	}()

	buf := make([]byte, 64*1024)

	for {
		n, err := conn.Read(buf)

		if n > 0 {
			if !sendPacket(
				done,
				send,
				id,
				CmdUDPData,
				buf[:n],
			) {
				return
			}
		}

		if err != nil {
			return
		}

		select {
		case <-done:
			return
		case <-session.done:
			return
		default:
		}
	}
}

func closeSession(
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
) {
	value, ok := sessions.Load(id)
	if !ok {
		return
	}

	session := value.(*Session)

	session.close()
	sessions.Delete(id)

	if send != nil {
		sendPacket(
			done,
			send,
			id,
			CmdClose,
			nil,
		)
	}

	debugf(
		"[WORKER] session closed id=%s",
		id,
	)
}

func sendPacket(
	done <-chan struct{},
	send chan<- brokerWriterMessage,
	id string,
	cmd byte,
	data []byte,
) bool {
	if len(id) != 36 {
		warnf(
			"[WORKER] invalid session id length=%d",
			len(id),
		)
		return false
	}

	buf := make([]byte, 37+len(data))

	copy(buf[:36], id)
	buf[36] = cmd

	if len(data) > 0 {
		copy(buf[37:], data)
	}

	msg := brokerWriterMessage{
		messageType: websocket.BinaryMessage,
		data:        buf,
	}

	select {
	case <-done:
		return false

	case send <- msg:
		return true
	}
}
