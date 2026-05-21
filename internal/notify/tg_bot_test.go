package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// fakeTGClient captures outbound Bot API calls so tests can assert on the
// JSON payloads without standing up an httptest server. Every Do returns a
// canned 200 / `{"ok":true}` body.
type fakeTGClient struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
}

func (f *fakeTGClient) Do(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(req.Body)
	f.requests = append(f.requests, req)
	f.bodies = append(f.bodies, body)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
		Header:     make(http.Header),
	}, nil
}

func (f *fakeTGClient) lastBody(t *testing.T) map[string]any {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		t.Fatalf("no requests recorded")
	}
	var out map[string]any
	if err := json.Unmarshal(f.bodies[len(f.bodies)-1], &out); err != nil {
		t.Fatalf("decode last body: %v", err)
	}
	return out
}

func (f *fakeTGClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

// allowAll is a whitelist resolver that maps every chat_id to a synthetic
// user — used by tests that exercise the dispatch path rather than the
// whitelist itself.
func allowAll(userID, locale string) WhitelistResolver {
	return func(ctx context.Context, chatID int64) (string, string, bool) {
		return userID, locale, true
	}
}

// denyAll is the inverse — every chat is rejected.
var denyAll WhitelistResolver = func(ctx context.Context, chatID int64) (string, string, bool) {
	return "", "", false
}

// newTestBot wires a bot with a fake HTTP client + given resolver. The
// router starts empty; callers register the commands they need.
func newTestBot(t *testing.T, resolver WhitelistResolver) (*Bot, *fakeTGClient, *CommandRouter) {
	t.Helper()
	fc := &fakeTGClient{}
	router := NewCommandRouter()
	bot, err := NewBot(BotConfig{
		BotToken:  "test-token",
		APIBase:   "http://api.telegram.local",
		Client:    fc,
		Router:    router,
		Whitelist: resolver,
	})
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}
	return bot, fc, router
}

func TestBot_DispatchesRegisteredCommand(t *testing.T) {
	bot, fc, router := newTestBot(t, allowAll("user-1", "zh-CN"))
	called := false
	router.RegisterCommand("nodes", func(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
		called = true
		if chat.UserID != "user-1" {
			t.Errorf("UserID = %q, want user-1", chat.UserID)
		}
		if args != "sub-42" {
			t.Errorf("args = %q, want sub-42", args)
		}
		return "ok", nil, nil
	})
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		UpdateID: 1,
		Message: &TGMessage{
			MessageID: 10,
			Text:      "/nodes sub-42",
			Chat:      TGChat{ID: 100, Type: "private"},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !called {
		t.Fatal("command handler not invoked")
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected 1 outbound API call, got %d", fc.callCount())
	}
	body := fc.lastBody(t)
	if body["text"] != "ok" {
		t.Errorf("outbound text = %v, want ok", body["text"])
	}
	if body["chat_id"].(float64) != 100 {
		t.Errorf("outbound chat_id = %v, want 100", body["chat_id"])
	}
}

func TestBot_DropsUnauthorisedChat(t *testing.T) {
	bot, fc, router := newTestBot(t, denyAll)
	router.RegisterCommand("nodes", func(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
		t.Fatal("handler should not be invoked when whitelist rejects the chat")
		return "", nil, nil
	})
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		Message: &TGMessage{
			Text: "/nodes",
			Chat: TGChat{ID: 999},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fc.callCount() != 0 {
		t.Fatalf("non-whitelisted chat should produce no API call, got %d", fc.callCount())
	}
}

func TestBot_StartIsAllowedFromUnknownChat(t *testing.T) {
	bot, fc, router := newTestBot(t, denyAll)
	got := false
	router.RegisterCommand("start", func(ctx context.Context, chat *TGChatContext, args string) (string, *TGInlineKeyboard, error) {
		got = true
		if chat.ChatID != 7 {
			t.Errorf("ChatID = %d, want 7", chat.ChatID)
		}
		return "welcome", nil, nil
	})
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		Message: &TGMessage{
			Text: "/start abc",
			Chat: TGChat{ID: 7},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if !got {
		t.Fatal("/start should run even when whitelist rejects the chat")
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected 1 outbound API call, got %d", fc.callCount())
	}
}

func TestBot_UnknownCommandRepliesWithError(t *testing.T) {
	bot, fc, _ := newTestBot(t, allowAll("user-1", "zh-CN"))
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		Message: &TGMessage{
			Text: "/whatever",
			Chat: TGChat{ID: 1},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fc.callCount() != 1 {
		t.Fatalf("unknown command should still ack the user, got %d outbound calls", fc.callCount())
	}
	body := fc.lastBody(t)
	if !strings.Contains(body["text"].(string), "Unknown") {
		t.Errorf("unexpected reply text: %v", body["text"])
	}
}

func TestBot_CallbackRoutesByAction(t *testing.T) {
	bot, fc, router := newTestBot(t, allowAll("user-1", "zh-CN"))
	var gotEvent, gotResource, gotAction string
	router.RegisterCallback("retry", func(ctx context.Context, chat *TGChatContext, eventType, resourceID, action string) (string, error) {
		gotEvent = eventType
		gotResource = resourceID
		gotAction = action
		return "done", nil
	})
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		CallbackQuery: &TGCallbackQ{
			ID:   "cb-1",
			Data: "node_offline:n-42:retry",
			From: TGUser{ID: 1},
			Message: &TGMessage{
				Chat: TGChat{ID: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if gotEvent != "node_offline" || gotResource != "n-42" || gotAction != "retry" {
		t.Fatalf("callback routed wrong: event=%q resource=%q action=%q", gotEvent, gotResource, gotAction)
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected 1 answerCallbackQuery, got %d", fc.callCount())
	}
	body := fc.lastBody(t)
	if body["callback_query_id"] != "cb-1" {
		t.Errorf("callback_query_id = %v", body["callback_query_id"])
	}
}

func TestBot_CallbackOnUnknownActionIsAcked(t *testing.T) {
	bot, fc, _ := newTestBot(t, allowAll("user-1", "zh-CN"))
	err := bot.HandleUpdate(context.Background(), &TGUpdate{
		CallbackQuery: &TGCallbackQ{
			ID:   "cb-99",
			Data: "x:y:unknown",
			From: TGUser{ID: 1},
			Message: &TGMessage{
				Chat: TGChat{ID: 1},
			},
		},
	})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected answerCallbackQuery, got %d", fc.callCount())
	}
	body := fc.lastBody(t)
	if !strings.Contains(body["text"].(string), "unknown") {
		t.Errorf("answer text = %v", body["text"])
	}
}

func TestBot_BuildAlertKeyboardThreeButtons(t *testing.T) {
	kb := BuildAlertKeyboard("node_offline", "n-1")
	if kb == nil || len(kb.Buttons) != 1 {
		t.Fatalf("expected 1 row of buttons, got %+v", kb)
	}
	if len(kb.Buttons[0]) != 3 {
		t.Fatalf("expected 3 buttons, got %d", len(kb.Buttons[0]))
	}
	for _, b := range kb.Buttons[0] {
		if !strings.HasPrefix(b.CallbackData, "node_offline:n-1:") {
			t.Errorf("callback_data not prefixed: %q", b.CallbackData)
		}
	}
}

func TestBot_EscapeMarkdownV2(t *testing.T) {
	cases := map[string]string{
		"hello":       "hello",
		"a.b":         "a\\.b",
		"plain_text":  "plain\\_text",
		"!!!":         "\\!\\!\\!",
		"":            "",
		"[link](url)": "\\[link\\]\\(url\\)",
	}
	for in, want := range cases {
		if got := tgEscapeMarkdownV2(in); got != want {
			t.Errorf("escape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBot_NoMessageNoCallback_IsNoop(t *testing.T) {
	bot, fc, _ := newTestBot(t, allowAll("user-1", "zh-CN"))
	err := bot.HandleUpdate(context.Background(), &TGUpdate{UpdateID: 1})
	if err != nil {
		t.Fatalf("HandleUpdate: %v", err)
	}
	if fc.callCount() != 0 {
		t.Fatalf("empty update should not trigger any API call, got %d", fc.callCount())
	}
}

func TestBot_NewBotRejectsMissingDeps(t *testing.T) {
	if _, err := NewBot(BotConfig{}); err == nil {
		t.Fatal("expected error for missing router")
	}
	if _, err := NewBot(BotConfig{Router: NewCommandRouter()}); err == nil {
		t.Fatal("expected error for missing whitelist")
	}
}

func TestBot_SetWebhookSendsURL(t *testing.T) {
	bot, fc, _ := newTestBot(t, allowAll("user-1", "zh-CN"))
	if err := bot.SetWebhook(context.Background(), "https://example/hook"); err != nil {
		t.Fatalf("SetWebhook: %v", err)
	}
	if fc.callCount() != 1 {
		t.Fatalf("expected 1 outbound call, got %d", fc.callCount())
	}
	body := fc.lastBody(t)
	if body["url"] != "https://example/hook" {
		t.Errorf("url = %v", body["url"])
	}
}
