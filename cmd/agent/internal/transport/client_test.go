package transport

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/cmd/agent/internal/collector"
	"shiguang-vps/pkg/agentlib"
)

// silentLogger discards all output — keeps `go test` console clean.
func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// mockHub spins up an httptest.Server running a WebSocket endpoint that
// drives the agent through a scripted scenario. The test passes a callback
// to define the per-connection behaviour.
type mockHub struct {
	t        *testing.T
	srv      *httptest.Server
	wsURL    string
	upgrader websocket.Upgrader
}

func newMockHub(t *testing.T, handler func(c *websocket.Conn, hello *agentlib.HelloPayload)) *mockHub {
	t.Helper()
	m := &mockHub{
		t: t,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := m.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Read hello.
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, err := agentlib.UnmarshalEnvelope(data)
		if err != nil || env.Type != agentlib.MsgHello {
			return
		}
		hello, err := agentlib.UnmarshalHello(env.Payload)
		if err != nil {
			return
		}
		handler(conn, hello)
	}))
	m.wsURL = strings.Replace(m.srv.URL, "http://", "ws://", 1)
	return m
}

func (m *mockHub) Close() { m.srv.Close() }

// sendEnvelope is a tiny helper for the mockHub handler.
func sendEnvelope(conn *websocket.Conn, t agentlib.MessageType, id string, payload any) error {
	raw, err := agentlib.MarshalPayload(payload)
	if err != nil {
		return err
	}
	data, err := agentlib.MarshalEnvelope(&agentlib.Envelope{
		Type: t, ID: id, Payload: raw, TS: time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// newTestClient builds a Client wired to mock.wsURL with conservative
// timeouts so tests stay fast.
func newTestClient(t *testing.T, hubURL string) *Client {
	t.Helper()
	c, err := NewClient(Config{
		HubURL:            hubURL,
		Token:             "test-token",
		AgentID:           "test-agent",
		Version:           agentlib.ProtocolVersion,
		HeartbeatInterval: 100 * time.Millisecond,
		HandshakeTimeout:  2 * time.Second,
		WriteTimeout:      1 * time.Second,
		BackoffMin:        50 * time.Millisecond,
		BackoffMax:        200 * time.Millisecond,
		Aggregator:        collector.NewAggregator(collector.Config{AgentID: "test-agent", NetSampleInterval: 100 * time.Millisecond}),
		Logger:            silentLogger(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// TestConnect_HandshakeOK confirms hello → hello_ack flow + token query +
// honoured heartbeat_interval override.
func TestConnect_HandshakeOK(t *testing.T) {
	var gotToken atomic.Value
	hub := newMockHub(t, func(conn *websocket.Conn, hello *agentlib.HelloPayload) {
		if hello.AgentID != "test-agent" {
			t.Errorf("hello agent_id = %q, want test-agent", hello.AgentID)
		}
		_ = sendEnvelope(conn, agentlib.MsgHelloAck, "ack", agentlib.HelloAckPayload{
			OK: true, HeartbeatInterval: 7, HubVersion: "1.0",
		})
		// Hold the connection open briefly so the client doesn't see EOF
		// before we assert.
		_, _, _ = conn.ReadMessage()
	})
	defer hub.Close()

	// Capture token from query before the upgrade by wrapping the server URL.
	parsed, _ := url.Parse(hub.wsURL)
	gotToken.Store("")
	hub.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken.Store(r.URL.Query().Get("token"))
		conn, err := hub.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, _ := agentlib.UnmarshalEnvelope(data)
		_, _ = agentlib.UnmarshalHello(env.Payload)
		_ = sendEnvelope(conn, agentlib.MsgHelloAck, "ack", agentlib.HelloAckPayload{
			OK: true, HeartbeatInterval: 7, HubVersion: "1.0",
		})
		_, _, _ = conn.ReadMessage()
	})

	c := newTestClient(t, parsed.String())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()
	if got := gotToken.Load().(string); got != "test-token" {
		t.Errorf("token query = %q, want test-token", got)
	}
	if want := 7 * time.Second; c.HeartbeatInterval() != want {
		t.Errorf("HeartbeatInterval = %v, want %v (from hello_ack)", c.HeartbeatInterval(), want)
	}
}

// TestConnect_VersionUnsupported confirms bye{version_unsupported} during
// handshake → ErrStopReconnect + StopReconnect flag.
func TestConnect_VersionUnsupported(t *testing.T) {
	hub := newMockHub(t, func(conn *websocket.Conn, hello *agentlib.HelloPayload) {
		_ = sendEnvelope(conn, agentlib.MsgBye, "bye-x", agentlib.ByePayload{
			Reason: agentlib.ByeReasonVersionUnsupported,
		})
	})
	defer hub.Close()

	c := newTestClient(t, hub.wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := c.Connect(ctx)
	if !errors.Is(err, ErrStopReconnect) {
		t.Fatalf("Connect: err = %v, want ErrStopReconnect", err)
	}
	if !c.stopReconnect.Load() {
		t.Errorf("stopReconnect flag should be set")
	}
}

// TestConnectWithBackoff_StopsOnVersionUnsupported confirms the reconnect
// loop bails out (does NOT retry) when the hub rejects on version.
func TestConnectWithBackoff_StopsOnVersionUnsupported(t *testing.T) {
	var attempts atomic.Int32
	hub := newMockHub(t, func(conn *websocket.Conn, hello *agentlib.HelloPayload) {
		attempts.Add(1)
		_ = sendEnvelope(conn, agentlib.MsgBye, "bye-x", agentlib.ByePayload{
			Reason: agentlib.ByeReasonVersionUnsupported,
		})
	})
	defer hub.Close()

	c := newTestClient(t, hub.wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := c.ConnectWithBackoff(ctx)
	if !errors.Is(err, ErrStopReconnect) {
		t.Fatalf("ConnectWithBackoff err = %v, want ErrStopReconnect", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 (no retry on version_unsupported)", got)
	}
}

// TestConnectWithBackoff_RetriesOnFailure confirms transient failures
// trigger exponential backoff (we observe ≥ 2 attempts within the test
// window).
func TestConnectWithBackoff_RetriesOnFailure(t *testing.T) {
	// Use a server that immediately 500s on the WebSocket upgrade. This
	// avoids race conditions vs. closing a real listener.
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1)

	c := newTestClient(t, wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()

	err := c.ConnectWithBackoff(ctx)
	if err == nil {
		t.Fatalf("ConnectWithBackoff: expected ctx.Err on timeout, got nil")
	}
	if attempts.Load() < 2 {
		t.Errorf("attempts = %d, want ≥ 2 (exponential backoff should retry)", attempts.Load())
	}
}

// TestRunSendsHeartbeat confirms the heartbeat loop emits at least one
// heartbeat envelope within the test window.
func TestRunSendsHeartbeat(t *testing.T) {
	var (
		mu         sync.Mutex
		sawHb      int
		sawMetrics int
	)
	hub := newMockHub(t, func(conn *websocket.Conn, hello *agentlib.HelloPayload) {
		_ = sendEnvelope(conn, agentlib.MsgHelloAck, "ack", agentlib.HelloAckPayload{
			OK: true, HeartbeatInterval: 0, HubVersion: "1.0",
		})
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			env, err := agentlib.UnmarshalEnvelope(data)
			if err != nil {
				continue
			}
			mu.Lock()
			switch env.Type {
			case agentlib.MsgHeartbeat:
				sawHb++
			case agentlib.MsgMetrics:
				sawMetrics++
			}
			mu.Unlock()
		}
	})
	defer hub.Close()

	c := newTestClient(t, hub.wsURL)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// 3 s window: enough for the CPU collector's 1 s sample to finish + the
	// heartbeat tick to fire at least once.
	runCtx, runCancel := context.WithTimeout(ctx, 3*time.Second)
	defer runCancel()
	_ = c.Run(runCtx)
	c.Close()

	mu.Lock()
	defer mu.Unlock()
	if sawHb < 1 {
		t.Errorf("heartbeats observed = %d, want ≥ 1", sawHb)
	}
	if sawMetrics < 1 {
		t.Errorf("metrics observed = %d, want ≥ 1", sawMetrics)
	}
}

// TestNewClient_ValidationErrors covers the required-field guard.
func TestNewClient_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
	}{
		{"missing hub url", Config{Token: "t", AgentID: "a"}},
		{"missing token", Config{HubURL: "ws://x", AgentID: "a"}},
		{"missing agent id", Config{HubURL: "ws://x", Token: "t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.cfg); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestBuildWSURL covers the token-query injection helper.
func TestBuildWSURL(t *testing.T) {
	got, err := buildWSURL("wss://hub.example.com/api/agent/ws", "secret-123")
	if err != nil {
		t.Fatalf("buildWSURL: %v", err)
	}
	if !strings.Contains(got, "token=secret-123") {
		t.Errorf("expected token in url, got %q", got)
	}
}
