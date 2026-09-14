package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type BrokerRequest struct {
	ID     string `json:"id"`
	Route  string `json:"route"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
}

type ProxyHello struct {
	Token string `json:"token"`
}

type brokerOutbound struct {
	messageType int
	data        []byte
}

type BrokerSession struct {
	id   string
	conn *BrokerConnection

	data      chan []byte
	connected chan error
	done      chan struct{}

	closeOnce sync.Once
}

func (s *BrokerSession) close(err error) {
	s.closeOnce.Do(func() {
		if err != nil {
			select {
			case s.connected <- err:
			default:
			}
		}

		close(s.done)
	})
}

func (s *BrokerSession) waitConnected() error {
	select {
	case err := <-s.connected:
		return err
	case <-s.done:
		select {
		case err := <-s.connected:
			return err
		default:
			return fmt.Errorf("broker session closed")
		}
	}
}

func (s *BrokerSession) send(cmd byte, payload []byte) error {
	if s == nil || s.conn == nil {
		return fmt.Errorf("broker session unavailable")
	}

	buf := make([]byte, 37+len(payload))
	copy(buf[:36], s.id)
	buf[36] = cmd
	copy(buf[37:], payload)

	return s.conn.send(websocket.BinaryMessage, buf)
}

func (s *BrokerSession) isClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

type BrokerConnection struct {
	ws *websocket.Conn

	writeCh chan brokerOutbound
	done    chan struct{}

	sessions  sync.Map // map[string]*BrokerSession
	closeOnce sync.Once
}

func (c *BrokerConnection) send(messageType int, data []byte) error {
	if c == nil {
		return fmt.Errorf("broker connection unavailable")
	}

	msg := brokerOutbound{
		messageType: messageType,
		data:        append([]byte(nil), data...),
	}

	select {
	case <-c.done:
		return fmt.Errorf("broker connection closed")
	case c.writeCh <- msg:
		return nil
	}
}

func (c *BrokerConnection) close(err error) {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.ws.Close()

		c.sessions.Range(func(key, value any) bool {
			s := value.(*BrokerSession)
			c.sessions.Delete(key)
			s.close(err)
			return true
		})

		brokerStateMu.Lock()
		if brokerConn == c {
			brokerConn = nil
		}
		brokerStateMu.Unlock()
	})
}

var (
	brokerStateMu sync.Mutex
	brokerConn    *BrokerConnection
	brokerDialing bool
	brokerWaitCh  chan struct{}
)

func getBrokerConnection() (*BrokerConnection, error) {
	for {
		brokerStateMu.Lock()

		if brokerConn != nil {
			c := brokerConn
			brokerStateMu.Unlock()
			return c, nil
		}

		if brokerDialing {
			waitCh := brokerWaitCh
			brokerStateMu.Unlock()
			<-waitCh
			continue
		}

		brokerDialing = true
		brokerWaitCh = make(chan struct{})
		waitCh := brokerWaitCh
		brokerStateMu.Unlock()

		c, err := dialBroker()

		brokerStateMu.Lock()
		if err == nil {
			brokerConn = c
		}
		brokerDialing = false
		close(waitCh)
		brokerStateMu.Unlock()

		return c, err
	}
}

func dialBroker() (*BrokerConnection, error) {
	infof("[BROKER] establishing persistent websocket")

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   64 * 1024,
		WriteBufferSize:  64 * 1024,
	}

	ws, _, err := dialer.Dial(brokerWS, nil)
	if err != nil {
		return nil, err
	}

	helloData, err := json.Marshal(ProxyHello{Token: proxyToken})
	if err != nil {
		_ = ws.Close()
		return nil, err
	}

	if err := ws.WriteMessage(websocket.TextMessage, helloData); err != nil {
		_ = ws.Close()
		return nil, err
	}

	c := &BrokerConnection{
		ws:      ws,
		writeCh: make(chan brokerOutbound, 4096),
		done:    make(chan struct{}),
	}
	ws.SetReadLimit(128 * 1024)
	_ = ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	ws.SetPongHandler(func(string) error { return ws.SetReadDeadline(time.Now().Add(60 * time.Second)) })

	infof("[BROKER] persistent websocket connected")

	go c.writeLoop()
	go c.readLoop()

	return c, nil
}

func (c *BrokerConnection) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.writeCh:
			if err := c.ws.WriteMessage(msg.messageType, msg.data); err != nil {
				warnf("[BROKER] websocket write failed: %v", err)
				c.close(err)
				return
			}
		case <-ticker.C:
			if err := c.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
				c.close(err)
				return
			}
		}
	}
}

func (c *BrokerConnection) readLoop() {
	for {
		messageType, msg, err := c.ws.ReadMessage()
		if err != nil {
			warnf("[BROKER] persistent websocket closed: %v", err)
			c.close(err)
			return
		}

		if messageType != websocket.BinaryMessage {
			debugf("[BROKER] ignoring websocket message type=%d bytes=%d", messageType, len(msg))
			continue
		}

		if len(msg) < 37 {
			warnf("[BROKER] invalid broker packet bytes=%d", len(msg))
			continue
		}

		id := string(msg[:36])
		cmd := msg[36]

		value, ok := c.sessions.Load(id)
		if !ok {
			debugf("[BROKER] packet for unknown session id=%s cmd=%d", id, cmd)
			continue
		}

		s := value.(*BrokerSession)

		switch cmd {
		case CmdConnected:
			select {
			case s.connected <- nil:
			default:
			}

		case CmdData, CmdUDPData:
			payload := append([]byte(nil), msg[37:]...)
			select {
			case s.data <- payload:
			case <-s.done:
			default:
				warnf("[BROKER] session data queue full id=%s cmd=%d", id, cmd)
				c.sessions.Delete(id)
				s.close(fmt.Errorf("broker session data queue full"))
			}

		case CmdClose:
			c.sessions.Delete(id)
			s.close(fmt.Errorf("remote broker session closed"))

		case CmdError:
			c.sessions.Delete(id)
			s.close(fmt.Errorf("remote broker session error: %s", string(msg[37:])))

		default:
			warnf("[BROKER] unknown command=%d session=%s", cmd, id)
		}
	}
}

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32]), nil
}

func (c *BrokerConnection) openSession(route Route, target, mode string) (*BrokerSession, error) {
	if mode == "" {
		mode = "tcp"
	}

	if mode != "tcp" && mode != "udp" {
		return nil, fmt.Errorf("invalid broker session mode=%q", mode)
	}

	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	s := &BrokerSession{
		id:        id,
		conn:      c,
		data:      make(chan []byte, 32),
		connected: make(chan error, 1),
		done:      make(chan struct{}),
	}

	c.sessions.Store(id, s)

	reqData, err := json.Marshal(BrokerRequest{
		ID:     id,
		Route:  string(route),
		Target: target,
		Mode:   mode,
	})
	if err != nil {
		c.sessions.Delete(id)
		s.close(err)
		return nil, err
	}

	if err := c.send(websocket.TextMessage, reqData); err != nil {
		c.sessions.Delete(id)
		s.close(err)
		return nil, err
	}

	debugf("[BROKER] session opened id=%s route=%s target=%s mode=%s", id, route, target, mode)
	return s, nil
}

func closeBrokerSession(s *BrokerSession) {
	if s == nil || s.conn == nil {
		return
	}

	if s.isClosed() {
		return
	}

	_ = s.send(CmdClose, nil)
	s.conn.sessions.Delete(s.id)
	s.close(nil)
}

func handleBroker(
	client net.Conn,
	reader *bufio.Reader,
	method,
	host string,
	route Route,
	onConnected func() error,
) {
	remoteAddr := client.RemoteAddr().String()

	infof(
		"[BROKER] opening multiplexed session remote=%s method=%s target=%s route=%s",
		remoteAddr,
		method,
		host,
		route,
	)

	c, err := getBrokerConnection()
	if err != nil {
		warnf(
			"[BROKER] connection failed remote=%s target=%s route=%s: %v",
			remoteAddr,
			host,
			route,
			err,
		)
		return
	}

	s, err := c.openSession(route, host, "tcp")
	if err != nil {
		warnf("[BROKER] session open failed remote=%s target=%s: %v", remoteAddr, host, err)
		return
	}

	defer closeBrokerSession(s)

	if err := s.waitConnected(); err != nil {
		warnf(
			"[BROKER] target connection failed remote=%s target=%s route=%s: %v",
			remoteAddr,
			host,
			route,
			err,
		)
		return
	}

	infof(
		"[BROKER] connection established remote=%s target=%s route=%s session=%s",
		remoteAddr,
		host,
		route,
		s.id,
	)

	if onConnected != nil {
		if err := onConnected(); err != nil {
			warnf(
				"[BROKER] connection response failed remote=%s target=%s: %v",
				remoteAddr,
				host,
				err,
			)
			return
		}
	}

	infof(
		"[BROKER] starting multiplexed client tunnel remote=%s target=%s session=%s",
		remoteAddr,
		host,
		s.id,
	)

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)

		buf := make([]byte, 32*1024)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				if sendErr := s.send(CmdData, buf[:n]); sendErr != nil {
					warnf(
						"[BROKER] client->broker write failed remote=%s target=%s session=%s: %v",
						remoteAddr,
						host,
						s.id,
						sendErr,
					)
					return
				}
			}

			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-clientDone:
			return

		case <-s.done:
			return

		case msg, ok := <-s.data:
			if !ok {
				return
			}

			if _, err := client.Write(msg); err != nil {
				warnf(
					"[BROKER] broker->client write failed remote=%s target=%s session=%s: %v",
					remoteAddr,
					host,
					s.id,
					err,
				)
				return
			}
		}
	}
}

func handleUDPBroker(route Route, target string) (*BrokerSession, error) {
	infof(
		"[BROKER] opening multiplexed UDP session target=%s route=%s",
		target,
		route,
	)

	c, err := getBrokerConnection()
	if err != nil {
		return nil, err
	}

	s, err := c.openSession(route, target, "udp")
	if err != nil {
		return nil, err
	}

	if err := s.waitConnected(); err != nil {
		closeBrokerSession(s)
		return nil, err
	}

	infof(
		"[BROKER] UDP session established target=%s route=%s session=%s",
		target,
		route,
		s.id,
	)

	return s, nil
}
