package notify

import (
	"strings"
	"testing"
)

func TestTemplate_RenderNodeOffline_zhCN(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	event := Event{
		Type:       EventNodeOffline,
		Locale:     "zh-CN",
		ResourceID: "node-1",
		Payload: NodeOfflinePayload{
			NodeID:    "node-1",
			NodeName:  "测试节点",
			AgentName: "agent-A",
			Duration:  "3 分钟",
		},
	}
	subj, body, err := tmpl.Render(event, "")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(subj, "测试节点") {
		t.Fatalf("expected node name in zh subject, got %q", subj)
	}
	if !strings.Contains(body, "3 分钟") {
		t.Fatalf("expected duration in zh body, got %q", body)
	}
}

func TestTemplate_RenderNodeOffline_en(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	event := Event{
		Type:       EventNodeOffline,
		Locale:     "en",
		ResourceID: "node-1",
		Payload: NodeOfflinePayload{
			NodeID:    "node-1",
			NodeName:  "edge-node",
			AgentName: "agent-A",
			Duration:  "3m",
		},
	}
	subj, body, err := tmpl.Render(event, "")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(subj, "edge-node") {
		t.Fatalf("expected node name in en subject, got %q", subj)
	}
	if !strings.Contains(strings.ToLower(body), "offline") {
		t.Fatalf("expected 'offline' in en body, got %q", body)
	}
}

func TestTemplate_RenderAllLocales_AllEvents(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	events := []EventType{
		EventNodeOffline, EventTrafficThreshold, EventSubscriptionSyncFailed,
		EventBackupCompleted, EventLoginAnomaly, EventOTAAvailable,
		EventScriptAlert,
	}
	payload := map[string]any{
		"NodeName": "n", "NodeID": "n1", "AgentName": "a", "Duration": "1m",
		"UsagePercent": 99.5, "TotalUsed": int64(1024), "ThresholdPct": int32(90),
		"PeriodStart": "2026-01-01", "PeriodEnd": "2026-01-31",
		"SubscriptionName": "sub-x", "ErrorMessage": "boom",
		"Filename": "b.tgz", "SizeBytes": int64(1024), "DurationMs": int64(50),
		"Username": "u", "IP": "1.2.3.4", "Reason": "weird",
		"LatestVersion": "1.2.3", "CurrentVersion": "1.2.0", "ReleaseURL": "https://...",
		"ScriptName": "s", "ErrorMsg": "type error",
	}
	for _, loc := range SupportedLocales {
		for _, ev := range events {
			subj, body, err := tmpl.Render(Event{
				Type:       ev,
				Locale:     loc,
				ResourceID: "r1",
				Payload:    payload,
			}, "")
			if err != nil {
				t.Fatalf("render %s/%s: %v", loc, ev, err)
			}
			if subj == "" || body == "" {
				t.Fatalf("empty render result for %s/%s: subj=%q body=%q", loc, ev, subj, body)
			}
		}
	}
}

func TestTemplate_FallbackToDefaultLocale(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	// "fr" is unsupported — must fall back through SupportedLocales.
	subj, body, err := tmpl.Render(Event{
		Type:   EventNodeOffline,
		Locale: "fr",
		Payload: NodeOfflinePayload{
			NodeName: "edge-node",
			Duration: "1m",
		},
	}, "")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if subj == "" || body == "" {
		t.Fatalf("expected fallback render to succeed, got empty result")
	}
}

func TestTemplate_OverrideBody_RenderedAsInlineTemplate(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	override := "custom: {{ .Payload.NodeName }} -> {{ .Payload.Duration }}"
	_, body, err := tmpl.Render(Event{
		Type: EventNodeOffline,
		Payload: NodeOfflinePayload{
			NodeName: "edge-node",
			Duration: "5s",
		},
	}, override)
	if err != nil {
		t.Fatalf("render override: %v", err)
	}
	if body != "custom: edge-node -> 5s" {
		t.Fatalf("override body: got %q", body)
	}
}

func TestTemplate_RenderHTML_FallsBackToPlainText(t *testing.T) {
	t.Parallel()
	tmpl := NewTemplate()
	html, err := tmpl.RenderHTML(Event{
		Type: EventNodeOffline,
		Payload: NodeOfflinePayload{
			NodeName: "edge",
			Duration: "1m",
		},
	})
	if err != nil {
		t.Fatalf("render html: %v", err)
	}
	if !strings.Contains(html, "<p>") {
		t.Fatalf("expected HTML wrapping, got %q", html)
	}
}

func TestNormaliseLocale(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"", DefaultLocale},
		{"zh", "zh-CN"},
		{"en", "en"},
		{"ja", "ja"},
		{"ko", "ko"},
		{"xx", "xx"}, // passed through; lookup fallback handles unknown
	}
	for _, tc := range cases {
		if got := normaliseLocale(tc.in); got != tc.want {
			t.Fatalf("normaliseLocale(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
