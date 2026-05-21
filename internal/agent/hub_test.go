package agent_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/storage"
	"shiguang-vps/pkg/agentlib"
)

// fakeConn is a minimal Connection used to drive Client without spinning up an
// HTTP server. It records outbound writes + lets the test push inbound bytes.
type fakeConn struct {
	mu       sync.Mutex
	outbound [][]byte
	inbound  chan []byte
	closed   atomic.Bool
}

func newFakeConn() *fakeConn {
	return &fakeConn{inbound: make(chan []byte, 16)}
}

func (c *fakeConn) ReadMessage() (int, []byte, error) {
	data, ok := <-c.inbound
	if !ok {
		return 0, nil, errors.New("conn closed")
	}
	return websocket.TextMessage, data, nil
}

func (c *fakeConn) WriteMessage(_ int, data []byte) error {
	if c.closed.Load() {
		return errors.New("write on closed conn")
	}
	c.mu.Lock()
	c.outbound = append(c.outbound, append([]byte(nil), data...))
	c.mu.Unlock()
	return nil
}

func (c *fakeConn) WriteControl(int, []byte, time.Time) error { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error            { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error           { return nil }
func (c *fakeConn) SetReadLimit(int64)                         {}
func (c *fakeConn) SetPongHandler(func(string) error)          {}
func (c *fakeConn) Close() error {
	if !c.closed.Swap(true) {
		close(c.inbound)
	}
	return nil
}

// sentEnvelopes returns the parsed envelopes the client wrote.
func (c *fakeConn) sentEnvelopes(t *testing.T) []*agent.Envelope {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*agent.Envelope, 0, len(c.outbound))
	for _, raw := range c.outbound {
		env, err := agentlib.UnmarshalEnvelope(raw)
		if err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		out = append(out, env)
	}
	return out
}

// pushEnvelope encodes + delivers an envelope as inbound.
func (c *fakeConn) pushEnvelope(t *testing.T, env *agent.Envelope) {
	t.Helper()
	data, err := agentlib.MarshalEnvelope(env)
	if err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
	c.inbound <- data
}

// recordingBus captures published events for assertions.
type recordingBus struct {
	mu       sync.Mutex
	statuses []string
	metrics  []*agent.MetricsPayload
}

func (b *recordingBus) PublishAgentStatus(_, status string) {
	b.mu.Lock()
	b.statuses = append(b.statuses, status)
	b.mu.Unlock()
}
func (b *recordingBus) PublishAgentMetrics(_ string, m *agent.MetricsPayload) {
	b.mu.Lock()
	b.metrics = append(b.metrics, m)
	b.mu.Unlock()
}

func (b *recordingBus) snapshot() (statuses []string, metrics []*agent.MetricsPayload) {
	b.mu.Lock()
	statuses = append([]string(nil), b.statuses...)
	metrics = append([]*agent.MetricsPayload(nil), b.metrics...)
	b.mu.Unlock()
	return
}

func TestHubRegisterAndSnapshot(t *testing.T) {
	bus := &recordingBus{}
	hub := agent.NewHub(agent.HubConfig{EventBus: bus})
	conn := newFakeConn()
	cli := agent.NewClient(agent.ClientConfig{
		Agent:             storage.AgentRecord{ID: "a1", Name: "n", UserID: "u1"},
		Conn:              conn,
		Hub:               hub,
		EventBus:          bus,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cli)
	if !hub.IsOnline("a1") {
		t.Fatalf("expected a1 online after Register")
	}
	if got := hub.Stats(); got.Online != 1 {
		t.Fatalf("stats online = %d", got.Online)
	}
	statuses, _ := bus.snapshot()
	if len(statuses) != 1 || statuses[0] != "online" {
		t.Fatalf("expected online event, got %v", statuses)
	}
	snap := hub.Snapshot()
	if len(snap) != 1 || snap[0].AgentID != "a1" || !snap[0].Online {
		t.Fatalf("snapshot mismatch: %+v", snap)
	}
}

func TestHubUnregisterClosesClient(t *testing.T) {
	hub := agent.NewHub(agent.HubConfig{})
	conn := newFakeConn()
	cli := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: conn, Hub: hub,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cli)
	hub.Unregister("a1", agent.ByeReasonTokenRotated)
	if hub.IsOnline("a1") {
		t.Fatalf("expected a1 offline after Unregister")
	}
	if !conn.closed.Load() {
		t.Fatalf("expected conn closed")
	}
	// A bye envelope must have been emitted to the client.
	sent := conn.sentEnvelopes(t)
	if len(sent) == 0 || sent[len(sent)-1].Type != agent.MsgBye {
		t.Fatalf("expected bye envelope, got %+v", sent)
	}
}

func TestHubSendCommandRoundtrip(t *testing.T) {
	hub := agent.NewHub(agent.HubConfig{})
	conn := newFakeConn()
	cli := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: conn, Hub: hub,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cli)
	// Pump writePump messages by starting Run in a goroutine. We close conn
	// at the end to release readPump.
	done := make(chan struct{})
	go func() { cli.Run(); close(done) }()
	if err := hub.SendCommand(context.Background(), "a1", "cmd-1",
		agent.CmdPayload{Cmd: agent.CmdRefreshSubscription}); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	// Allow writePump to flush.
	waitFor(t, time.Second, func() bool {
		conn.mu.Lock()
		defer conn.mu.Unlock()
		return len(conn.outbound) > 0
	})
	sent := conn.sentEnvelopes(t)
	if len(sent) < 1 {
		t.Fatalf("expected at least one outbound envelope")
	}
	if sent[0].Type != agent.MsgCmd || sent[0].ID != "cmd-1" {
		t.Fatalf("unexpected envelope: %+v", sent[0])
	}
	cli.Close()
	<-done
}

func TestHubSendCommandOffline(t *testing.T) {
	hub := agent.NewHub(agent.HubConfig{})
	err := hub.SendCommand(context.Background(), "missing", "id",
		agent.CmdPayload{Cmd: agent.CmdRefreshSubscription})
	if !errors.Is(err, agent.ErrAgentOffline) {
		t.Fatalf("expected ErrAgentOffline, got %v", err)
	}
}

func TestHubRegisterReplacesPreviousConnection(t *testing.T) {
	hub := agent.NewHub(agent.HubConfig{})
	connA := newFakeConn()
	cliA := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: connA, Hub: hub,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cliA)

	connB := newFakeConn()
	cliB := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: connB, Hub: hub,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cliB)

	// The old connection must have been closed via SendBye.
	if !connA.closed.Load() {
		t.Fatalf("expected old connA closed when new client registers")
	}
	// The new one is still alive.
	if connB.closed.Load() {
		t.Fatalf("did not expect connB closed")
	}
}

func TestHubCloseBroadcastsBye(t *testing.T) {
	hub := agent.NewHub(agent.HubConfig{})
	conn := newFakeConn()
	cli := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: conn, Hub: hub,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cli)
	hub.Close()
	if hub.IsOnline("a1") {
		t.Fatalf("expected agent removed after Close")
	}
	if !conn.closed.Load() {
		t.Fatalf("expected conn closed by hub.Close")
	}
}

func TestClientHandleMetricsCachesAndPublishes(t *testing.T) {
	bus := &recordingBus{}
	hub := agent.NewHub(agent.HubConfig{EventBus: bus})
	conn := newFakeConn()
	cli := agent.NewClient(agent.ClientConfig{
		Agent: storage.AgentRecord{ID: "a1"}, Conn: conn, Hub: hub,
		EventBus:          bus,
		HeartbeatInterval: time.Second,
	})
	hub.Register(cli)
	done := make(chan struct{})
	go func() { cli.Run(); close(done) }()

	conn.pushEnvelope(t, &agent.Envelope{
		Type: agent.MsgMetrics, ID: "m-1",
		Payload: mustMarshal(t, agent.MetricsPayload{
			AgentID: "a1", CPUPercent: 42.5, Uptime: 100,
		}),
	})

	waitFor(t, time.Second, func() bool {
		_, m := bus.snapshot()
		return len(m) == 1
	})
	if cached := cli.LatestMetrics(); cached == nil || cached.CPUPercent != 42.5 {
		t.Fatalf("metrics cache wrong: %+v", cached)
	}
	cli.Close()
	<-done
}

func TestVersionCompatibility(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"1.0.0", true},
		{"1.5", true},
		{"1", true},
		{"2.0.0", false},
		{"", false},
		{"abc", false},
		{"0.9", false},
	}
	for _, c := range cases {
		if got := agent.IsVersionCompatible(c.v); got != c.want {
			t.Fatalf("IsVersionCompatible(%q) = %v, want %v", c.v, got, c.want)
		}
	}
}

// --- helpers --------------------------------------------------------------

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := agentlib.MarshalPayload(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func waitFor(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", d)
}
