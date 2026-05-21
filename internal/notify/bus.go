package notify

import (
	"sync"
	"sync/atomic"
)

// SSEEvent is the canonical payload pushed to /api/notify/stream subscribers.
// Kind matches the contract enum (notification_event / agent_status_change /
// subscription_sync / tcping_progress / script_log / ota_progress / system).
// Payload is the kind-specific JSON-serialisable value the handler marshals
// before writing the event frame.
type SSEEvent struct {
	Kind    string `json:"kind"`
	Payload any    `json:"payload"`
}

// EventBus fans out SSEEvents to per-user subscribers. Subscribers are
// uniquely identified by an opaque ID (auto-generated) and consume events
// via a buffered channel. The bus is goroutine-safe; subscribers may unsub
// from any goroutine.
//
// Implementation notes:
//   - Each subscription holds a 32-deep buffered channel; if the consumer
//     stalls, additional events for that subscriber are dropped (logged via
//     atomic.AddUint64 Dropped counter). This avoids back-pressure across
//     unrelated subscribers.
//   - The cancel func returned by Subscribe is idempotent and must be
//     called by the consumer (typically from the SSE handler's defer block).
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[uint64]chan SSEEvent
	nextID      atomic.Uint64

	// Dropped counts events lost due to a full subscriber buffer. Surfaced
	// for tests + diagnostic logs.
	Dropped atomic.Uint64
}

// NewEventBus returns a ready-to-use bus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[uint64]chan SSEEvent),
	}
}

// Subscribe registers a new per-user subscriber. The returned cancel func
// must be called to release resources; calling it more than once is safe.
func (b *EventBus) Subscribe(userID string) (<-chan SSEEvent, func()) {
	if b == nil || userID == "" {
		ch := make(chan SSEEvent)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan SSEEvent, 32)
	id := b.nextID.Add(1)
	b.mu.Lock()
	subs, ok := b.subscribers[userID]
	if !ok {
		subs = make(map[uint64]chan SSEEvent)
		b.subscribers[userID] = subs
	}
	subs[id] = ch
	b.mu.Unlock()
	var cancelOnce sync.Once
	cancel := func() {
		cancelOnce.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if subs, ok := b.subscribers[userID]; ok {
				if existing, ok := subs[id]; ok {
					delete(subs, id)
					close(existing)
				}
				if len(subs) == 0 {
					delete(b.subscribers, userID)
				}
			}
		})
	}
	return ch, cancel
}

// Publish pushes event to every subscriber owned by userID. A full subscriber
// channel causes the event to be dropped for that subscriber only; other
// subscribers continue to receive it.
func (b *EventBus) Publish(userID string, event SSEEvent) {
	if b == nil || userID == "" {
		return
	}
	b.mu.RLock()
	subs := b.subscribers[userID]
	// Snapshot the channels under the read lock; deliver without holding
	// the lock so a slow consumer cannot block other subscribers.
	out := make([]chan SSEEvent, 0, len(subs))
	for _, ch := range subs {
		out = append(out, ch)
	}
	b.mu.RUnlock()
	for _, ch := range out {
		select {
		case ch <- event:
		default:
			b.Dropped.Add(1)
		}
	}
}

// Broadcast pushes event to every subscriber across every user. Used for
// system-wide notices (silent-mode prefix rotation, OTA available).
func (b *EventBus) Broadcast(event SSEEvent) {
	if b == nil {
		return
	}
	b.mu.RLock()
	out := make([]chan SSEEvent, 0)
	for _, subs := range b.subscribers {
		for _, ch := range subs {
			out = append(out, ch)
		}
	}
	b.mu.RUnlock()
	for _, ch := range out {
		select {
		case ch <- event:
		default:
			b.Dropped.Add(1)
		}
	}
}

// SubscriberCount returns the number of active subscribers for userID, or the
// total across all users when userID is empty. Used by /healthz and tests.
func (b *EventBus) SubscriberCount(userID string) int {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if userID == "" {
		n := 0
		for _, subs := range b.subscribers {
			n += len(subs)
		}
		return n
	}
	return len(b.subscribers[userID])
}
