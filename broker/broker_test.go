package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const testSessionID = "12345678-1234-4234-8234-123456789012"

func newTestWorker(t *testing.T) (*websocket.Conn, *Worker) {
	t.Helper()

	serverConnCh := make(chan *websocket.Conn, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}

		serverConnCh <- conn
	}))

	t.Cleanup(server.Close)

	wsURL := "ws" + server.URL[len("http"):]

	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}

	var workerConn *websocket.Conn

	select {
	case workerConn = <-serverConnCh:
	case <-time.After(time.Second):
		_ = clientConn.Close()
		t.Fatal("timed out waiting for worker websocket connection")
	}

	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = workerConn.Close()
	})

	worker := &Worker{
		conn:       workerConn,
		send:       make(chan []byte, 2048),
		done:       make(chan struct{}),
		workerGUID: "test-worker",
	}

	return clientConn, worker
}

func newTestProxy() *ProxyConnection {
	return &ProxyConnection{
		send: make(chan []byte, 16),
		done: make(chan struct{}),
	}
}

func newTestSession(worker *Worker, proxy *ProxyConnection) *Session {
	return &Session{
		proxy:  proxy,
		worker: worker,
		mode:   "tcp",
	}
}

func clearTestSessions() {
	sessions.Range(func(key, value any) bool {
		sessions.Delete(key)
		return true
	})
}

func waitForSessionRemoval(t *testing.T, id string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		if _, ok := sessions.Load(id); !ok {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatalf("session %q was not removed", id)
}

func TestWorkerReader_CmdData_DeliversPayloadToCorrectSession(t *testing.T) {
	clearTestSessions()
	t.Cleanup(clearTestSessions)

	clientConn, worker := newTestWorker(t)
	proxy := newTestProxy()

	session := newTestSession(worker, proxy)

	sessions.Store(testSessionID, session)

	go workerReader(worker)

	payload := []byte("hello over tcp")
	msg := packet(testSessionID, CmdData, payload)

	if err := clientConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatalf("write CmdData: %v", err)
	}

	select {
	case got := <-proxy.send:
		if string(got) != string(msg) {
			t.Fatalf(
				"unexpected message: got %q, want %q",
				got,
				msg,
			)
		}

		if len(got) < 37 {
			t.Fatalf("message is too short: %d bytes", len(got))
		}

		if string(got[:36]) != testSessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				string(got[:36]),
				testSessionID,
			)
		}

		if got[36] != CmdData {
			t.Fatalf(
				"unexpected command: got %d, want %d",
				got[36],
				CmdData,
			)
		}

		if string(got[37:]) != string(payload) {
			t.Fatalf(
				"unexpected payload: got %q, want %q",
				string(got[37:]),
				string(payload),
			)
		}

	case <-time.After(time.Second):
		t.Fatal("proxy did not receive CmdData")
	}

	value, ok := sessions.Load(testSessionID)
	if !ok {
		t.Fatal("session was unexpectedly removed")
	}

	if value.(*Session) != session {
		t.Fatal("CmdData was delivered to the wrong session")
	}

	removeWorker(worker)
}

func TestWorkerReader_CmdUDPData_DeliversPayloadToCorrectSession(t *testing.T) {
	clearTestSessions()
	t.Cleanup(clearTestSessions)

	clientConn, worker := newTestWorker(t)
	proxy := newTestProxy()

	session := newTestSession(worker, proxy)
	session.mode = "udp"

	sessions.Store(testSessionID, session)

	go workerReader(worker)

	payload := []byte("hello udp")
	msg := packet(testSessionID, CmdUDPData, payload)

	if err := clientConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatalf("write CmdUDPData: %v", err)
	}

	select {
	case got := <-proxy.send:
		if string(got) != string(msg) {
			t.Fatalf(
				"unexpected message: got %q, want %q",
				got,
				msg,
			)
		}

		if len(got) < 37 {
			t.Fatalf("message is too short: %d bytes", len(got))
		}

		if string(got[:36]) != testSessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				string(got[:36]),
				testSessionID,
			)
		}

		if got[36] != CmdUDPData {
			t.Fatalf(
				"unexpected command: got %d, want %d",
				got[36],
				CmdUDPData,
			)
		}

		if string(got[37:]) != string(payload) {
			t.Fatalf(
				"unexpected payload: got %q, want %q",
				string(got[37:]),
				string(payload),
			)
		}

	case <-time.After(time.Second):
		t.Fatal("proxy did not receive CmdUDPData")
	}

	value, ok := sessions.Load(testSessionID)
	if !ok {
		t.Fatal("session was unexpectedly removed")
	}

	if value.(*Session) != session {
		t.Fatal("CmdUDPData was delivered to the wrong session")
	}

	removeWorker(worker)
}

func TestWorkerReader_CmdClose_RemovesSessionAndNotifiesProxy(t *testing.T) {
	clearTestSessions()
	t.Cleanup(clearTestSessions)

	clientConn, worker := newTestWorker(t)
	proxy := newTestProxy()

	session := newTestSession(worker, proxy)

	sessions.Store(testSessionID, session)

	go workerReader(worker)

	msg := packet(testSessionID, CmdClose, nil)

	if err := clientConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatalf("write CmdClose: %v", err)
	}

	select {
	case got := <-proxy.send:
		if string(got) != string(msg) {
			t.Fatalf(
				"unexpected message: got %q, want %q",
				got,
				msg,
			)
		}

		if len(got) != 37 {
			t.Fatalf(
				"unexpected CmdClose packet length: got %d, want 37",
				len(got),
			)
		}

		if string(got[:36]) != testSessionID {
			t.Fatalf(
				"unexpected session ID: got %q, want %q",
				string(got[:36]),
				testSessionID,
			)
		}

		if got[36] != CmdClose {
			t.Fatalf(
				"unexpected command: got %d, want %d",
				got[36],
				CmdClose,
			)
		}

	case <-time.After(time.Second):
		t.Fatal("proxy did not receive CmdClose")
	}

	waitForSessionRemoval(t, testSessionID)

	removeWorker(worker)
}

func TestWorkerReader_UnknownSessionID_IsIgnored(t *testing.T) {
	clearTestSessions()
	t.Cleanup(clearTestSessions)

	clientConn, worker := newTestWorker(t)
	proxy := newTestProxy()

	session := newTestSession(worker, proxy)

	sessions.Store(testSessionID, session)

	go workerReader(worker)

	unknownID := "99999999-9999-4999-8999-999999999999"

	msg := packet(
		unknownID,
		CmdData,
		[]byte("must be ignored"),
	)

	if err := clientConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
		t.Fatalf(
			"write unknown-session packet: %v",
			err,
		)
	}

	select {
	case got := <-proxy.send:
		t.Fatalf(
			"unexpected message delivered to proxy: %q",
			got,
		)

	case <-time.After(100 * time.Millisecond):
	}

	value, ok := sessions.Load(testSessionID)
	if !ok {
		t.Fatal("known session was unexpectedly removed")
	}

	if value.(*Session) != session {
		t.Fatal("known session was replaced")
	}

	removeWorker(worker)
}

func TestWorkerReader_MalformedPacket_IsIgnored(t *testing.T) {
	clearTestSessions()
	t.Cleanup(clearTestSessions)

	clientConn, worker := newTestWorker(t)
	proxy := newTestProxy()

	session := newTestSession(worker, proxy)

	sessions.Store(testSessionID, session)

	go workerReader(worker)

	malformedPackets := [][]byte{
		{},
		[]byte("short"),
		make([]byte, 36),
	}

	for i, msg := range malformedPackets {
		if err := clientConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
			t.Fatalf(
				"write malformed packet #%d: %v",
				i,
				err,
			)
		}
	}

	select {
	case got := <-proxy.send:
		t.Fatalf(
			"unexpected message delivered to proxy: %q",
			got,
		)

	case <-time.After(100 * time.Millisecond):
	}

	value, ok := sessions.Load(testSessionID)
	if !ok {
		t.Fatal("session was unexpectedly removed after malformed packet")
	}

	if value.(*Session) != session {
		t.Fatal("session was unexpectedly replaced")
	}

	removeWorker(worker)
}
