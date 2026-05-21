package notify

import (
	"testing"
	"time"
)

func TestEventBus_PublishDeliversToUserSubscribers(t *testing.T) {
	t.Parallel()
	bus := NewEventBus()
	ch, cancel := bus.Subscribe("u1")
	defer cancel()
	bus.Publish("u1", SSEEvent{Kind: "test", Payload: map[string]any{"x": 1}})
	select {
	case ev := <-ch:
		if ev.Kind != "test" {
			t.Fatalf("unexpected kind: %s", ev.Kind)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestEventBus_PublishIsolatesUsers(t *testing.T) {
	t.Parallel()
	bus := NewEventBus()
	a, ca := bus.Subscribe("u1")
	defer ca()
	b, cb := bus.Subscribe("u2")
	defer cb()
	bus.Publish("u1", SSEEvent{Kind: "x"})
	select {
	case <-a:
	case <-time.After(time.Second):
		t.Fatalf("u1 did not receive")
	}
	select {
	case ev := <-b:
		t.Fatalf("u2 must not receive u1's event: %#v", ev)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestEventBus_CancelClosesChannel(t *testing.T) {
	t.Parallel()
	bus := NewEventBus()
	ch, cancel := bus.Subscribe("u1")
	cancel()
	_, ok := <-ch
	if ok {
		t.Fatalf("channel should be closed after cancel")
	}
	// Second cancel must not panic.
	cancel()
}

func TestEventBus_SubscriberCount(t *testing.T) {
	t.Parallel()
	bus := NewEventBus()
	if n := bus.SubscriberCount(""); n != 0 {
		t.Fatalf("empty bus should have 0 subs, got %d", n)
	}
	_, c1 := bus.Subscribe("u1")
	_, c2 := bus.Subscribe("u1")
	_, c3 := bus.Subscribe("u2")
	defer c1()
	defer c2()
	defer c3()
	if bus.SubscriberCount("u1") != 2 || bus.SubscriberCount("u2") != 1 {
		t.Fatalf("counts: u1=%d u2=%d", bus.SubscriberCount("u1"), bus.SubscriberCount("u2"))
	}
	if bus.SubscriberCount("") != 3 {
		t.Fatalf("total count: %d", bus.SubscriberCount(""))
	}
}

func TestEventBus_BroadcastReachesAllUsers(t *testing.T) {
	t.Parallel()
	bus := NewEventBus()
	a, ca := bus.Subscribe("u1")
	defer ca()
	b, cb := bus.Subscribe("u2")
	defer cb()
	bus.Broadcast(SSEEvent{Kind: "system"})
	for _, ch := range []<-chan SSEEvent{a, b} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("subscriber missed broadcast")
		}
	}
}
