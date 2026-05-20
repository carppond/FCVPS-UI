package substore

import (
	"os"
	"testing"
)

func TestParseACL4SSR_Sample(t *testing.T) {
	raw, err := os.ReadFile("testdata/acl4ssr_sample.ini")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	cfg, err := ParseACL4SSR(string(raw))
	if err != nil {
		t.Fatalf("ParseACL4SSR: %v", err)
	}
	if cfg.General["loglevel"] != "info" {
		t.Errorf("general[loglevel]: %v", cfg.General)
	}
	if len(cfg.Proxy) != 3 {
		t.Errorf("want 3 proxies, got %d", len(cfg.Proxy))
	}
	if len(cfg.Groups) != 3 {
		t.Errorf("want 3 groups, got %d", len(cfg.Groups))
	}
	// Auto group should have parsed type / url / interval / tolerance.
	auto := cfg.Groups[0]
	if auto.Name != "Auto" || auto.Type != "url-test" {
		t.Errorf("auto group: %+v", auto)
	}
	if auto.URL == "" {
		t.Error("auto group missing url")
	}
	if auto.Interval != 300 || auto.Tolerance != 50 {
		t.Errorf("auto interval/tolerance: %d/%d", auto.Interval, auto.Tolerance)
	}
	if len(cfg.Rules) != 5 {
		t.Errorf("want 5 rules, got %d", len(cfg.Rules))
	}
	// IP-CIDR rule must have NoResolve=true.
	var found bool
	for _, r := range cfg.Rules {
		if r.Type == "IP-CIDR" {
			if !r.NoResolve {
				t.Errorf("IP-CIDR should be no-resolve")
			}
			found = true
		}
	}
	if !found {
		t.Error("IP-CIDR rule missing")
	}
	if cfg.Override["clash.mode"] != "rule" {
		t.Errorf("override: %v", cfg.Override)
	}
}

func TestParseACL4SSR_Empty(t *testing.T) {
	_, err := ParseACL4SSR("")
	if err == nil {
		t.Fatal("want error on empty input")
	}
}

func TestParseACL4SSR_OnlyComments(t *testing.T) {
	cfg, err := ParseACL4SSR("# only comments\n; another\n// third\n")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cfg.Proxy) != 0 || len(cfg.Groups) != 0 || len(cfg.Rules) != 0 {
		t.Error("nothing should be parsed from comment-only file")
	}
}

func TestParseACL4SSR_MalformedLines(t *testing.T) {
	in := `[Proxy]
not-a-uri
[Proxy Group]
NoEquals
GoodGroup=select,DIRECT
[Rule]
INVALID
DOMAIN,example.com,Proxy
`
	cfg, err := ParseACL4SSR(in)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(cfg.Proxy) != 0 {
		t.Errorf("malformed URI should be skipped")
	}
	if len(cfg.Groups) != 1 {
		t.Errorf("want 1 group, got %d", len(cfg.Groups))
	}
	if len(cfg.Rules) != 1 {
		t.Errorf("want 1 rule, got %d", len(cfg.Rules))
	}
}
