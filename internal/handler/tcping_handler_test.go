package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shiguang-vps/internal/types"
)

// stubDialFor returns a dialFunc that accepts TCP connections to known
// reachable hosts (matching server == "ok.*") and refuses everything else.
// The listener is closed when the test ends.
//
// We bind a real TCP listener on 127.0.0.1 so connect()-style behaviour
// (RTT measurable in single-digit ms) is preserved without relying on the
// public network. Tests opt in by setting NodeRecord.Server to the listener
// address; the dialFunc rewrites "ok-1.1.1.1" → that loopback address.
func stubDialFor(t *testing.T) dialFunc {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	target := ln.Addr().String()
	return func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
		host, _, _ := net.SplitHostPort(address)
		if strings.HasPrefix(host, "ok") {
			d := net.Dialer{Timeout: timeout}
			return d.DialContext(ctx, network, target)
		}
		// Simulate an unreachable host by returning a context-deadline error
		// after a tiny delay (keeps tests fast while still exercising the
		// "latency=-1" path).
		select {
		case <-time.After(20 * time.Millisecond):
		case <-ctx.Done():
		}
		return nil, &net.OpError{Op: "dial", Err: errStubUnreachable{}}
	}
}

type errStubUnreachable struct{}

func (errStubUnreachable) Error() string { return "stub: host unreachable" }

func TestTCPingSingleReachable(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("alice")
	rec := s.do(http.MethodPost, "/api/tcping/single", TCPingSingleRequest{
		Server: "ok-host", Port: 1234,
	}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("single: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[TCPingSingleResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Data.Reachable || env.Data.LatencyMs < 0 {
		t.Fatalf("expected reachable + non-negative latency, got %+v", env.Data)
	}
	if env.Data.LatencyMs > 5000 {
		t.Fatalf("loopback latency suspiciously high: %d", env.Data.LatencyMs)
	}
}

func TestTCPingSingleUnreachable(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("bob")
	rec := s.do(http.MethodPost, "/api/tcping/single", TCPingSingleRequest{
		Server: "fail-host", Port: 1, TimeoutMs: 200,
	}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("single: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[TCPingSingleResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Reachable || env.Data.LatencyMs != -1 {
		t.Fatalf("expected unreachable, got %+v", env.Data)
	}
}

func TestTCPingBatchUpdatesLatency(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("carol")
	subID := s.seedSub("carol-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-good", "ss", "ok-host", 1234)
	s.seedNode(subID, "n-bad", "ss", "fail-host", 9999)

	rec := s.do(http.MethodPost, "/api/tcping/batch", types.TCPingRequest{
		NodeIDs:   []string{"n-good", "n-bad"},
		TimeoutMs: 500,
	}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch: %d body=%s", rec.Code, rec.Body.String())
	}
	var env nodeEnvelope[types.TCPingResponse]
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Results) != 2 {
		t.Fatalf("expected 2 results, got %+v", env.Data)
	}
	byID := map[string]types.TCPingResult{}
	for _, r := range env.Data.Results {
		byID[r.NodeID] = r
	}
	if !byID["n-good"].Reachable || byID["n-good"].LatencyMs < 0 {
		t.Fatalf("expected n-good reachable, got %+v", byID["n-good"])
	}
	if byID["n-bad"].Reachable || byID["n-bad"].LatencyMs != -1 {
		t.Fatalf("expected n-bad unreachable, got %+v", byID["n-bad"])
	}

	// Verify persistence: the repo now exposes latency on the node record.
	stored, err := s.nodes.GetByID(context.Background(), "n-good", "carol-id")
	if err != nil {
		t.Fatalf("read n-good: %v", err)
	}
	if stored.LastLatencyMs == nil || *stored.LastLatencyMs < 0 {
		t.Fatalf("expected persisted latency on n-good, got %+v", stored.LastLatencyMs)
	}
}

func TestTCPingBatchRejectsOverLimit(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("dave")
	ids := make([]string, tcpingMaxIDs+1)
	for i := range ids {
		ids[i] = "n-" + strconv.Itoa(i)
	}
	rec := s.do(http.MethodPost, "/api/tcping/batch", types.TCPingRequest{NodeIDs: ids}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 over limit, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTCPingNodeEndpointPersists(t *testing.T) {
	s := newNodeTestStack(t)
	_, tok := s.createUserWithToken("eve")
	subID := s.seedSub("eve-id", "primary", types.SubTypeManual)
	s.seedNode(subID, "n-1", "ss", "ok-host", 4242)
	rec := s.do(http.MethodPost, "/api/nodes/n-1/tcping", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("node tcping: %d body=%s", rec.Code, rec.Body.String())
	}
	stored, err := s.nodes.GetByID(context.Background(), "n-1", "eve-id")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if stored.LastLatencyMs == nil {
		t.Fatalf("expected persisted latency, got nil")
	}
}

// TestTCPingBatchConcurrencyBudget ensures the worker pool actually parallelises
// work. We hammer 80 reachable hosts each with a 200ms artificial wait inside
// the dialer; with concurrency=50 the total wall time must be well below the
// sequential equivalent (16s).
func TestTCPingBatchConcurrencyBudget(t *testing.T) {
	const total = 80
	const perHostWait = 100 * time.Millisecond
	var inflight, peak int32
	mu := sync.Mutex{}

	s := newNodeTestStack(t)
	// Replace the dialer with one that blocks per-host so we can observe
	// concurrent workers.
	s.tcping().dial = func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
		cur := atomic.AddInt32(&inflight, 1)
		mu.Lock()
		if cur > peak {
			peak = cur
		}
		mu.Unlock()
		defer atomic.AddInt32(&inflight, -1)
		select {
		case <-time.After(perHostWait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, &net.OpError{Op: "dial", Err: errStubUnreachable{}}
	}

	_, tok := s.createUserWithToken("frank")
	subID := s.seedSub("frank-id", "primary", types.SubTypeManual)
	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		id := "n-" + strconv.Itoa(i)
		ids = append(ids, id)
		s.seedNode(subID, id, "ss", "host-"+strconv.Itoa(i), 80)
	}

	start := time.Now()
	rec := s.do(http.MethodPost, "/api/tcping/batch", types.TCPingRequest{
		NodeIDs: ids, TimeoutMs: 1000,
	}, tok)
	elapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch: %d body=%s", rec.Code, rec.Body.String())
	}
	// With 50 parallel workers and 80 jobs at 100ms each, two waves of work
	// suffice; we leave generous headroom (CI variability).
	if elapsed > 5*time.Second {
		t.Fatalf("batch took %v which exceeds the 5s budget", elapsed)
	}
	if peak < 10 {
		t.Fatalf("expected meaningful parallelism (peak inflight ≥ 10), got %d", peak)
	}
}

// tcping is a tiny test-only accessor that retrieves the wired TCPingHandler
// instance from the stack so tests that need to swap the dialer can do so
// without rebuilding the entire router.
func (s *nodeTestStack) tcping() *TCPingHandler { return s.tcpingHandler }

// TestTCPingBatch200NodesUnder5s pins the PRD M-NODE.1 budget: 200 nodes,
// concurrency=50, total wall-time < 5 seconds. We use a stub dialer that
// sleeps 80ms per host so 200 jobs / 50 workers ≈ 4 waves * 80ms ≈ 320ms of
// pure dial time, leaving plenty of margin for the surrounding handler /
// repo / network overhead.
func TestTCPingBatch200NodesUnder5s(t *testing.T) {
	const total = 200
	const perHostWait = 80 * time.Millisecond
	s := newNodeTestStack(t)
	s.tcping().dial = func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
		select {
		case <-time.After(perHostWait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return nil, &net.OpError{Op: "dial", Err: errStubUnreachable{}}
	}
	_, tok := s.createUserWithToken("grace")
	subID := s.seedSub("grace-id", "primary", types.SubTypeManual)
	ids := make([]string, 0, total)
	for i := 0; i < total; i++ {
		id := "n-" + strconv.Itoa(i)
		ids = append(ids, id)
		s.seedNode(subID, id, "ss", "host-"+strconv.Itoa(i), 80)
	}
	start := time.Now()
	rec := s.do(http.MethodPost, "/api/tcping/batch", types.TCPingRequest{
		NodeIDs: ids, TimeoutMs: 1000,
	}, tok)
	elapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch: %d body=%s", rec.Code, rec.Body.String())
	}
	t.Logf("200-node batch elapsed: %v", elapsed)
	if elapsed > 5*time.Second {
		t.Fatalf("PRD M-NODE.1 budget exceeded: %v", elapsed)
	}
}
