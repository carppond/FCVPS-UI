package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/sshrelay"
	"shiguang-vps/internal/storage"
)

// SSH relay tuning.
const (
	// sshIdleTimeout drops a session whose client sends nothing (no keystrokes,
	// no pong) for this long. Generous so a left-open shell survives.
	sshIdleTimeout = 30 * time.Minute
	// sshPingInterval keeps the connection (and any intermediary proxy) warm.
	sshPingInterval  = 30 * time.Second
	sshWriteWait     = 10 * time.Second
	sshReadLimit     = 1 << 20 // 1 MiB — generous for paste, bounded vs abuse
	sshOutputBufSize = 32 * 1024
)

// SSHWSHandler bridges a browser/mobile xterm to an SSH shell on a VPS asset:
//
//	xterm  ──WSS──►  this handler  ──►  sshrelay (x/crypto/ssh PTY)  ──►  VPS
//
// Auth mirrors the SSE stream (Bearer header OR ?token=) because browser
// WebSocket clients cannot set an Authorization header. The asset is looked up
// scoped to the authenticated user, so one user can never open a shell on
// another user's host. SSH credentials are read server-side and never leave
// the hub.
//
// Wire protocol once upgraded:
//   - client → server: binary frame = raw keystrokes; text frame = control
//     JSON {"type":"resize","cols":N,"rows":M}
//   - server → client: binary frame = PTY output; text frame = a relay error
//     line (shown in the terminal before close)
type SSHWSHandler struct {
	tokens   *auth.TokenStore
	repo     *storage.VpsAssetRepo
	upgrader websocket.Upgrader
	logger   *slog.Logger
	now      func() time.Time
}

// NewSSHWSHandler wires the handler. tokens + repo are required; logger may be nil.
func NewSSHWSHandler(tokens *auth.TokenStore, repo *storage.VpsAssetRepo, logger *slog.Logger) *SSHWSHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &SSHWSHandler{
		tokens: tokens,
		repo:   repo,
		upgrader: websocket.Upgrader{
			HandshakeTimeout: 10 * time.Second,
			ReadBufferSize:   4096,
			WriteBufferSize:  4096,
			// The bearer token authenticates the request; the browser's
			// same-origin WS still carries it via ?token=. Mobile sends no Origin.
			CheckOrigin: func(*http.Request) bool { return true },
		},
		logger: logger,
		now:    time.Now,
	}
}

// resizeMsg is the only control message the client sends as a text frame.
type resizeMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// ServeHTTP implements GET /api/vps-assets/{id}/ssh.
func (h *SSHWSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := h.authenticate(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	assetID := r.PathValue("id")
	rec, err := h.repo.GetByID(r.Context(), assetID, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrVpsAssetNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "lookup failed", http.StatusInternalServerError)
		return
	}
	if rec.IP == "" || rec.SSHUser == "" {
		http.Error(w, "ssh not configured for this asset", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade already wrote the response.
	}
	defer conn.Close()

	sess, err := sshrelay.Dial(r.Context(), sshrelay.Target{
		Host:       rec.IP,
		Port:       rec.SSHPort,
		User:       rec.SSHUser,
		Password:   rec.SSHPassword,
		PrivateKey: rec.SSHPrivateKey,
	}, sshrelay.Config{Logger: h.logger})
	if err != nil {
		h.writeErrLine(conn, "连接失败 / connection failed: "+err.Error())
		h.logger.Info("ssh relay: dial failed",
			slog.String("user_id", user.ID),
			slog.String("asset_id", assetID),
			slog.String("err", err.Error()),
			slog.String("trace_id", traceID))
		return
	}
	defer sess.Close()

	h.logger.Info("ssh relay: session opened",
		slog.String("user_id", user.ID),
		slog.String("asset_id", assetID),
		slog.String("host", rec.IP),
		slog.String("trace_id", traceID))
	start := h.now()
	defer func() {
		h.logger.Info("ssh relay: session closed",
			slog.String("user_id", user.ID),
			slog.String("asset_id", assetID),
			slog.Duration("duration", h.now().Sub(start)))
	}()

	h.bridge(conn, sess)
}

// bridge pumps bytes both ways until either side closes. gorilla forbids
// concurrent writes, so all conn writes (output + pings) take wmu.
func (h *SSHWSHandler) bridge(conn *websocket.Conn, sess *sshrelay.Session) {
	var wmu sync.Mutex
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }

	// relay → ws (PTY output)
	go func() {
		defer finish()
		buf := make([]byte, sshOutputBufSize)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				wmu.Lock()
				_ = conn.SetWriteDeadline(h.now().Add(sshWriteWait))
				werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n])
				wmu.Unlock()
				if werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// ping keepalive
	go func() {
		ticker := time.NewTicker(sshPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				wmu.Lock()
				_ = conn.SetWriteDeadline(h.now().Add(sshWriteWait))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				wmu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// Unblock the read pump below when the relay side ends.
	go func() {
		<-done
		_ = conn.Close()
	}()

	// ws → relay (keystrokes + resize). Runs on this goroutine.
	conn.SetReadLimit(sshReadLimit)
	_ = conn.SetReadDeadline(h.now().Add(sshIdleTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(h.now().Add(sshIdleTimeout))
	})
	defer finish()
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(h.now().Add(sshIdleTimeout))
		switch mt {
		case websocket.BinaryMessage:
			if _, werr := sess.Write(data); werr != nil {
				return
			}
		case websocket.TextMessage:
			var msg resizeMsg
			if json.Unmarshal(data, &msg) == nil && msg.Type == "resize" {
				_ = sess.Resize(msg.Cols, msg.Rows)
			}
		}
	}
}

// writeErrLine pushes a human-readable error as a text frame so the terminal
// can display it, then lets the deferred Close drop the connection.
func (h *SSHWSHandler) writeErrLine(conn *websocket.Conn, msg string) {
	_ = conn.SetWriteDeadline(h.now().Add(sshWriteWait))
	_ = conn.WriteMessage(websocket.TextMessage, []byte(msg))
}

// authenticate accepts a Bearer header or ?token= query param, matching the
// SSE stream handler (browser WS clients cannot set headers).
func (h *SSHWSHandler) authenticate(r *http.Request) *storage.UserRecord {
	if h.tokens == nil {
		return nil
	}
	token := ""
	if c, err := r.Cookie(auth.SessionCookieName); err == nil && c.Value != "" {
		token = c.Value // web: httpOnly cookie auto-sent on same-origin WS
	} else if v := r.Header.Get("Authorization"); len(v) > 7 && v[:7] == "Bearer " {
		token = v[7:]
	} else if q := r.URL.Query().Get("token"); q != "" {
		token = q
	}
	if token == "" {
		return nil
	}
	result, err := h.tokens.Lookup(r.Context(), token)
	if err != nil || result == nil || result.User == nil {
		return nil
	}
	return result.User
}

// compile-time guard: ServeHTTP matches http.HandlerFunc.
var _ http.HandlerFunc = (*SSHWSHandler)(nil).ServeHTTP
