package handler

import "testing"

func TestRedactChannelConfig(t *testing.T) {
	// telegram: bot_token redacted, chat_id kept.
	out := redactChannelConfig("telegram", map[string]any{
		"bot_token": "123:secret", "chat_id": "42",
	}).(map[string]any)
	if out["bot_token"] != redactedSecret {
		t.Errorf("bot_token not redacted: %v", out["bot_token"])
	}
	if out["chat_id"] != "42" {
		t.Errorf("chat_id should be kept: %v", out["chat_id"])
	}

	// empty secret stays empty (not redacted → UI shows "not set").
	out2 := redactChannelConfig("bark", map[string]any{"device_key": ""}).(map[string]any)
	if out2["device_key"] != "" {
		t.Errorf("empty secret should not be redacted: %v", out2["device_key"])
	}

	// webhook headers: values redacted.
	out3 := redactChannelConfig("webhook", map[string]any{
		"url":     "https://h/x?token=abc",
		"headers": map[string]any{"Authorization": "Bearer t", "X-Env": "prod"},
	}).(map[string]any)
	if out3["url"] != redactedSecret {
		t.Errorf("webhook url not redacted")
	}
	hm := out3["headers"].(map[string]any)
	if hm["Authorization"] != redactedSecret {
		t.Errorf("header value not redacted: %v", hm["Authorization"])
	}
}

func TestRedactDoesNotMutateInput(t *testing.T) {
	in := map[string]any{"bot_token": "real"}
	_ = redactChannelConfig("telegram", in)
	if in["bot_token"] != "real" {
		t.Errorf("input map was mutated: %v", in["bot_token"])
	}
}

func TestMergeChannelSecrets(t *testing.T) {
	stored := `{"bot_token":"real-secret","chat_id":"42"}`

	// Sentinel → restored from stored.
	keep := map[string]any{"bot_token": redactedSecret, "chat_id": "99"}
	mergeChannelSecrets("telegram", keep, stored)
	if keep["bot_token"] != "real-secret" {
		t.Errorf("sentinel should restore stored secret, got %v", keep["bot_token"])
	}
	if keep["chat_id"] != "99" {
		t.Errorf("non-secret edit should be preserved, got %v", keep["chat_id"])
	}

	// Real new value → kept (user changed the secret).
	change := map[string]any{"bot_token": "new-secret"}
	mergeChannelSecrets("telegram", change, stored)
	if change["bot_token"] != "new-secret" {
		t.Errorf("changed secret must be kept, got %v", change["bot_token"])
	}
}

func TestMergeWebhookHeaders(t *testing.T) {
	stored := `{"url":"https://h/x","headers":{"Authorization":"Bearer real","X-Env":"prod"}}`
	in := map[string]any{
		"url":     redactedSecret,
		"headers": map[string]any{"Authorization": redactedSecret, "X-Env": "staging"},
	}
	mergeChannelSecrets("webhook", in, stored)
	if in["url"] != "https://h/x" {
		t.Errorf("url not restored: %v", in["url"])
	}
	hm := in["headers"].(map[string]any)
	if hm["Authorization"] != "Bearer real" {
		t.Errorf("header secret not restored: %v", hm["Authorization"])
	}
	if hm["X-Env"] != "staging" {
		t.Errorf("non-secret header edit must persist: %v", hm["X-Env"])
	}
}
