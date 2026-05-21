package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"shiguang-vps/internal/storage"
)

// EventLogRetention controls how long delivery rows stay in
// notification_events. The cleanup task runs daily and deletes anything
// older than this window (matches PRD M-NOTIFY default).
const EventLogRetention = 30 * 24 * time.Hour

// ManagerConfig wires the manager to its collaborators. Repo + EventRepo
// are required; Registry defaults to DefaultRegistry; Templates defaults
// to a fresh NewTemplate(); Dedupe defaults to NewDedupe(DefaultDedupeWindow).
// Bus may be nil — when set, every successfully-sent event is also
// re-published as an SSEEvent so live UI consumers see the alert.
type ManagerConfig struct {
	ChannelRepo *storage.NotificationChannelRepo
	EventRepo   *storage.NotificationEventRepo
	UserRepo    *storage.UserRepo
	Registry    *Registry
	Templates   *Template
	Dedupe      *Dedupe
	Bus         *EventBus
	Logger      *slog.Logger
	Now         func() time.Time
}

// Manager is the high-level façade handlers / background workers use to fire
// notifications. Its Emit method:
//
//  1. Computes a dedupe key from (event_type, resource_id) and rejects
//     events that have fired within the last 5 minutes.
//  2. Loads every enabled channel of the recipient user that has opted
//     into the event_type.
//  3. Renders the (subject, body) pair via the locale-specific templates.
//  4. Calls each channel's Send concurrently — failures are isolated to
//     the offending channel and recorded as separate notification_events
//     rows with status=failed.
//  5. Publishes a notification_event SSEEvent for the recipient's bus so
//     the UI bell badge updates in real time.
type Manager struct {
	cfg       ManagerConfig
	logger    *slog.Logger
	now       func() time.Time
}

// NewManager wires the manager. ChannelRepo + EventRepo + UserRepo must be
// non-nil; other deps default to sensible builds.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.ChannelRepo == nil || cfg.EventRepo == nil || cfg.UserRepo == nil {
		return nil, errors.New("notify manager: repos required")
	}
	if cfg.Registry == nil {
		cfg.Registry = DefaultRegistry
	}
	if cfg.Templates == nil {
		cfg.Templates = NewTemplate()
	}
	if cfg.Dedupe == nil {
		cfg.Dedupe = NewDedupe(DefaultDedupeWindow, cfg.Now)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Manager{
		cfg:    cfg,
		logger: cfg.Logger,
		now:    cfg.Now,
	}, nil
}

// Emit fans the event out to every enabled, opted-in channel of the user.
// Returns the number of channels actually invoked (after dedupe / opt-in
// filtering). Errors are aggregated; partial success is the normal case and
// the returned error is non-nil only if every channel failed.
//
// Concurrency model: each channel's Send is invoked in its own goroutine
// with a 30s timeout. The function blocks until all goroutines complete so
// the caller can correlate trace IDs without holding background state.
func (m *Manager) Emit(ctx context.Context, event Event) (int, error) {
	if event.UserID == "" || event.Type == "" {
		return 0, errors.New("notify emit: user_id and type required")
	}
	dedupeKey := DedupeKey(string(event.Type), event.ResourceID)
	if !m.cfg.Dedupe.ShouldEmit(dedupeKey) {
		m.logSkippedDedupe(ctx, event, dedupeKey)
		return 0, nil
	}
	channels, err := m.cfg.ChannelRepo.ListByUser(ctx, event.UserID, string(event.Type))
	if err != nil {
		return 0, fmt.Errorf("list channels: %w", err)
	}
	if len(channels) == 0 {
		return 0, nil
	}
	if event.Locale == "" {
		event.Locale = m.lookupUserLocale(ctx, event.UserID)
	}
	payloadJSON, _ := json.Marshal(event.Payload)
	results := m.dispatch(ctx, event, channels)
	successCount, lastErr := m.persistResults(ctx, event, channels, results, dedupeKey, string(payloadJSON))
	if m.cfg.Bus != nil && successCount > 0 {
		m.cfg.Bus.Publish(event.UserID, SSEEvent{
			Kind: "notification_event",
			Payload: map[string]any{
				"event_type":  event.Type,
				"resource_id": event.ResourceID,
				"subject":     event.Subject,
				"locale":      event.Locale,
			},
		})
	}
	if successCount == 0 && lastErr != nil {
		return 0, lastErr
	}
	return successCount, nil
}

// SendTest fires a single synthetic event through the specified channel,
// bypassing dedupe. Used by the admin UI's "Test" button. The user_id
// argument is the owner of the channel — cross-user requests must fail at
// the handler layer (not here).
func (m *Manager) SendTest(ctx context.Context, channelID, userID string) error {
	if channelID == "" || userID == "" {
		return errors.New("notify send-test: channel_id and user_id required")
	}
	rec, err := m.cfg.ChannelRepo.GetByID(ctx, channelID, userID)
	if err != nil {
		return fmt.Errorf("load channel: %w", err)
	}
	if !rec.Enabled {
		return errors.New("channel disabled")
	}
	ch, cfgMap, err := m.buildChannel(*rec)
	if err != nil {
		return err
	}
	locale := m.lookupUserLocale(ctx, userID)
	event := Event{
		Type:       "test",
		UserID:     userID,
		ResourceID: rec.ID,
		Subject:    fmt.Sprintf("[shiguang-vps] test channel %s", rec.Name),
		Locale:     locale,
		Payload: map[string]any{
			"ChannelName": rec.Name,
			"Kind":        rec.Kind,
			"Now":         m.now().Format(time.RFC3339),
		},
	}
	msg := Message{
		EventType: string(event.Type),
		Subject:   event.Subject,
		Body: fmt.Sprintf("This is a test notification for channel %s (%s). If you see this, the channel is configured correctly.",
			rec.Name, rec.Kind),
		Locale: locale,
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = ch.Send(timeoutCtx, cfgMap, msg)
	status := EventStatusSent
	errMsg := ""
	if err != nil {
		status = EventStatusFailed
		errMsg = err.Error()
	}
	payloadJSON, _ := json.Marshal(event.Payload)
	if _, perr := m.cfg.EventRepo.Insert(ctx, storage.NotificationEventRecord{
		UserID:      userID,
		ChannelID:   rec.ID,
		EventType:   "test",
		PayloadJSON: string(payloadJSON),
		Status:      string(status),
		SentAt:      m.now().UnixMilli(),
		Error:       errMsg,
	}); perr != nil && m.logger != nil {
		m.logger.Warn("send-test event log", slog.String("err", perr.Error()))
	}
	return err
}

// StartCleanup launches the daily event-log retention sweep. The returned
// stop func cancels the worker; safe to call multiple times. Used by main.go
// so notification_events does not grow without bound.
func (m *Manager) StartCleanup(ctx context.Context) func() {
	subCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		// Run once on startup so a freshly-started hub purges immediately.
		m.cleanupOnce(subCtx)
		for {
			select {
			case <-subCtx.Done():
				return
			case <-ticker.C:
				m.cleanupOnce(subCtx)
			}
		}
	}()
	return cancel
}

func (m *Manager) cleanupOnce(ctx context.Context) {
	cutoff := m.now().Add(-EventLogRetention).UnixMilli()
	n, err := m.cfg.EventRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil && m.logger != nil {
		m.logger.Warn("notify cleanup", slog.String("err", err.Error()))
		return
	}
	if n > 0 && m.logger != nil {
		m.logger.Info("notify cleanup", slog.Int64("deleted", n))
	}
}

func (m *Manager) lookupUserLocale(ctx context.Context, userID string) string {
	user, err := m.cfg.UserRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return DefaultLocale
	}
	if user.Locale == "" {
		return DefaultLocale
	}
	return user.Locale
}

// dispatchResult tracks one channel's send outcome for persistResults.
type dispatchResult struct {
	channelIdx int
	err        error
}

// dispatch concurrently invokes Send on every channel and returns one result
// per channel (preserving the order of `channels`).
func (m *Manager) dispatch(ctx context.Context, event Event, channels []storage.NotificationChannelRecord) []dispatchResult {
	results := make([]dispatchResult, len(channels))
	done := make(chan dispatchResult, len(channels))
	for i := range channels {
		go func(i int) {
			rec := channels[i]
			ch, cfgMap, err := m.buildChannel(rec)
			if err != nil {
				done <- dispatchResult{channelIdx: i, err: err}
				return
			}
			subject, body, err := m.cfg.Templates.Render(event, rec.Template)
			if err != nil {
				done <- dispatchResult{channelIdx: i, err: err}
				return
			}
			msg := Message{
				EventType: string(event.Type),
				Subject:   subject,
				Body:      body,
				Locale:    event.Locale,
			}
			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			sendErr := ch.Send(timeoutCtx, cfgMap, msg)
			done <- dispatchResult{channelIdx: i, err: sendErr}
		}(i)
	}
	for i := 0; i < len(channels); i++ {
		r := <-done
		results[r.channelIdx] = r
	}
	return results
}

// persistResults writes notification_events rows for every dispatched channel
// and returns (successCount, lastError).
func (m *Manager) persistResults(ctx context.Context, event Event, channels []storage.NotificationChannelRecord, results []dispatchResult, dedupeKey, payloadJSON string) (int, error) {
	now := m.now().UnixMilli()
	success := 0
	var lastErr error
	for i, r := range results {
		rec := channels[i]
		status := EventStatusSent
		errMsg := ""
		if r.err != nil {
			status = EventStatusFailed
			errMsg = r.err.Error()
			lastErr = r.err
			if m.logger != nil {
				m.logger.Warn("notify channel send failed",
					slog.String("channel_kind", rec.Kind),
					slog.String("event_type", string(event.Type)),
					slog.String("err", errMsg),
				)
			}
		} else {
			success++
		}
		if _, err := m.cfg.EventRepo.Insert(ctx, storage.NotificationEventRecord{
			UserID:      event.UserID,
			ChannelID:   rec.ID,
			EventType:   string(event.Type),
			DedupeKey:   dedupeKey,
			PayloadJSON: payloadJSON,
			Status:      string(status),
			SentAt:      now,
			Error:       errMsg,
			CreatedAt:   now,
		}); err != nil && m.logger != nil {
			m.logger.Warn("notify event log insert",
				slog.String("err", err.Error()),
				slog.String("channel_id", rec.ID),
			)
		}
	}
	return success, lastErr
}

// logSkippedDedupe writes a notification_events row with status =
// skipped_dedupe so operators can audit which events the deduper swallowed.
func (m *Manager) logSkippedDedupe(ctx context.Context, event Event, dedupeKey string) {
	payloadJSON, _ := json.Marshal(event.Payload)
	now := m.now().UnixMilli()
	if _, err := m.cfg.EventRepo.Insert(ctx, storage.NotificationEventRecord{
		UserID:      event.UserID,
		EventType:   string(event.Type),
		DedupeKey:   dedupeKey,
		PayloadJSON: string(payloadJSON),
		Status:      string(EventStatusSkippedDedupe),
		CreatedAt:   now,
	}); err != nil && m.logger != nil {
		m.logger.Warn("notify dedupe-skipped insert", slog.String("err", err.Error()))
	}
}

// buildChannel turns a stored channel row into a Channel ready to Send.
// Returns the parsed config map alongside the Channel so the manager can
// forward it to Channel.Send (every channel uses the same shape).
func (m *Manager) buildChannel(rec storage.NotificationChannelRecord) (Channel, map[string]any, error) {
	var cfgMap map[string]any
	if rec.ConfigJSON != "" {
		if err := json.Unmarshal([]byte(rec.ConfigJSON), &cfgMap); err != nil {
			return nil, nil, fmt.Errorf("decode channel config: %w", err)
		}
	}
	if cfgMap == nil {
		cfgMap = map[string]any{}
	}
	ch, err := m.cfg.Registry.Build(rec.Kind, cfgMap)
	if err != nil {
		return nil, nil, err
	}
	return ch, cfgMap, nil
}
