package substore

import "testing"

func TestParseSubscriptionUserinfo(t *testing.T) {
	t.Run("full header (3X-UI style)", func(t *testing.T) {
		m := parseSubscriptionUserinfo("upload=100; download=200; total=1000; expire=1719792000")
		if m == nil {
			t.Fatal("expected non-nil meta")
		}
		if m.used != 300 {
			t.Errorf("used: want 300, got %d", m.used)
		}
		if m.total != 1000 {
			t.Errorf("total: want 1000, got %d", m.total)
		}
		if m.expireMs != 1719792000000 {
			t.Errorf("expireMs: want 1719792000000, got %d", m.expireMs)
		}
	})

	t.Run("no expire field leaves expireMs zero", func(t *testing.T) {
		m := parseSubscriptionUserinfo("upload=0; download=5; total=500")
		if m == nil || m.used != 5 || m.total != 500 || m.expireMs != 0 {
			t.Fatalf("unexpected meta: %+v", m)
		}
	})

	t.Run("empty / unparseable returns nil", func(t *testing.T) {
		if parseSubscriptionUserinfo("") != nil {
			t.Error("empty header should be nil")
		}
		if parseSubscriptionUserinfo("garbage; foo=bar") != nil {
			t.Error("header with no numeric known keys should be nil")
		}
	})
}
