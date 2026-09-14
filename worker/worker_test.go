package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testSessionID = "12345678-1234-4234-8234-123456789012"

func clearSessions() {
	sessions.Range(func(key, value any) bool {
		if session, ok := value.(*Session); ok {
			session.close()
		}

		sessions.Delete(key)
		return true
	})
}

func TestRemoteReader_PayloadLargerThan32KB_IsFragmented(t *testing.T) {
	clearSessions()
	t.Cleanup(clearSessions)

	done := make(chan struct{})
	send := make(chan brokerWriterMessage, 4)

	workerConn, targetConn := net.Pipe()
	t.Cleanup(func() {
		_ = workerConn.Close()
		_ = targetConn.Close()
	})

	session := newSession(testSessionID, "tcp")

	if !session.setConn(workerConn) {
		t.Fatal("failed to set session connection")
	}

	sessions.Store(testSessionID, session)

	go remoteReader(
		done,
		send,
		testSessionID,
		session,
	)

	payload := make([]byte, MaxPacketData+1)

	for i := range payload {
		payload[i] = byte(i % 251)
	}

	writeDone := make(chan error, 1)

	go func() {
		_, err := targetConn.Write(payload)
		writeDone <- err
	}()

	var received []byte

	for i := 0; i < 2; i++ {
		select {
		case msg := <-send:
			if len(msg.data) < 37 {
				t.Fatalf("packet is too short: %d bytes", len(msg.data))
			}

			if string(msg.data[:36]) != testSessionID {
				t.Fatalf("unexpected session ID: %q", string(msg.data[:36]))
			}

			if msg.data[36] != CmdData {
				t.Fatalf(
					"unexpected command: got %d, want CmdData (%d)",
					msg.data[36],
					CmdData,
				)
			}

			received = append(received, msg.data[37:]...)

		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for fragmented payload")
		}
	}

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("target write failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("target write did not complete")
	}

	if len(received) != len(payload) {
		t.Fatalf(
			"payload length mismatch: got %d, want %d",
			len(received),
			len(payload),
		)
	}

	for i := range payload {
		if received[i] != payload[i] {
			t.Fatalf(
				"payload mismatch at byte %d: got %d, want %d",
				i,
				received[i],
				payload[i],
			)
		}
	}

	select {
	case msg := <-send:
		t.Fatalf(
			"unexpected additional packet: cmd=%d payload=%d bytes",
			msg.data[36],
			len(msg.data)-37,
		)
	default:
	}
}

func TestEnqueueSessionWrite_QueueOverflow_ClosesSession(t *testing.T) {
	clearSessions()
	t.Cleanup(clearSessions)

	const sessionID = testSessionID

	session := newSession(sessionID, "tcp")
	sessions.Store(sessionID, session)

	done := make(chan struct{})

	for i := 0; i < SessionWriteQueue; i++ {
		enqueueSessionWrite(
			done,
			sessionID,
			[]byte("payload"),
		)
	}

	if len(session.writeCh) != SessionWriteQueue {
		t.Fatalf(
			"queue was not filled: got %d, want %d",
			len(session.writeCh),
			SessionWriteQueue,
		)
	}

	enqueueSessionWrite(
		done,
		sessionID,
		[]byte("overflow"),
	)

	select {
	case <-session.done:
	case <-time.After(time.Second):
		t.Fatal("session was not closed after queue overflow")
	}

	if _, ok := sessions.Load(sessionID); ok {
		t.Fatalf(
			"session %q is still present after queue overflow",
			sessionID,
		)
	}

}

func TestHandleBrokerConnection_Disconnect_ClosesAllSessions(t *testing.T) {
	clearSessions()
	t.Cleanup(clearSessions)

	workerConn1, brokerConn1 := net.Pipe()
	workerConn2, brokerConn2 := net.Pipe()

	session1 := newSession("12345678-1234-4234-8234-123456789012", "tcp")
	session2 := newSession("22345678-1234-4234-8234-123456789012", "tcp")

	if !session1.setConn(workerConn1) {
		t.Fatal("failed to set session1 connection")
	}

	if !session2.setConn(workerConn2) {
		t.Fatal("failed to set session2 connection")
	}

	sessions.Store(session1.id, session1)
	sessions.Store(session2.id, session2)

	t.Cleanup(func() {
		_ = brokerConn1.Close()
		_ = brokerConn2.Close()
		clearSessions()
	})

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	serverReady := make(chan struct{})
	closeServer := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ws, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}

			close(serverReady)

			<-closeServer

			_ = ws.Close()
		},
	))

	t.Cleanup(server.Close)

	wsURL := "ws" + server.URL[len("http"):]

	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial test broker: %v", err)
	}
	t.Cleanup(func() {
		_ = ws.Close()
	})

	handlerDone := make(chan error, 1)

	go func() {
		handlerDone <- handleBrokerConnection(
			context.Background(),
			ws,
		)
	}()

	select {
	case <-serverReady:
	case <-time.After(2 * time.Second):
		t.Fatal("test broker did not accept worker connection")
	}

	close(closeServer)

	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handleBrokerConnection did not finish after broker disconnect")
	}

	select {
	case <-session1.done:
	case <-time.After(time.Second):
		t.Fatal("session1 was not closed")
	}

	if _, ok := sessions.Load(session1.id); ok {
		t.Fatalf("session1 %q still exists", session1.id)
	}

	select {
	case <-session2.done:
	case <-time.After(time.Second):
		t.Fatal("session2 was not closed")
	}

	if _, ok := sessions.Load(session2.id); ok {
		t.Fatalf("session2 %q still exists", session2.id)
	}

	for name, conn := range map[string]net.Conn{
		"session1": brokerConn1,
		"session2": brokerConn2,
	} {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))

		var buf [1]byte
		_, err := conn.Read(buf[:])

		if err == nil {
			t.Fatalf("%s connection is still open", name)
		}
	}
}

func TestRunBrokerConnection_AfterDisconnect_CanReconnect(t *testing.T) {
	clearSessions()
	t.Cleanup(clearSessions)

	var connectionCount atomic.Int32

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	secondConnection := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ws, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}

			n := connectionCount.Add(1)

			if n == 1 {
				_, _, _ = ws.ReadMessage()
				_ = ws.Close()
				return
			}

			secondConnection <- ws
		},
	))

	t.Cleanup(server.Close)

	brokerAddr = "ws" + server.URL[len("http"):]

	ctx := context.Background()

	err := runBrokerConnection(ctx, "test-worker")
	if err == nil {
		t.Fatal("expected first broker connection to end with an error")
	}

	if connectionCount.Load() != 1 {
		t.Fatalf(
			"unexpected connection count after first attempt: got %d, want 1",
			connectionCount.Load(),
		)
	}

	secondCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	secondDone := make(chan error, 1)

	go func() {
		secondDone <- runBrokerConnection(
			secondCtx,
			"test-worker",
		)
	}()

	select {
	case ws := <-secondConnection:
		if ws == nil {
			t.Fatal("second broker connection is nil")
		}

		cancel()

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not reconnect to broker")
	}

	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second broker connection did not terminate after cancellation")
	}

	if connectionCount.Load() != 2 {
		t.Fatalf(
			"unexpected connection count: got %d, want 2",
			connectionCount.Load(),
		)
	}
}


func TestOpenTCP_ConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	target := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	const sessionID = "12345678-1234-4234-8234-123456789012"

	sessions.Range(func(key, value any) bool {
		sessions.Delete(key)
		return true
	})

	done := make(chan struct{})
	defer close(done)

	send := make(chan brokerWriterMessage, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finished := make(chan struct{})

	go func() {
		openTCP(
			ctx,
			done,
			send,
			sessionID,
			target,
		)

		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("openTCP did not finish after target connection was refused")
	}

	if _, ok := sessions.Load(sessionID); ok {
		t.Fatalf("session %q is still present after failed TCP connection", sessionID)
	}

	select {
	case msg := <-send:
		if msg.messageType != websocket.BinaryMessage {
			t.Fatalf(
				"unexpected message type: got %d, want %d",
				msg.messageType,
				websocket.BinaryMessage,
			)
		}

		if len(msg.data) != 37 {
			t.Fatalf(
				"unexpected packet length: got %d, want 37",
				len(msg.data),
			)
		}

		gotID := string(msg.data[:36])
		if gotID != sessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				gotID,
				sessionID,
			)
		}

		gotCmd := msg.data[36]
		if gotCmd != CmdClose {
			t.Fatalf(
				"unexpected command: got %d, want CmdClose (%d)",
				gotCmd,
				CmdClose,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not send CmdClose after target connection failure")
	}

	select {
	case msg := <-send:
		if len(msg.data) >= 37 && msg.data[36] == CmdConnected {
			t.Fatal("worker sent CmdConnected despite target connection failure")
		}

		t.Fatalf(
			"unexpected additional broker message: cmd=%d",
			msg.data[36],
		)

	default:
	}
}

func TestRemoteReader_TargetDisconnect_SendsCmdClose(t *testing.T) {
	const sessionID = "12345678-1234-4234-8234-123456789012"

	sessions.Range(func(key, value any) bool {
		sessions.Delete(key)
		return true
	})

	clientConn, targetConn := net.Pipe()
	defer clientConn.Close()

	session := newSession(sessionID, "tcp")

	if !session.setConn(clientConn) {
		t.Fatal("failed to set session connection")
	}

	sessions.Store(sessionID, session)

	done := make(chan struct{})
	send := make(chan brokerWriterMessage, 1)

	go remoteReader(
		done,
		send,
		sessionID,
		session,
	)

	if err := targetConn.Close(); err != nil {
		t.Fatalf("close target connection: %v", err)
	}

	select {
	case msg := <-send:
		if msg.messageType != websocket.BinaryMessage {
			t.Fatalf(
				"unexpected message type: got %d, want %d",
				msg.messageType,
				websocket.BinaryMessage,
			)
		}

		if len(msg.data) != 37 {
			t.Fatalf(
				"unexpected packet length: got %d, want 37",
				len(msg.data),
			)
		}

		gotID := string(msg.data[:36])
		if gotID != sessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				gotID,
				sessionID,
			)
		}

		gotCmd := msg.data[36]
		if gotCmd != CmdClose {
			t.Fatalf(
				"unexpected command: got %d, want CmdClose (%d)",
				gotCmd,
				CmdClose,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not send CmdClose after target disconnect")
	}

	if _, ok := sessions.Load(sessionID); ok {
		t.Fatalf(
			"session %q is still present after target disconnect",
			sessionID,
		)
	}

	select {
	case <-session.done:
	default:
		t.Fatal("session.done was not closed after target disconnect")
	}
}

func TestOpenUDP_TargetError_ClosesSession(t *testing.T) {
	const sessionID = "12345678-1234-4234-8234-123456789012"

	sessions.Range(func(key, value any) bool {
		sessions.Delete(key)
		return true
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	send := make(chan brokerWriterMessage, 1)

	target := "127.0.0.1:not-a-port"

	finished := make(chan struct{})

	go func() {
		openUDP(
			ctx,
			done,
			send,
			sessionID,
			target,
		)

		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("openUDP did not finish after target error")
	}

	if _, ok := sessions.Load(sessionID); ok {
		t.Fatalf(
			"session %q is still present after UDP target error",
			sessionID,
		)
	}

	select {
	case msg := <-send:
		if len(msg.data) != 37 {
			t.Fatalf(
				"unexpected packet length: got %d, want 37",
				len(msg.data),
			)
		}

		gotID := string(msg.data[:36])
		if gotID != sessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				gotID,
				sessionID,
			)
		}

		gotCmd := msg.data[36]
		if gotCmd != CmdClose {
			t.Fatalf(
				"unexpected command: got %d, want CmdClose (%d)",
				gotCmd,
				CmdClose,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not send CmdClose after UDP target error")
	}
}
