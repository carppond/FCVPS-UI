package handler

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"shiguang-vps/internal/agent"
	"shiguang-vps/internal/config"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/util"
	"shiguang-vps/pkg/agentlib"
)

// wsTestStack spins up a real httptest server backed by the agent WS handler.
// Tests interact with it through a gorilla/websocket dialer.
type wsTestStack struct {
	t          *testing.T
	server     *httptest.Server
	repo       *storage.AgentRepo
	recordRepo *storage.AgentRecordRepo
	hub        *agent.Hub
	wsURL      string
}

func newWSTestStack(t *testing.T) *wsTestStack {
	t.Helper()
	dir := t.TempDir()
	db, err := storage.Open(config.DatabaseConfig{
		DataDir: dir, Filename: "test.db", BusyTimeoutMs: 5000,
		MaxOpenWrite: 1, MaxOpenRead: 2,
	})
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(context.Background()); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	users := storage.NewUserRepo(db, time.Now)
	if _, err := users.Create(context.Background(), storage.UserRecord{
		ID: "u1", Username: "u1", PasswordHash: "h", Role: "user", IsActive: true,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	repo := storage.NewAgentRepo(db, time.Now)
	recordRepo := storage.NewAgentRecordRepo(db)
	hub := agent.NewHub(agent.HubConfig{
		AgentRepo:         repo,
		RecordRepo:        recordRepo,
		HeartbeatInterval: 5 * time.Second, // shorter for tests
		IdleTimeout:       15 * time.Second,
	})
	wsHandler := NewAgentWSHandler(hub, repo, recordRepo, nil)
	server := httptest.NewServer(wsHandler)
	t.Cleanup(func() { server.Close() })
	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	return &wsTestStack{
		t: t, server: server, repo: repo, recordRepo: recordRepo,
		hub: hub, wsURL: wsURL,
	}
}

// createAgent seeds an agent + returns the plaintext token.
func (s *wsTestStack) createAgent(name string) (string, string) {
	s.t.Helper()
	token := util.RandomHex32()
	rec, err := s.repo.Create(context.Background(), storage.AgentRecord{
		ID:        "a-" + name,
		UserID:    "u1",
		Name:      name,
		TokenHash: util.SHA256Hex(token),
		Kind:      "native",
	})
	if err != nil {
		s.t.Fatalf("create agent: %v", err)
	}
	return rec.ID, token
}

// dial opens a WS connection with the given token via ?token=<...>.
func (s *wsTestStack) dial(token string) (*websocket.Conn, error) {
	u, _ := url.Parse(s.wsURL)
	q := u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	return conn, err
}

func TestAgentWSHandshakeAndHeartbeat(t *testing.T) {
	s := newWSTestStack(t)
	agentID, token := s.createAgent("vps1")

	conn, err := s.dial(token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send hello.
	hello := agent.HelloPayload{
		AgentID: agentID, Token: token,
		OS: "linux", Arch: "amd64", Version: "1.0.0",
		Kind: "native", Capabilities: []string{"metrics"},
	}
	if err := writeEnv(conn, agent.MsgHello, "hello-1", hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	// Read hello_ack.
	env, err := readEnv(conn, 2*time.Second)
	if err != nil {
		t.Fatalf("read hello_ack: %v", err)
	}
	if env.Type != agent.MsgHelloAck {
		t.Fatalf("expected hello_ack, got %s", env.Type)
	}
	ack, err := agentlib.UnmarshalHelloAck(env.Payload)
	if err != nil || !ack.OK {
		t.Fatalf("hello_ack payload: %v %+v", err, ack)
	}

	// Hub registered the agent.
	waitForCond(t, time.Second, func() bool {
		return s.hub.IsOnline(agentID)
	})

	// Send a metrics frame and confirm it lands in the DB.
	metrics := agent.MetricsPayload{
		AgentID: agentID, CPUPercent: 12.5,
		MemUsed: 1024, MemTotal: 4096, Uptime: 100,
	}
	if err := writeEnv(conn, agent.MsgMetrics, "m-1", metrics); err != nil {
		t.Fatalf("write metrics: %v", err)
	}
	waitForCond(t, 2*time.Second, func() bool {
		got, _ := s.recordRepo.ListRecent(context.Background(), agentID, time.Time{}, 10)
		return len(got) >= 1
	})
	got, _ := s.recordRepo.ListRecent(context.Background(), agentID, time.Time{}, 10)
	if got[0].CPUPercent != 12.5 || got[0].MemUsed != 1024 {
		t.Fatalf("metrics not persisted correctly: %+v", got[0])
	}

	// Latest metrics cache reflects the live value.
	cli := s.hub.Client(agentID)
	if cli == nil || cli.LatestMetrics() == nil || cli.LatestMetrics().CPUPercent != 12.5 {
		t.Fatalf("cached metrics wrong")
	}
}

func TestAgentWSUnknownTokenReturns404(t *testing.T) {
	s := newWSTestStack(t)
	_, err := s.dial("not-a-real-token")
	if err == nil {
		t.Fatalf("expected dial to fail with bad token")
	}
}

func TestAgentWSIncompatibleVersionRejected(t *testing.T) {
	s := newWSTestStack(t)
	agentID, token := s.createAgent("vps1")
	conn, err := s.dial(token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	hello := agent.HelloPayload{
		AgentID: agentID, Token: token,
		OS: "linux", Arch: "amd64", Version: "2.0.0", // wrong major
		Kind: "native",
	}
	if err := writeEnv(conn, agent.MsgHello, "hello-1", hello); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	env, err := readEnv(conn, 2*time.Second)
	if err != nil {
		t.Fatalf("read bye: %v", err)
	}
	if env.Type != agent.MsgBye {
		t.Fatalf("expected bye, got %s", env.Type)
	}
	bye, _ := agentlib.UnmarshalBye(env.Payload)
	if bye == nil || bye.Reason != agent.ByeReasonVersionUnsupported {
		t.Fatalf("expected version_unsupported reason, got %+v", bye)
	}
	if s.hub.IsOnline(agentID) {
		t.Fatalf("agent must not be registered after version mismatch")
	}
}

func TestAgentWSAgentIDMismatchClosesSilently(t *testing.T) {
	s := newWSTestStack(t)
	_, token := s.createAgent("vps1")
	conn, err := s.dial(token)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	hello := agent.HelloPayload{
		AgentID: "wrong-id", Token: token,
		Version: "1.0.0", Kind: "native",
	}
	_ = writeEnv(conn, agent.MsgHello, "hello-1", hello)
	// Server closes without a bye for ID mismatch (potential token sharing).
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("expected read failure (server closed)")
	}
}

// --- helpers --------------------------------------------------------------

func writeEnv(conn *websocket.Conn, msgType agent.MessageType, id string, payload any) error {
	raw, err := agentlib.MarshalPayload(payload)
	if err != nil {
		return err
	}
	data, err := agentlib.MarshalEnvelope(&agent.Envelope{
		Type: msgType, ID: id, Payload: raw,
		TS: time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func readEnv(conn *websocket.Conn, timeout time.Duration) (*agent.Envelope, error) {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return agentlib.UnmarshalEnvelope(data)
}

func waitForCond(t *testing.T, d time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not satisfied within %v", d)
}
