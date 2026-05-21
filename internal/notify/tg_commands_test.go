package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"shiguang-vps/internal/storage"
	"shiguang-vps/pkg/agentlib"
)

// fakeNodeRepo lets us preload a static node list per (userID, opts).
type fakeNodeRepo struct {
	nodes []storage.NodeRecord
	err   error
	total int64
}

func (f *fakeNodeRepo) ListByUser(ctx context.Context, userID string, opts storage.NodeListOptions) ([]storage.NodeRecord, int64, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	return f.nodes, f.total, nil
}

type fakeSubRepo struct {
	subs map[string]*storage.SubscriptionRecord
}

func (f *fakeSubRepo) GetByID(ctx context.Context, id, userID string) (*storage.SubscriptionRecord, error) {
	rec, ok := f.subs[id]
	if !ok {
		return nil, storage.ErrSubscriptionNotFound
	}
	if rec.UserID != userID {
		return nil, storage.ErrSubscriptionNotFound
	}
	return rec, nil
}

func (f *fakeSubRepo) List(ctx context.Context, userID string, opts storage.SubscriptionListOptions) ([]storage.SubscriptionRecord, int64, error) {
	out := make([]storage.SubscriptionRecord, 0, len(f.subs))
	for _, s := range f.subs {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, int64(len(out)), nil
}

type fakeAgentRepo struct {
	agents map[string]*storage.AgentRecord
}

func (f *fakeAgentRepo) GetByID(ctx context.Context, id, userID string) (*storage.AgentRecord, error) {
	rec, ok := f.agents[id]
	if !ok {
		return nil, storage.ErrAgentNotFound
	}
	if rec.UserID != userID {
		return nil, storage.ErrAgentNotFound
	}
	return rec, nil
}

type fakeTrafficRepo struct {
	summary *storage.TrafficSummary
	err     error
}

func (f *fakeTrafficRepo) GetMonthSummary(ctx context.Context, userID string, year int, month time.Month, monthlyLimit int64) (*storage.TrafficSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.summary, nil
}

type fakeChannelRepo struct {
	channels map[string]*storage.NotificationChannelRecord
	updates  []storage.NotificationChannelRecord
}

func (f *fakeChannelRepo) GetByID(ctx context.Context, id, userID string) (*storage.NotificationChannelRecord, error) {
	rec, ok := f.channels[id]
	if !ok || rec.UserID != userID {
		return nil, storage.ErrNotificationChannelNotFound
	}
	return rec, nil
}

func (f *fakeChannelRepo) Update(ctx context.Context, rec storage.NotificationChannelRecord) error {
	cur, ok := f.channels[rec.ID]
	if !ok {
		return storage.ErrNotificationChannelNotFound
	}
	cur.ConfigJSON = rec.ConfigJSON
	cur.Enabled = rec.Enabled
	f.updates = append(f.updates, rec)
	return nil
}

func (f *fakeChannelRepo) List(ctx context.Context, userID string, opts storage.NotificationChannelListOptions) ([]storage.NotificationChannelRecord, int64, error) {
	out := make([]storage.NotificationChannelRecord, 0, len(f.channels))
	for _, c := range f.channels {
		if c.UserID == userID {
			out = append(out, *c)
		}
	}
	return out, int64(len(out)), nil
}

type fakeUserRepo struct {
	users map[string]*storage.UserRecord
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id string) (*storage.UserRecord, error) {
	rec, ok := f.users[id]
	if !ok {
		return nil, storage.ErrUserNotFound
	}
	return rec, nil
}

type fakeSettingsRepo struct {
	values map[string]string
}

func (f *fakeSettingsRepo) Get(ctx context.Context, key string) (string, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return "", storage.ErrSettingNotFound
}

func (f *fakeSettingsRepo) Set(ctx context.Context, key, value string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[key] = value
	return nil
}

type fakeSyncer struct {
	called bool
	err    error
}

func (f *fakeSyncer) SyncOne(ctx context.Context, sub *storage.SubscriptionRecord) (any, error) {
	f.called = true
	return nil, f.err
}

type fakeHub struct {
	online   map[string]bool
	sent     []string
	sendErr  error
}

func (f *fakeHub) IsOnline(agentID string) bool { return f.online[agentID] }
func (f *fakeHub) SendCommand(ctx context.Context, agentID, cmdID string, p agentlib.CmdPayload) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, fmt.Sprintf("%s/%s/%s", agentID, cmdID, p.Cmd))
	return nil
}

// fixedTime returns a deterministic clock.
func fixedTime() time.Time { return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC) }

func newChatCtx(userID string) *TGChatContext {
	return &TGChatContext{ChatID: 42, UserID: userID, Locale: "zh-CN"}
}

func TestCmd_Help_ListsAllCommands(t *testing.T) {
	cfg := CommandsConfig{Now: fixedTime}
	out, _, err := cfg.cmdHelp(context.Background(), newChatCtx("u-1"), "")
	if err != nil {
		t.Fatalf("cmdHelp: %v", err)
	}
	for _, name := range []string{"/start", "/nodes", "/refresh", "/agent_restart", "/traffic", "/silent", "/help"} {
		if !strings.Contains(out, name) {
			t.Errorf("help missing %s", name)
		}
	}
}

func TestCmd_Nodes_RendersLatencyAndProtocol(t *testing.T) {
	latency := int32(34)
	cfg := CommandsConfig{
		Nodes: &fakeNodeRepo{
			nodes: []storage.NodeRecord{
				{
					ID:            "n-1",
					Protocol:      "vmess",
					Tag:           "HK-01",
					Server:        "1.2.3.4",
					Port:          443,
					LastLatencyMs: &latency,
				},
			},
			total: 1,
		},
		Now: fixedTime,
	}
	out, _, err := cfg.cmdNodes(context.Background(), newChatCtx("u-1"), "")
	if err != nil {
		t.Fatalf("cmdNodes: %v", err)
	}
	if !strings.Contains(out, "HK") {
		t.Errorf("output missing node tag: %s", out)
	}
	if !strings.Contains(out, "vmess") {
		t.Errorf("output missing protocol: %s", out)
	}
	if !strings.Contains(out, "34ms") {
		t.Errorf("output missing latency: %s", out)
	}
}

func TestCmd_Nodes_EmptyList(t *testing.T) {
	cfg := CommandsConfig{Nodes: &fakeNodeRepo{}, Now: fixedTime}
	out, _, err := cfg.cmdNodes(context.Background(), newChatCtx("u-1"), "")
	if err != nil {
		t.Fatalf("cmdNodes: %v", err)
	}
	if !strings.Contains(out, "No nodes") {
		t.Errorf("empty reply unexpected: %s", out)
	}
}

func TestCmd_Refresh_InvokesSyncer(t *testing.T) {
	syncer := &fakeSyncer{}
	cfg := CommandsConfig{
		Subscriptions: &fakeSubRepo{
			subs: map[string]*storage.SubscriptionRecord{
				"sub-1": {ID: "sub-1", UserID: "u-1", Name: "demo"},
			},
		},
		Syncer: syncer,
		Now:    fixedTime,
	}
	out, _, err := cfg.cmdRefresh(context.Background(), newChatCtx("u-1"), "sub-1")
	if err != nil {
		t.Fatalf("cmdRefresh: %v", err)
	}
	if !syncer.called {
		t.Fatal("syncer not called")
	}
	if !strings.Contains(out, "Refresh queued") {
		t.Errorf("output: %s", out)
	}
}

func TestCmd_Refresh_NotFound(t *testing.T) {
	cfg := CommandsConfig{
		Subscriptions: &fakeSubRepo{subs: map[string]*storage.SubscriptionRecord{}},
		Syncer:        &fakeSyncer{},
		Now:           fixedTime,
	}
	out, _, err := cfg.cmdRefresh(context.Background(), newChatCtx("u-1"), "missing")
	if err != nil {
		t.Fatalf("cmdRefresh: %v", err)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected not-found message, got: %s", out)
	}
}

func TestCmd_AgentRestart_AdminOnly(t *testing.T) {
	cfg := CommandsConfig{
		AdminCheck: func(ctx context.Context, userID string) bool { return false },
		Now:        fixedTime,
	}
	out, _, err := cfg.cmdAgentRestart(context.Background(), newChatCtx("u-1"), "a-1")
	if err != nil {
		t.Fatalf("cmdAgentRestart: %v", err)
	}
	if !strings.Contains(out, "Admin only") {
		t.Errorf("non-admin should be blocked, got: %s", out)
	}
}

func TestCmd_AgentRestart_SendsRestart(t *testing.T) {
	hub := &fakeHub{online: map[string]bool{"a-1": true}}
	cfg := CommandsConfig{
		AdminCheck: func(ctx context.Context, userID string) bool { return true },
		Agents: &fakeAgentRepo{
			agents: map[string]*storage.AgentRecord{
				"a-1": {ID: "a-1", UserID: "u-1", Name: "edge-1"},
			},
		},
		Hub: hub,
		Now: fixedTime,
	}
	out, _, err := cfg.cmdAgentRestart(context.Background(), newChatCtx("u-1"), "a-1")
	if err != nil {
		t.Fatalf("cmdAgentRestart: %v", err)
	}
	if len(hub.sent) != 1 {
		t.Fatalf("expected 1 SendCommand, got %d", len(hub.sent))
	}
	if !strings.Contains(hub.sent[0], "restart") {
		t.Errorf("command not restart: %s", hub.sent[0])
	}
	if !strings.Contains(out, "Restart dispatched") {
		t.Errorf("output: %s", out)
	}
}

func TestCmd_AgentRestart_OfflineAgent(t *testing.T) {
	hub := &fakeHub{online: map[string]bool{}}
	cfg := CommandsConfig{
		AdminCheck: func(ctx context.Context, userID string) bool { return true },
		Agents: &fakeAgentRepo{
			agents: map[string]*storage.AgentRecord{
				"a-1": {ID: "a-1", UserID: "u-1", Name: "edge-1"},
			},
		},
		Hub: hub,
		Now: fixedTime,
	}
	out, _, err := cfg.cmdAgentRestart(context.Background(), newChatCtx("u-1"), "a-1")
	if err != nil {
		t.Fatalf("cmdAgentRestart: %v", err)
	}
	if !strings.Contains(out, "offline") {
		t.Errorf("expected offline message, got: %s", out)
	}
}

func TestCmd_Traffic_ShowsUsage(t *testing.T) {
	cfg := CommandsConfig{
		Traffic: &fakeTrafficRepo{
			summary: &storage.TrafficSummary{
				UserID:      "u-1",
				PeriodStart: "2026-05-01",
				PeriodEnd:   "2026-05-31",
				TotalLimit:  1024 * 1024 * 1024 * 100, // 100 GiB
				TotalUsed:   1024 * 1024 * 1024 * 25,  // 25 GiB
				TotalIn:     1024 * 1024 * 1024 * 10,
				TotalOut:    1024 * 1024 * 1024 * 15,
				Agents: []storage.TrafficAgentBreakdown{
					{AgentID: "a-1", TotalUsed: 1024 * 1024 * 1024 * 20},
					{AgentID: "a-2", TotalUsed: 1024 * 1024 * 1024 * 5},
				},
			},
		},
		Now: fixedTime,
	}
	out, _, err := cfg.cmdTraffic(context.Background(), newChatCtx("u-1"), "")
	if err != nil {
		t.Fatalf("cmdTraffic: %v", err)
	}
	if !strings.Contains(out, "25\\.00 GiB") {
		t.Errorf("expected 25 GiB used, got: %s", out)
	}
	if !strings.Contains(out, "Per agent") {
		t.Errorf("expected per-agent breakdown, got: %s", out)
	}
}

func TestCmd_Silent_TogglesOn(t *testing.T) {
	settings := &fakeSettingsRepo{values: map[string]string{}}
	cfg := CommandsConfig{
		AdminCheck: func(ctx context.Context, userID string) bool { return true },
		Settings:   settings,
		Now:        fixedTime,
	}
	out, _, err := cfg.cmdSilent(context.Background(), newChatCtx("u-1"), "on")
	if err != nil {
		t.Fatalf("cmdSilent: %v", err)
	}
	if settings.values[storage.SettingSilentModeEnabled] != "1" {
		t.Errorf("silent mode not enabled: %v", settings.values)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("output: %s", out)
	}
}

func TestCmd_Silent_NonAdminBlocked(t *testing.T) {
	settings := &fakeSettingsRepo{values: map[string]string{}}
	cfg := CommandsConfig{
		AdminCheck: func(ctx context.Context, userID string) bool { return false },
		Settings:   settings,
		Now:        fixedTime,
	}
	out, _, err := cfg.cmdSilent(context.Background(), newChatCtx("u-1"), "on")
	if err != nil {
		t.Fatalf("cmdSilent: %v", err)
	}
	if _, exists := settings.values[storage.SettingSilentModeEnabled]; exists {
		t.Errorf("non-admin should not toggle silent mode: %v", settings.values)
	}
	if !strings.Contains(out, "Admin only") {
		t.Errorf("output: %s", out)
	}
}

func TestCmd_Start_BindsChatToChannel(t *testing.T) {
	channel := &storage.NotificationChannelRecord{
		ID:         "c-1",
		UserID:     "u-1",
		Kind:       "telegram",
		Name:       "primary",
		ConfigJSON: `{"bot_token":"x","chat_id":""}`,
	}
	chans := &fakeChannelRepo{
		channels: map[string]*storage.NotificationChannelRecord{"c-1": channel},
	}
	users := &fakeUserRepo{
		users: map[string]*storage.UserRecord{
			"u-1": {ID: "u-1", Username: "alice"},
		},
	}
	cfg := CommandsConfig{
		Channels: chans,
		Users:    users,
		Now:      fixedTime,
	}
	out, _, err := cfg.cmdStart(context.Background(), newChatCtx(""), "u-1:c-1")
	if err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	if len(chans.updates) != 1 {
		t.Fatalf("expected 1 channel update, got %d", len(chans.updates))
	}
	if !strings.Contains(chans.updates[0].ConfigJSON, `"42"`) {
		t.Errorf("chat_id 42 not persisted: %s", chans.updates[0].ConfigJSON)
	}
	if !strings.Contains(out, "bound") {
		t.Errorf("reply text: %s", out)
	}
}

func TestCmd_Start_InvalidToken(t *testing.T) {
	cfg := CommandsConfig{
		Channels: &fakeChannelRepo{channels: map[string]*storage.NotificationChannelRecord{}},
		Users:    &fakeUserRepo{users: map[string]*storage.UserRecord{}},
		Now:      fixedTime,
	}
	out, _, err := cfg.cmdStart(context.Background(), newChatCtx(""), "nonsense")
	if err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	if !strings.Contains(out, "Invalid") {
		t.Errorf("expected invalid-token reply, got: %s", out)
	}
}

func TestCmd_Start_DuplicateChatRejected(t *testing.T) {
	channel := &storage.NotificationChannelRecord{
		ID:         "c-1",
		UserID:     "u-1",
		Kind:       "telegram",
		Name:       "primary",
		ConfigJSON: `{"bot_token":"x","chat_ids":["42"]}`,
	}
	chans := &fakeChannelRepo{
		channels: map[string]*storage.NotificationChannelRecord{"c-1": channel},
	}
	users := &fakeUserRepo{
		users: map[string]*storage.UserRecord{"u-1": {ID: "u-1"}},
	}
	cfg := CommandsConfig{
		Channels: chans,
		Users:    users,
		Now:      fixedTime,
	}
	out, _, err := cfg.cmdStart(context.Background(), newChatCtx(""), "u-1:c-1")
	if err != nil {
		t.Fatalf("cmdStart: %v", err)
	}
	if len(chans.updates) != 0 {
		t.Fatalf("duplicate chat should not trigger an update, got %d", len(chans.updates))
	}
	if !strings.Contains(out, "already bound") {
		t.Errorf("output: %s", out)
	}
}

func TestRegisterCommands_AllSeven(t *testing.T) {
	router := NewCommandRouter()
	RegisterCommands(router, CommandsConfig{Now: fixedTime})
	for _, name := range []string{"start", "help", "nodes", "refresh", "agent_restart", "traffic", "silent"} {
		if _, ok := router.lookupCommand("/" + name); !ok {
			t.Errorf("command %s not registered", name)
		}
	}
}

// Smoke test that the channel-config helpers round-trip.
func TestChannelConfig_AppendChatID(t *testing.T) {
	raw := `{"bot_token":"abc"}`
	cfg, err := decodeChannelConfig(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !appendChatID(cfg, 1234) {
		t.Fatal("first append should succeed")
	}
	if appendChatID(cfg, 1234) {
		t.Fatal("duplicate append should be rejected")
	}
	encoded, err := encodeChannelConfig(cfg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(encoded, `"1234"`) {
		t.Errorf("encoded missing chat_id: %s", encoded)
	}
}

// Sanity test for the syncer error path.
func TestCmd_Refresh_SyncerError(t *testing.T) {
	cfg := CommandsConfig{
		Subscriptions: &fakeSubRepo{
			subs: map[string]*storage.SubscriptionRecord{
				"sub-1": {ID: "sub-1", UserID: "u-1", Name: "demo"},
			},
		},
		Syncer: &fakeSyncer{err: errors.New("boom")},
		Now:    fixedTime,
	}
	out, _, err := cfg.cmdRefresh(context.Background(), newChatCtx("u-1"), "sub-1")
	if err != nil {
		t.Fatalf("cmdRefresh: %v", err)
	}
	if !strings.Contains(out, "Refresh failed") {
		t.Errorf("expected refresh-failed message, got: %s", out)
	}
}
