// Package transport owns the agent ↔ hub WebSocket session.
//
// The Client is a long-lived value: ConnectWithBackoff drives the dial loop
// with exponential backoff, Run drives one connected session (read pump +
// write pump + heartbeat + command handling), and Close terminates the
// session gracefully.
//
// On bye{reason:"version_unsupported"} the client stops reconnecting
// (§1.8 protocol negotiation) and logs a "need upgrade" warning. Any other
// disconnect retries up to a 60 s ceiling.
package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/cmd/agent/internal/collector"
	"shiguang-vps/pkg/agentlib"
)

// Config wires a Client to its hub + the local environment. AgentID + Token
// + HubURL are required; the remaining fields fall back to sensible defaults.
type Config struct {
	HubURL  string // e.g. wss://hub.example.com/api/agent/ws
	Token   string // plaintext, sent as ?token=…
	AgentID string // UUID assigned during agent registration
	Version string // semver of this agent binary
	Tags    []string

	// HeartbeatInterval is the initial cadence; hello_ack may override it.
	HeartbeatInterval time.Duration
	// CollectInterval is the metrics emission cadence (defaults to the
	// HeartbeatInterval value).
	CollectInterval time.Duration
	// HandshakeTimeout caps the WebSocket handshake + hello_ack roundtrip.
	HandshakeTimeout time.Duration
	// WriteTimeout caps each outbound message.
	WriteTimeout time.Duration
	// MaxMessageBytes is the inbound read limit (defense in depth — hub
	// messages are tiny in v1).
	MaxMessageBytes int64

	// BackoffMin / BackoffMax bound the reconnect schedule.
	BackoffMin time.Duration
	BackoffMax time.Duration

	// Aggregator collects the metrics payload. If nil, a default aggregator
	// stamped with AgentID is built lazily.
	Aggregator *collector.Aggregator

	// Dialer is overridable for tests. nil → websocket.DefaultDialer.
	Dialer *websocket.Dialer
	// Now is the clock source (test injectable).
	Now func() time.Time
	// Logger may be nil; defaults to slog.Default().
	Logger *slog.Logger
}

// Defaults applied lazily in NewClient.
const (
	defaultHeartbeat       = 30 * time.Second
	defaultHandshake       = 10 * time.Second
	defaultWriteTimeout    = 10 * time.Second
	defaultMaxMessageBytes = 256 * 1024
	defaultBackoffMin      = 1 * time.Second
	defaultBackoffMax      = 60 * time.Second
)

// ErrStopReconnect is returned by ConnectWithBackoff when the hub explicitly
// asked the agent to stop retrying (bye{version_unsupported} or
// bye{agent_deleted}). main() should exit non-zero on this so an operator's
// process supervisor surfaces the upgrade requirement.
var ErrStopReconnect = errors.New("transport: stop reconnect (hub rejected agent)")

// Client is the connected session. Construction with NewClient is mandatory.
type Client struct {
	cfg Config

	heartbeatInterval atomic.Int64 // nanos; updated by hello_ack
	stopReconnect     atomic.Bool

	// per-session state — rotated on each new connection.
	connMu sync.Mutex
	conn   *websocket.Conn
	send   chan []byte
	done   chan struct{} // closed when the session terminates
}

// NewClient validates cfg and applies defaults. Returns an error if a
// required field is missing or HubURL is unparseable.
func NewClient(cfg Config) (*Client, error) {
	if cfg.HubURL == "" {
		return nil, fmt.Errorf("transport: HubURL is required")
	}
	if cfg.AgentID == "" {
		return nil, fmt.Errorf("transport: AgentID is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("transport: Token is required")
	}
	if _, err := url.Parse(cfg.HubURL); err != nil {
		return nil, fmt.Errorf("transport: parse HubURL: %w", err)
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = defaultHeartbeat
	}
	if cfg.CollectInterval <= 0 {
		cfg.CollectInterval = cfg.HeartbeatInterval
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = defaultHandshake
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.MaxMessageBytes <= 0 {
		cfg.MaxMessageBytes = defaultMaxMessageBytes
	}
	if cfg.BackoffMin <= 0 {
		cfg.BackoffMin = defaultBackoffMin
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = defaultBackoffMax
	}
	if cfg.Aggregator == nil {
		cfg.Aggregator = collector.NewAggregator(collector.Config{AgentID: cfg.AgentID})
	}
	if cfg.Dialer == nil {
		cfg.Dialer = &websocket.Dialer{HandshakeTimeout: cfg.HandshakeTimeout}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	c := &Client{cfg: cfg}
	c.heartbeatInterval.Store(int64(cfg.HeartbeatInterval))
	return c, nil
}

// HeartbeatInterval returns the current (possibly hub-overridden) cadence.
func (c *Client) HeartbeatInterval() time.Duration {
	return time.Duration(c.heartbeatInterval.Load())
}

// StopReconnect signals ConnectWithBackoff to bail out at the next iteration.
// Used both internally (on bye{version_unsupported}) and by Close().
func (c *Client) StopReconnect() { c.stopReconnect.Store(true) }

// Connect establishes one WebSocket session: dial, send hello, read
// hello_ack. Returns the connected websocket.Conn so callers (typically
// Run) can drive the message pumps.
//
// On bye{version_unsupported} returned during the ack phase the function
// flips StopReconnect and returns ErrStopReconnect.
func (c *Client) Connect(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	u, err := buildWSURL(c.cfg.HubURL, c.cfg.Token)
	if err != nil {
		return fmt.Errorf("transport connect: build url: %w", err)
	}
	conn, _, err := c.cfg.Dialer.DialContext(ctx, u, http.Header{})
	if err != nil {
		return fmt.Errorf("transport connect: dial: %w", err)
	}
	conn.SetReadLimit(c.cfg.MaxMessageBytes)

	// Send hello.
	hello := agentlib.HelloPayload{
		AgentID:      c.cfg.AgentID,
		Token:        c.cfg.Token,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		Version:      c.cfg.Version,
		Kind:         agentlib.KindNative,
		Capabilities: []string{"metrics", "restart"},
	}
	if err := c.writeEnvelope(conn, agentlib.MsgHello, newMsgID("hello"), hello); err != nil {
		_ = conn.Close()
		return fmt.Errorf("transport connect: send hello: %w", err)
	}

	// Read hello_ack (or a bye that aborts the handshake).
	_ = conn.SetReadDeadline(c.cfg.Now().Add(c.cfg.HandshakeTimeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("transport connect: read hello_ack: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	env, err := agentlib.UnmarshalEnvelope(data)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("transport connect: parse envelope: %w", err)
	}
	switch env.Type {
	case agentlib.MsgHelloAck:
		ack, err := agentlib.UnmarshalHelloAck(env.Payload)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("transport connect: parse hello_ack: %w", err)
		}
		if !ack.OK {
			_ = conn.Close()
			return fmt.Errorf("transport connect: hello_ack rejected")
		}
		if ack.HeartbeatInterval > 0 {
			c.heartbeatInterval.Store(int64(time.Duration(ack.HeartbeatInterval) * time.Second))
		}
		c.cfg.Logger.Info("agent transport: connected",
			slog.String("hub_url", c.cfg.HubURL),
			slog.String("hub_version", ack.HubVersion),
			slog.Duration("heartbeat", c.HeartbeatInterval()),
		)
	case agentlib.MsgBye:
		c.handleBye(env, conn)
		return ErrStopReconnect
	default:
		_ = conn.Close()
		return fmt.Errorf("transport connect: unexpected envelope type %q", env.Type)
	}

	c.connMu.Lock()
	c.conn = conn
	c.send = make(chan []byte, 32)
	c.done = make(chan struct{})
	c.connMu.Unlock()
	return nil
}

// ConnectWithBackoff repeatedly calls Connect with exponential backoff
// (BackoffMin → 2× → … → BackoffMax). Returns nil on the first successful
// connect; returns ctx.Err() if ctx is cancelled and ErrStopReconnect when
// the hub asks the agent to stop.
func (c *Client) ConnectWithBackoff(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	delay := c.cfg.BackoffMin
	for {
		if c.stopReconnect.Load() {
			return ErrStopReconnect
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.Connect(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrStopReconnect) {
			return err
		}
		c.cfg.Logger.Warn("agent transport: connect failed, retrying",
			slog.String("err", err.Error()),
			slog.Duration("delay", delay),
		)
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
		delay *= 2
		if delay > c.cfg.BackoffMax {
			delay = c.cfg.BackoffMax
		}
	}
}

// Run drives the connected session: read pump + write pump + heartbeat.
// Returns when any of the pumps exit (connection closed, ctx cancelled,
// hub-side bye) — the caller decides whether to reconnect.
//
// Pre-condition: Connect (or ConnectWithBackoff) must have completed
// successfully before Run is called; otherwise Run returns an error
// immediately.
func (c *Client) Run(ctx context.Context) error {
	c.connMu.Lock()
	conn := c.conn
	send := c.send
	c.connMu.Unlock()
	if conn == nil || send == nil {
		return fmt.Errorf("transport run: not connected (call Connect first)")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Close hooks done channel + underlying conn once everyone exits.
	closeSession := func() {
		c.connMu.Lock()
		if c.done != nil {
			select {
			case <-c.done:
			default:
				close(c.done)
			}
			c.done = nil
		}
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		defer cancel()
		c.runReadPump(runCtx, conn)
	}()
	go func() {
		defer wg.Done()
		defer cancel()
		c.runWritePump(runCtx, conn, send)
	}()
	go func() {
		defer wg.Done()
		hb := NewHeartbeat(c)
		hb.Run(runCtx)
	}()
	wg.Wait()
	closeSession()
	return nil
}

// Close terminates the in-flight session + stops further reconnects. Safe to
// call multiple times.
func (c *Client) Close() error {
	c.StopReconnect()
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	return nil
}

// writeEnvelope serialises payload + ships it on conn directly (used for
// hello, before the write pump exists).
func (c *Client) writeEnvelope(conn *websocket.Conn, t agentlib.MessageType, id string, payload any) error {
	raw, err := agentlib.MarshalPayload(payload)
	if err != nil {
		return err
	}
	data, err := agentlib.MarshalEnvelope(&agentlib.Envelope{
		Type:    t,
		ID:      id,
		Payload: raw,
		TS:      c.cfg.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(c.cfg.Now().Add(c.cfg.WriteTimeout))
	return conn.WriteMessage(websocket.TextMessage, data)
}

// enqueue ships a pre-built envelope through the write pump.
func (c *Client) enqueue(t agentlib.MessageType, id string, payload any) error {
	raw, err := agentlib.MarshalPayload(payload)
	if err != nil {
		return err
	}
	data, err := agentlib.MarshalEnvelope(&agentlib.Envelope{
		Type:    t,
		ID:      id,
		Payload: raw,
		TS:      c.cfg.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	c.connMu.Lock()
	send := c.send
	c.connMu.Unlock()
	if send == nil {
		return fmt.Errorf("transport: not connected")
	}
	select {
	case send <- data:
		return nil
	default:
		// Drop on back pressure rather than blocking the heartbeat.
		return fmt.Errorf("transport: send buffer full")
	}
}

// handleBye logs + classifies a hub-originated bye. If the reason is
// version_unsupported or agent_deleted the client stops reconnecting.
func (c *Client) handleBye(env *agentlib.Envelope, conn *websocket.Conn) {
	bye, _ := agentlib.UnmarshalBye(env.Payload)
	reason := ""
	if bye != nil {
		reason = bye.Reason
	}
	switch reason {
	case agentlib.ByeReasonVersionUnsupported:
		c.cfg.Logger.Warn("agent transport: hub rejected protocol version; need upgrade",
			slog.String("agent_version", c.cfg.Version),
			slog.String("reason", reason),
		)
		c.StopReconnect()
	case agentlib.ByeReasonAgentDeleted:
		c.cfg.Logger.Warn("agent transport: hub reports agent deleted; stopping",
			slog.String("reason", reason))
		c.StopReconnect()
	default:
		c.cfg.Logger.Info("agent transport: bye from hub",
			slog.String("reason", reason))
	}
	if conn != nil {
		_ = conn.Close()
	}
}

// buildWSURL appends ?token=… to the hub URL, preserving any existing path /
// query. When the user passes only the hub root (`ws://host:port` or
// `ws://host:port/`), the canonical /api/agent/ws path is auto-attached so
// the install command stays short — without this the root path hits the
// silent-mode nginx mimic and the connect fails with "bad handshake".
//
// The install script / panel hand out an http(s) hub URL (the same address the
// web UI uses), but websocket.Dialer only accepts ws/wss — so https→wss and
// http→ws are normalised here. It does NOT log the token.
func buildWSURL(hubURL, token string) (string, error) {
	u, err := url.Parse(hubURL)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api/agent/ws"
	}
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// newMsgID returns a short hex ID — uuid v4 would also be fine but pulling in
// a uuid library for a 16-byte identifier is overkill for v1.
func newMsgID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "-fallback"
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
