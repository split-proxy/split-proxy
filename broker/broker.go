package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
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

type Worker struct {
	conn       *websocket.Conn
	send       chan []byte
	done       chan struct{}
	workerGUID string
	groupName  string
	once       sync.Once
}

type DBWorker struct {
	GUID      string
	GroupName *string
	Online    bool
}

type ProxyConnection struct {
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

func newProxyConnection(conn *websocket.Conn) *ProxyConnection {
	p := &ProxyConnection{
		conn: conn,
		send: make(chan []byte, 2048),
		done: make(chan struct{}),
	}

	registerProxyConnection(p)

	go p.writer()

	return p
}

func (p *ProxyConnection) writer() {
	for {
		select {
		case <-p.done:
			return

		case msg := <-p.send:
			if err := p.conn.WriteMessage(
				websocket.BinaryMessage,
				msg,
			); err != nil {
				p.close(err)
				return
			}
		}
	}
}

func (p *ProxyConnection) write(msg []byte) error {
	if p == nil {
		return fmt.Errorf("proxy connection unavailable")
	}

	msg = append([]byte(nil), msg...)

	select {
	case <-p.done:
		return fmt.Errorf("proxy connection closed")

	case p.send <- msg:
		return nil
	}
}

// close closes only the proxy transport and all logical sessions belonging to it.
func (p *ProxyConnection) close(err error) {
	p.closeOnce.Do(func() {
		unregisterProxyConnection(p)

		close(p.done)

		if p.conn != nil {
			_ = p.conn.Close()
		}

		sessions.Range(func(key, value any) bool {
			id, ok := key.(string)
			if !ok {
				return true
			}

			s := value.(*Session)

			if s.proxy == p {
				if s.worker != nil {
					go func(id string, worker *Worker) {
						_ = sendToWorker(
							worker,
							id,
							CmdClose,
							nil,
						)
					}(id, s.worker)
				}

				sessions.Delete(key)
			}

			return true
		})

		if err != nil {
			debugf(
				"[BROKER] proxy connection closed: %v",
				err,
			)
		}
	})
}

type Session struct {
	proxy  *ProxyConnection
	worker *Worker
	mode   string
}

type WorkerGroup struct {
	workers []*Worker
	rr      int
}

type WorkerHello struct {
	WorkerGUID string `json:"workerGUID"`
	Token      string `json:"token"`
}

type ProxyHello struct {
	Token string `json:"token"`
}

type BrokerRequest struct {
	ID     string `json:"id"`
	Route  string `json:"route"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
}

var (
	pool *pgxpool.Pool

	groups   = map[string]*WorkerGroup{}
	groupsMu sync.RWMutex

	activeWorkers   = map[string]*Worker{}
	activeWorkersMu sync.RWMutex

	sessions sync.Map

	proxyConnections   = map[*ProxyConnection]struct{}{}
	proxyConnectionsMu sync.Mutex

	upgrader = websocket.Upgrader{
		ReadBufferSize:  64 * 1024,
		WriteBufferSize: 64 * 1024,
    CheckOrigin: func(r *http.Request) bool {
      return r.Header.Get("Origin") == ""
    },
	}

	workerToken = os.Getenv("WORKER_TOKEN")
	proxyToken  = os.Getenv("PROXY_TOKEN")
)

const (
	CmdData       byte = 0
	CmdConnect    byte = 1
	CmdConnected  byte = 2
	CmdClose      byte = 3
	CmdError      byte = 4
	CmdUDPConnect byte = 5
	CmdUDPData    byte = 6

	DBQueryTimeout     = 5 * time.Second
	WorkerSyncTimeout  = 10 * time.Second
	WorkerSendTimeout  = 30 * time.Second
	WorkerSyncInterval = 5 * time.Second
	WorkerReadLimit    = 128 * 1024

	WorkerPingInterval = 30 * time.Second
	WorkerPongTimeout   = 60 * time.Second
	WorkerPingTimeout   = 10 * time.Second

	BrokerShutdownTimeout = 10 * time.Second
)

var (
	shuttingDownMu sync.RWMutex
	shuttingDown   bool
)

func isShuttingDown() bool {
	shuttingDownMu.RLock()
	defer shuttingDownMu.RUnlock()

	return shuttingDown
}

func setShuttingDown() {
	shuttingDownMu.Lock()
	shuttingDown = true
	shuttingDownMu.Unlock()
}

func registerProxyConnection(p *ProxyConnection) {
	proxyConnectionsMu.Lock()
	proxyConnections[p] = struct{}{}
	proxyConnectionsMu.Unlock()
}

func unregisterProxyConnection(p *ProxyConnection) {
	proxyConnectionsMu.Lock()
	delete(proxyConnections, p)
	proxyConnectionsMu.Unlock()
}

func closeAllProxyConnections() {
	proxyConnectionsMu.Lock()

	connections := make([]*ProxyConnection, 0, len(proxyConnections))

	for p := range proxyConnections {
		connections = append(connections, p)
	}

	proxyConnectionsMu.Unlock()

	for _, p := range connections {
		p.close(nil)
	}
}

func closeAllWorkers() {
	activeWorkersMu.RLock()

	workers := make([]*Worker, 0, len(activeWorkers))

	for _, w := range activeWorkers {
		workers = append(workers, w)
	}

	activeWorkersMu.RUnlock()

	for _, w := range workers {
		removeWorker(w)
	}
}

func closeAllSessions() {
	sessions.Range(func(key, value any) bool {
		sessions.Delete(key)
		return true
	})
}

func main() {
	initLogger()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer stop()

	infof(
		"[BROKER] starting log_level=%s",
		strings.ToLower(os.Getenv("LOG_LEVEL")),
	)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("[BROKER] DATABASE_URL is not set")
	}

	var err error

	pool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal("[BROKER] failed to create postgres pool:", err)
	}

	defer pool.Close()

	secret := os.Getenv("RANDOM_ENDPOINT_SECRET")
	if secret == "" {
		log.Fatal("[BROKER] RANDOM_ENDPOINT_SECRET is not set")
	}

	http.HandleFunc(
		"/ws/proxy/"+secret,
		proxyHandler,
	)

	http.HandleFunc(
		"/ws/worker/"+secret,
		workerHandler,
	)

	port := os.Getenv("BROKER_PORT")
	if port == "" {
		log.Fatal("[BROKER] BROKER_PORT is not set")
	}

	go workerSyncLoop(ctx)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           nil,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	infof("[BROKER] listening :%s", port)

	serverErr := make(chan error, 1)

	go func() {
		err := server.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}

		serverErr <- nil
	}()

	select {
	case <-ctx.Done():
		infof("[BROKER] shutdown signal received")

	case err := <-serverErr:
		if err != nil {
			errorf("[BROKER] HTTP server failed: %v", err)
		}

		return
	}

	setShuttingDown()

	infof("[BROKER] stopping new connections")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		BrokerShutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		warnf(
			"[BROKER] HTTP server shutdown error: %v",
			err,
		)
	}

	infof("[BROKER] closing worker connections")

	closeAllWorkers()

	infof("[BROKER] closing proxy connections")

	closeAllProxyConnections()

	infof("[BROKER] clearing sessions")

	closeAllSessions()

	infof("[BROKER] closing postgres pool")

	pool.Close()

	infof("[BROKER] shutdown complete")
}

func registerWorker(
	ctx context.Context,
	pool *pgxpool.Pool,
	workerGUID,
	ip string,
) error {
	qctx, cancel := context.WithTimeout(
		ctx,
		DBQueryTimeout,
	)
	defer cancel()

	_, err := pool.Exec(
		qctx,
		`INSERT INTO core_worker
			(guid, ip_address, online, created_at, updated_at)
			VALUES ($1,$2,TRUE,NOW(),NOW())
			ON CONFLICT (guid)
			DO UPDATE SET
				ip_address=EXCLUDED.ip_address,
				online=TRUE,
				updated_at=NOW()`,
		workerGUID,
		ip,
	)

	return err
}

func removeWorker(w *Worker) {
	if w == nil {
		return
	}

	w.once.Do(func() {
		active := false

		activeWorkersMu.Lock()

		if activeWorkers[w.workerGUID] == w {
			delete(activeWorkers, w.workerGUID)
			active = true
		}

		activeWorkersMu.Unlock()

		removeWorkerFromGroup(w)

		sessions.Range(func(key, value any) bool {
			id, ok := key.(string)
			if !ok {
				return true
			}

			s := value.(*Session)

			if s.worker == w {
				if s.proxy != nil {
					_ = s.proxy.write(
						packet(id, CmdClose, nil),
					)
				}

				sessions.Delete(key)
			}

			return true
		})

		close(w.done)

		if w.conn != nil {
			_ = w.conn.Close()
		}

		if !active {
			return
		}

		qctx, cancel := context.WithTimeout(
			context.Background(),
			DBQueryTimeout,
		)
		defer cancel()

		if _, err := pool.Exec(
			qctx,
			`UPDATE core_worker
			 SET online=FALSE, updated_at=NOW()
			 WHERE guid=$1`,
			w.workerGUID,
		); err != nil {
			errorf(
				"[BROKER] failed to mark worker %s offline: %v",
				w.workerGUID,
				err,
			)
		}
	})
}

func getClientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if ip := strings.TrimSpace(strings.Split(forwarded, ",")[0]); ip != "" {
			return ip
		}
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	return r.RemoteAddr
}

func workerHandler(w http.ResponseWriter, r *http.Request) {
	if isShuttingDown() {
		http.Error(
			w,
			"broker shutting down",
			http.StatusServiceUnavailable,
		)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		errorf(
			"[BROKER] worker upgrade error: %v",
			err,
		)
		return
	}

	if isShuttingDown() {
		_ = ws.Close()
		return
	}

	ws.SetReadLimit(WorkerReadLimit)

	_ = ws.SetReadDeadline(
		time.Now().Add(WorkerPongTimeout),
	)

	ws.SetPongHandler(
		func(string) error {
			return ws.SetReadDeadline(
				time.Now().Add(WorkerPongTimeout),
			)
		},
	)

	_, msg, err := ws.ReadMessage()
	if err != nil {
		_ = ws.Close()
		return
	}

	var hello WorkerHello

	if err = json.Unmarshal(msg, &hello); err != nil ||
		hello.WorkerGUID == "" ||
		hello.Token != workerToken {
		_ = ws.Close()
		return
	}

	if isShuttingDown() {
		_ = ws.Close()
		return
	}

	ip := getClientIP(r)

	if err = registerWorker(
		r.Context(),
		pool,
		hello.WorkerGUID,
		ip,
	); err != nil {
		_ = ws.Close()
		return
	}

	groupName, err := getWorkerGroup(
		r.Context(),
		hello.WorkerGUID,
	)
	if err != nil {
		_ = ws.Close()
		return
	}

	if isShuttingDown() {
		_ = ws.Close()
		return
	}

	wrk := &Worker{
		conn:       ws,
		send:       make(chan []byte, 4096),
		done:       make(chan struct{}),
		workerGUID: hello.WorkerGUID,
		groupName:  groupName,
	}

	old := getActiveWorker(wrk.workerGUID)

	setActiveWorker(wrk)
	addWorkerToGroup(wrk)

	if old != nil {
		removeWorker(old)
	}

	infof(
		"[BROKER] worker connected guid=%s group=%s",
		hello.WorkerGUID,
		groupName,
	)

	go workerWriter(wrk)
	go workerReader(wrk)
	go workerPing(wrk)
}

func workerWriter(w *Worker) {
	for {
		select {
		case <-w.done:
			return

		case msg := <-w.send:
			if err := w.conn.WriteMessage(
				websocket.BinaryMessage,
				msg,
			); err != nil {
				removeWorker(w)
				return
			}
		}
	}
}

func workerPing(w *Worker) {
	t := time.NewTicker(WorkerPingInterval)
	defer t.Stop()

	for {
		select {
		case <-w.done:
			return

		case <-t.C:
			if err := w.conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(WorkerPingTimeout),
			); err != nil {
				removeWorker(w)
				return
			}
		}
	}
}

func packet(id string, cmd byte, data []byte) []byte {
	b := make([]byte, 37+len(data))

	copy(b[:36], id)
	b[36] = cmd

	if len(data) > 0 {
		copy(b[37:], data)
	}

	return b
}

func workerReader(w *Worker) {
	for {
		_, msg, err := w.conn.ReadMessage()
		if err != nil {
			removeWorker(w)
			return
		}

		if len(msg) < 37 {
			continue
		}

		id := string(msg[:36])
		cmd := msg[36]

		v, ok := sessions.Load(id)
		if !ok {
			continue
		}

		s := v.(*Session)

		if s.worker != w {
			continue
		}

		switch cmd {
		case CmdData, CmdUDPData, CmdConnected:
			if s.proxy == nil ||
				s.proxy.write(msg) != nil {
				sessions.Delete(id)
			}

		case CmdClose:
			if s.proxy != nil {
				_ = s.proxy.write(msg)
			}

			sessions.Delete(id)

		default:
			warnf(
				"[BROKER] unknown worker command=%d session=%s",
				cmd,
				id,
			)
		}
	}
}

func getWorker(groupName string) *Worker {
	if isShuttingDown() {
		return nil
	}

	groupsMu.Lock()
	defer groupsMu.Unlock()

	g := groups[groupName]

	if g == nil || len(g.workers) == 0 {
		return nil
	}

	w := g.workers[g.rr%len(g.workers)]
	g.rr++

	return w
}

func getWorkerGroup(
	ctx context.Context,
	guid string,
) (string, error) {
	qctx, cancel := context.WithTimeout(
		ctx,
		DBQueryTimeout,
	)
	defer cancel()

	var g *string

	err := pool.QueryRow(
		qctx,
		`SELECT wg.group_name
		 FROM core_worker w
		 LEFT JOIN core_workergroup wg
		   ON wg.id=w.group_id
		 WHERE w.guid=$1`,
		guid,
	).Scan(&g)

	if err != nil {
		return "", err
	}

	if g == nil {
		return "", nil
	}

	return *g, nil
}

func addWorkerToGroup(w *Worker) {
	if isShuttingDown() {
		return
	}

	groupsMu.Lock()
	defer groupsMu.Unlock()

	g := groups[w.groupName]

	if g == nil {
		g = &WorkerGroup{}
		groups[w.groupName] = g
	}

	g.workers = append(g.workers, w)
}

func removeWorkerFromGroup(w *Worker) {
	groupsMu.Lock()
	defer groupsMu.Unlock()

	g := groups[w.groupName]

	if g == nil {
		return
	}

	for i, x := range g.workers {
		if x == w {
			g.workers = append(
				g.workers[:i],
				g.workers[i+1:]...,
			)
			break
		}
	}

	if len(g.workers) == 0 {
		delete(groups, w.groupName)
	}
}

func loadWorkersFromDB(
	ctx context.Context,
) ([]DBWorker, error) {
	qctx, cancel := context.WithTimeout(
		ctx,
		DBQueryTimeout,
	)
	defer cancel()

	rows, err := pool.Query(
		qctx,
		`SELECT
			w.guid,
			wg.group_name,
			w.online
		 FROM core_worker w
		 LEFT JOIN core_workergroup wg
		   ON wg.id=w.group_id`,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []DBWorker

	for rows.Next() {
		var x DBWorker

		if err := rows.Scan(
			&x.GUID,
			&x.GroupName,
			&x.Online,
		); err != nil {
			return nil, err
		}

		out = append(out, x)
	}

	return out, rows.Err()
}

func workerSyncLoop(ctx context.Context) {
	t := time.NewTicker(WorkerSyncInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			infof("[BROKER] worker sync loop stopped")
			return

		case <-t.C:
			if isShuttingDown() {
				return
			}

			syncWorkersFromDB(ctx)
		}
	}
}

func syncWorkersFromDB(parentCtx context.Context) {
	if isShuttingDown() {
		return
	}

	ctx, cancel := context.WithTimeout(
		parentCtx,
		WorkerSyncTimeout,
	)
	defer cancel()

	ws, err := loadWorkersFromDB(ctx)
	if err != nil {
		if ctx.Err() == nil {
			warnf(
				"[BROKER] worker sync failed: %v",
				err,
			)
		}

		return
	}

	for _, dbw := range ws {
		if isShuttingDown() {
			return
		}

		if w := getActiveWorker(dbw.GUID); w != nil {
			g := ""

			if dbw.GroupName != nil {
				g = *dbw.GroupName
			}

			moveWorkerToGroup(w, g)
		}
	}
}

func moveWorkerToGroup(
	w *Worker,
	newGroup string,
) {
	if isShuttingDown() {
		return
	}

	groupsMu.Lock()
	defer groupsMu.Unlock()

	old := w.groupName

	if old == newGroup {
		return
	}

	if g := groups[old]; g != nil {
		for i, x := range g.workers {
			if x == w {
				g.workers = append(
					g.workers[:i],
					g.workers[i+1:]...,
				)
				break
			}
		}

		if len(g.workers) == 0 {
			delete(groups, old)
		}
	}

	if newGroup != "" {
		g := groups[newGroup]

		if g == nil {
			g = &WorkerGroup{}
			groups[newGroup] = g
		}

		g.workers = append(g.workers, w)
	}

	w.groupName = newGroup
}

func setActiveWorker(w *Worker) {
	if isShuttingDown() {
		return
	}

	activeWorkersMu.Lock()
	activeWorkers[w.workerGUID] = w
	activeWorkersMu.Unlock()
}

func getActiveWorker(guid string) *Worker {
	activeWorkersMu.RLock()
	defer activeWorkersMu.RUnlock()

	return activeWorkers[guid]
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	if isShuttingDown() {
		http.Error(
			w,
			"broker shutting down",
			http.StatusServiceUnavailable,
		)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	if isShuttingDown() {
		_ = ws.Close()
		return
	}

	ws.SetReadLimit(WorkerReadLimit)

	pc := newProxyConnection(ws)

	defer pc.close(nil)

	_, msg, err := ws.ReadMessage()
	if err != nil {
		return
	}

	var auth ProxyHello

	if err = json.Unmarshal(msg, &auth); err != nil ||
		auth.Token != proxyToken {
		return
	}

	if isShuttingDown() {
		return
	}

	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			return
		}

		if isShuttingDown() {
			return
		}

		switch mt {
		case websocket.TextMessage:
			var req BrokerRequest

			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}

			if err := openProxySession(
				pc,
				req,
			); err != nil {
				warnf(
					"[BROKER] open session failed: %v",
					err,
				)
			}

		case websocket.BinaryMessage:
			if len(msg) < 37 {
				continue
			}

			id := string(msg[:36])
			cmd := msg[36]

			v, ok := sessions.Load(id)
			if !ok {
				continue
			}

			s := v.(*Session)

			if s.proxy != pc {
				continue
			}

			switch cmd {
			case CmdData, CmdUDPData:
				if err := sendToWorker(
					s.worker,
					id,
					cmd,
					msg[37:],
				); err != nil {
					sessions.Delete(id)

					_ = pc.write(
						packet(id, CmdClose, nil),
					)
				}

			case CmdClose:
				_ = sendToWorker(
					s.worker,
					id,
					CmdClose,
					nil,
				)

				sessions.Delete(id)
			}
		}
	}
}

func sendProxySessionError(
	pc *ProxyConnection,
	id string,
	err error,
) {
	if err == nil || pc == nil {
		return
	}

	_ = pc.write(
		packet(
			id,
			CmdError,
			[]byte(err.Error()),
		),
	)
}

func openProxySession(
	pc *ProxyConnection,
	req BrokerRequest,
) error {
	if isShuttingDown() {
		err := fmt.Errorf("broker shutting down")

		sendProxySessionError(
			pc,
			req.ID,
			err,
		)

		return err
	}

	mode := req.Mode

	if mode == "" {
		mode = "tcp"
	}

	if mode != "tcp" && mode != "udp" {
		return fmt.Errorf(
			"invalid session mode=%q",
			mode,
		)
	}

	id := req.ID

	if id == "" {
		id = uuid.New().String()
	}

	if len(id) != 36 {
		return fmt.Errorf(
			"invalid session id=%q",
			id,
		)
	}

	if _, ok := sessions.Load(id); ok {
		return fmt.Errorf(
			"duplicate session id=%s",
			id,
		)
	}

	w := getWorker(req.Route)

	if w == nil {
		err := fmt.Errorf(
			"no workers available for group=%s",
			req.Route,
		)

		sendProxySessionError(
			pc,
			id,
			err,
		)

		return err
	}

	if isShuttingDown() {
		err := fmt.Errorf("broker shutting down")

		sendProxySessionError(
			pc,
			id,
			err,
		)

		return err
	}

	sessions.Store(
		id,
		&Session{
			proxy:  pc,
			worker: w,
			mode:   mode,
		},
	)

	cmd := CmdConnect

	if mode == "udp" {
		cmd = CmdUDPConnect
	}

	if err := sendToWorker(
		w,
		id,
		cmd,
		[]byte(req.Target),
	); err != nil {
		sessions.Delete(id)

		sendProxySessionError(
			pc,
			id,
			err,
		)

		return err
	}

	return nil
}

func sendToWorker(
	w *Worker,
	id string,
	cmd byte,
	data []byte,
) error {
	if isShuttingDown() {
		return fmt.Errorf("broker shutting down")
	}

	if w == nil {
		return fmt.Errorf("worker unavailable")
	}

	buf := packet(
		id,
		cmd,
		data,
	)

	t := time.NewTimer(WorkerSendTimeout)
	defer t.Stop()

	select {
	case <-w.done:
		return fmt.Errorf("worker disconnected")

	case w.send <- buf:
		return nil

	case <-t.C:
		return fmt.Errorf("worker send timeout")
	}
}
