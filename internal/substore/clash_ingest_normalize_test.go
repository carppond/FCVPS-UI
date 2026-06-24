package substore

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestClashIngest_NormalizesForAllProducers(t *testing.T) {
	clashSub := `proxies:
  - {name: r, type: vless, server: 1.2.3.4, port: 443, uuid: u, network: tcp, tls: true, servername: www.bing.com, flow: xtls-rprx-vision, client-fingerprint: chrome, reality-opts: {public-key: PBK, short-id: ab}}
  - {name: w, type: vmess, server: 5.6.7.8, port: 443, uuid: u2, alterId: 0, cipher: auto, network: ws, tls: true, servername: a.com, ws-opts: {path: /pp, headers: {Host: a.com}}}`
	nodes, err := parseClashYAML([]byte(clashSub))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(nodes))
	}
	r := nodes[0]
	if !r.Reality || r.SNI != "www.bing.com" || r.Raw["pbk"] != "PBK" || r.Raw["sid"] != "ab" {
		t.Errorf("reality node not normalized: Reality=%v SNI=%q pbk=%v sid=%v", r.Reality, r.SNI, r.Raw["pbk"], r.Raw["sid"])
	}
	w := nodes[1]
	if w.SNI != "a.com" || w.Path != "/pp" || w.Host != "a.com" || w.Raw["aid"] != "0" {
		t.Errorf("ws node not normalized: SNI=%q Path=%q Host=%q aid=%v", w.SNI, w.Path, w.Host, w.Raw["aid"])
	}
	// uri-list output must carry SNI + reality + ws path now
	uri, _ := ProduceURIList(nodes, ClashProducerOpts{})
	dec, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(uri)))
	if derr != nil {
		dec, _ = base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(uri)))
	}
	us := string(dec)
	for _, want := range []string{"security=reality", "pbk=PBK", "sni=www.bing.com", "flow=xtls-rprx-vision", "encryption=none"} {
		if !strings.Contains(us, want) {
			t.Errorf("uri-list missing %q:\n%s", want, us)
		}
	}
}
