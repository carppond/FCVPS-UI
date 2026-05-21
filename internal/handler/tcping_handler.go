// Package handler — TCPing endpoints.
//
// Implements the M-NODE TCPing surface (T-11):
//
//   - POST /api/tcping/single        — single (server, port) probe; no DB write
//   - POST /api/tcping/batch         — batch probe by node IDs; updates DB
//   - POST /api/nodes/{id}/tcping    — single node probe; updates DB
//
// Concurrency budget per PRD M-NODE.1: max 200 IDs per batch, default
// concurrency=50, default timeout=5000ms (caller can override up to 30000ms).
// The implementation uses a buffered job channel + N workers; results are
// written into a fixed-size slice indexed by job number so order is preserved.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"shiguang-vps/internal/auth"
	"shiguang-vps/internal/handler/middleware"
	"shiguang-vps/internal/storage"
	"shiguang-vps/internal/types"
	"shiguang-vps/internal/util"
)

// TCPing constraints — PRD M-NODE.1.
const (
	tcpingMaxIDs           = 200
	tcpingMaxConcurrency   = 50
	tcpingDefaultTimeoutMs = 5000
	tcpingMinTimeoutMs     = 100
	tcpingMaxTimeoutMs     = 30000
)

// dialFunc allows tests to substitute a deterministic dialer (a net.Listen
// loop) for the production net.DialTimeout. Production paths leave it nil so
// the package default is used.
type dialFunc func(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error)

// TCPingHandler hosts the /api/tcping/* and /api/nodes/{id}/tcping endpoints.
type TCPingHandler struct {
	nodes  *storage.NodeRepo
	logger *slog.Logger
	dial   dialFunc
	now    func() time.Time
}

// NewTCPingHandler wires the handler. dial may be nil; production paths use
// net.DialTimeout. now may be nil; defaults to time.Now.
func NewTCPingHandler(nodes *storage.NodeRepo, logger *slog.Logger) *TCPingHandler {
	return &TCPingHandler{nodes: nodes, logger: logger, now: time.Now}
}

// TCPingSingleRequest is the body of POST /api/tcping/single. Server + port
// describe an arbitrary endpoint; the call does NOT touch the database and
// can be used by the UI to probe a host the user is about to add as a node.
type TCPingSingleRequest struct {
	Server    string `json:"server"`
	Port      int32  `json:"port"`
	TimeoutMs int32  `json:"timeout_ms,omitempty"`
}

// TCPingSingleResponse is the response payload (a single TCPingResult plus
// an optional human-readable error).
type TCPingSingleResponse struct {
	LatencyMs int32  `json:"latency_ms"`
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// Single implements POST /api/tcping/single. The endpoint is authenticated
// so anonymous callers can't enumerate the network from the hub's public IP.
func (h *TCPingHandler) Single(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	_ = auth.MustUserFromContext(r.Context())
	var req TCPingSingleRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if req.Server == "" || req.Port <= 0 || req.Port > 65535 {
		util.RespondError(w, types.ErrValidationOutOfRange,
			"server / port required (1-65535)", nil, traceID)
		return
	}
	timeout := normaliseTimeout(req.TimeoutMs)
	latency, err := h.probe(r.Context(), req.Server, int(req.Port), timeout)
	resp := TCPingSingleResponse{LatencyMs: latency, Reachable: latency >= 0}
	if err != nil && latency < 0 {
		resp.Error = err.Error()
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[TCPingSingleResponse]{
		Data: resp, RequestID: traceID,
	})
}

// Batch implements POST /api/tcping/batch. Body contains a slice of node IDs
// owned by the calling user; the handler concurrently probes every node and
// persists the latency in a single transaction at the end.
//
// Hard limit: 200 IDs per request (PRD M-NODE.1). Concurrency: 50 (override
// via TCPingRequest.Concurrency, capped at 50).
func (h *TCPingHandler) Batch(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	var req types.TCPingRequest
	if err := util.DecodeJSONBody(r, &req); err != nil {
		util.RespondError(w, types.ErrValidationInvalidFormat, "invalid body", nil, traceID)
		return
	}
	if len(req.NodeIDs) == 0 {
		util.RespondError(w, types.ErrValidationRequiredField, "node_ids required", nil, traceID)
		return
	}
	if len(req.NodeIDs) > tcpingMaxIDs {
		util.RespondError(w, types.ErrValidationOutOfRange,
			fmt.Sprintf("at most %d ids per batch", tcpingMaxIDs), nil, traceID)
		return
	}
	timeout := normaliseTimeout(req.TimeoutMs)
	concurrency := int(req.Concurrency)
	if concurrency <= 0 || concurrency > tcpingMaxConcurrency {
		concurrency = tcpingMaxConcurrency
	}

	// Hydrate every node up-front so we can validate ownership in one pass +
	// reject unknown / cross-user IDs before launching workers.
	jobs := make([]tcpingJob, 0, len(req.NodeIDs))
	for _, id := range req.NodeIDs {
		rec, err := h.nodes.GetByID(r.Context(), id, user.ID)
		if err != nil {
			// Skip unknown / cross-user IDs silently so the batch doesn't
			// fail wholesale; the client receives latency=-1 for those.
			jobs = append(jobs, tcpingJob{id: id, skip: true})
			continue
		}
		jobs = append(jobs, tcpingJob{
			id: id, server: rec.Server, port: int(rec.Port),
		})
	}
	results := h.runBatch(r.Context(), jobs, concurrency, timeout)

	// Persist latency results in one shot.
	now := h.now().UnixMilli()
	toPersist := make([]storage.TCPingPersist, 0, len(results))
	for i, res := range results {
		if jobs[i].skip {
			continue
		}
		toPersist = append(toPersist, storage.TCPingPersist{
			NodeID:    jobs[i].id,
			LatencyMs: res.LatencyMs,
			TestedAt:  now,
		})
	}
	if err := h.nodes.BatchUpdateLatency(r.Context(), toPersist); err != nil && h.logger != nil {
		h.logger.Warn("persist batch tcping",
			slog.String("err", err.Error()), slog.String("trace_id", traceID))
	}

	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.TCPingResponse]{
		Data: types.TCPingResponse{Results: results}, RequestID: traceID,
	})
}

// Node implements POST /api/nodes/{id}/tcping — single-node measure that also
// writes the result to the database.
func (h *TCPingHandler) Node(w http.ResponseWriter, r *http.Request) {
	traceID := middleware.TraceIDFromContext(r.Context())
	user := auth.MustUserFromContext(r.Context())
	id := r.PathValue("id")
	rec, err := h.nodes.GetByID(r.Context(), id, user.ID)
	if err != nil {
		if errors.Is(err, storage.ErrNodeNotFound) {
			util.RespondError(w, types.ErrNotFoundNode, "node not found", nil, traceID)
			return
		}
		util.RespondError(w, types.ErrInternalUnknown, "internal error", nil, traceID)
		return
	}
	// Optional override via query string (?timeout_ms=...) — keeps Postman
	// usage friction-free without forcing a JSON body.
	timeout := normaliseTimeout(parseInt32Query(r, "timeout_ms"))
	latency, _ := h.probe(r.Context(), rec.Server, int(rec.Port), timeout)
	now := h.now().UnixMilli()
	if err := h.nodes.BatchUpdateLatency(r.Context(), []storage.TCPingPersist{
		{NodeID: id, LatencyMs: latency, TestedAt: now},
	}); err != nil && h.logger != nil {
		h.logger.Warn("persist node tcping",
			slog.String("err", err.Error()), slog.String("trace_id", traceID))
	}
	util.RespondJSON(w, http.StatusOK, types.APIResponse[types.TCPingResult]{
		Data: types.TCPingResult{
			NodeID: id, LatencyMs: latency, Reachable: latency >= 0,
		},
		RequestID: traceID,
	})
}

// tcpingJob is a single unit of work for the batch worker pool.
type tcpingJob struct {
	id     string
	server string
	port   int
	skip   bool // unknown / cross-user IDs are marked so workers no-op
}

// runBatch fans out the supplied jobs across a worker pool of size
// concurrency, returning a slice of TCPingResult sharing the input order.
func (h *TCPingHandler) runBatch(ctx context.Context, jobs []tcpingJob, concurrency int, timeout time.Duration) []types.TCPingResult {
	results := make([]types.TCPingResult, len(jobs))
	queue := make(chan int, len(jobs))
	for i := range jobs {
		queue <- i
	}
	close(queue)

	var wg sync.WaitGroup
	if concurrency > len(jobs) {
		concurrency = len(jobs)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range queue {
				job := jobs[idx]
				if job.skip {
					results[idx] = types.TCPingResult{
						NodeID: job.id, LatencyMs: -1, Reachable: false,
					}
					continue
				}
				latency, _ := h.probe(ctx, job.server, job.port, timeout)
				results[idx] = types.TCPingResult{
					NodeID:    job.id,
					LatencyMs: latency,
					Reachable: latency >= 0,
				}
			}
		}()
	}
	wg.Wait()
	return results
}

// probe runs a single TCP dial with the supplied timeout, returning the
// elapsed milliseconds (rounded down) on success and -1 + the underlying
// error on failure (including timeout). Caller-side cancellation via
// ctx.Done is respected.
func (h *TCPingHandler) probe(ctx context.Context, server string, port int, timeout time.Duration) (int32, error) {
	addr := net.JoinHostPort(server, strconv.Itoa(port))
	start := time.Now()
	conn, err := h.dialer()(ctx, "tcp", addr, timeout)
	if err != nil {
		return -1, err
	}
	_ = conn.Close()
	elapsed := time.Since(start)
	// Use ceil(microseconds / 1000) instead of trunc so a sub-millisecond
	// dial (e.g. 0.7 ms to a CDN-close server) shows as "1 ms" instead of
	// "0 ms" — which users misread as "didn't measure" or "broken".
	us := elapsed.Microseconds()
	if us <= 0 {
		return 0, nil
	}
	ms := int32((us + 999) / 1000) // ceil
	return ms, nil
}

// dialer returns the configured dialFunc (test override) or the package
// default backed by net.DialTimeout with context cancellation observed.
func (h *TCPingHandler) dialer() dialFunc {
	if h.dial != nil {
		return h.dial
	}
	return defaultDial
}

// defaultDial honours both the explicit timeout and the request context's
// Done channel — whichever expires first ends the dial.
func defaultDial(ctx context.Context, network, address string, timeout time.Duration) (net.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, network, address)
}

// normaliseTimeout clamps the caller-supplied timeout into the allowed
// [tcpingMinTimeoutMs, tcpingMaxTimeoutMs] band, defaulting to 5000ms when
// the value is unset (≤ 0).
func normaliseTimeout(ms int32) time.Duration {
	v := int(ms)
	if v <= 0 {
		v = tcpingDefaultTimeoutMs
	}
	if v < tcpingMinTimeoutMs {
		v = tcpingMinTimeoutMs
	}
	if v > tcpingMaxTimeoutMs {
		v = tcpingMaxTimeoutMs
	}
	return time.Duration(v) * time.Millisecond
}

// parseInt32Query pulls a single integer query param (no body decode). Empty
// / malformed input returns 0 so normaliseTimeout falls back to the default.
func parseInt32Query(r *http.Request, key string) int32 {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0
	}
	if n > int(int32(^uint32(0)>>1)) {
		return int32(int32(^uint32(0) >> 1))
	}
	return int32(n)
}

// ensure encoding/json is referenced (used implicitly via util.DecodeJSONBody
// in a future expansion). Keep the import slot if compiler ever inlines.
var _ = json.Number("0")
