package watchdog

import (
	"sync/atomic"
	"testing"
	"time"

	"mobilevc/kernel/eventbus"
	"mobilevc/kernel/lockmgr"
	"mobilevc/protocol"
)

func TestTimeoutFiresAndReleasesLock(t *testing.T) {
	bus := eventbus.New()
	defer bus.Close()
	lm := lockmgr.New(nil)
	defer lm.Close()

	if _, err := lm.Acquire("sess-x", "exec-1", time.Hour); err != nil {
		t.Fatal(err)
	}

	timeoutEvents := make(chan eventbus.Envelope, 4)
	bus.Subscribe("tap", eventbus.Filter{Sources: []eventbus.Source{eventbus.SourceKernel}}, func(env eventbus.Envelope) {
		if e, ok := env.Payload.(protocol.ErrorEvent); ok && e.Code == "watchdog_timeout" {
			select {
			case timeoutEvents <- env:
			default:
			}
		}
	})

	w := NewWithConfig(bus, lm, Config{MinIdleTimeout: 10 * time.Millisecond})
	defer w.Close()
	w.Watch(WatchOptions{
		ExecutionID: "exec-1",
		SessionID:   "sess-x",
		LockKey:     "sess-x",
		Timeout:     50 * time.Millisecond,
	})

	select {
	case env := <-timeoutEvents:
		errEvent := env.Payload.(protocol.ErrorEvent)
		if errEvent.ExecutionID != "exec-1" {
			t.Fatalf("ExecutionID=%q want exec-1", errEvent.ExecutionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout event not delivered")
	}

	if owner, _ := lm.Holder("sess-x"); owner != "" {
		t.Fatalf("lock not released, holder=%q", owner)
	}
	if w.Pending() != 0 {
		t.Fatalf("Pending=%d want 0", w.Pending())
	}
}

func TestHeartbeatPostpones(t *testing.T) {
	bus := eventbus.New()
	defer bus.Close()
	w := NewWithConfig(bus, nil, Config{MinIdleTimeout: 10 * time.Millisecond})
	defer w.Close()

	var fired atomic.Int64
	bus.Subscribe("tap", eventbus.Filter{}, func(env eventbus.Envelope) {
		if e, ok := env.Payload.(protocol.ErrorEvent); ok && e.Code == "watchdog_timeout" {
			fired.Add(1)
		}
	})

	w.Watch(WatchOptions{ExecutionID: "exec-2", SessionID: "s", Timeout: 80 * time.Millisecond})
	for i := 0; i < 4; i++ {
		time.Sleep(40 * time.Millisecond)
		w.Heartbeat("exec-2")
	}
	w.Settle("exec-2")
	time.Sleep(120 * time.Millisecond)

	if fired.Load() != 0 {
		t.Fatalf("timeout fired %d times despite heartbeats", fired.Load())
	}
}

func TestSettleStopsTimer(t *testing.T) {
	bus := eventbus.New()
	defer bus.Close()
	w := NewWithConfig(bus, nil, Config{MinIdleTimeout: 10 * time.Millisecond})
	defer w.Close()

	var fired atomic.Int64
	bus.Subscribe("tap", eventbus.Filter{}, func(env eventbus.Envelope) {
		if e, ok := env.Payload.(protocol.ErrorEvent); ok && e.Code == "watchdog_timeout" {
			fired.Add(1)
		}
	})

	w.Watch(WatchOptions{ExecutionID: "exec-3", SessionID: "s", Timeout: 60 * time.Millisecond})
	w.Settle("exec-3")
	time.Sleep(150 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatalf("timeout fired after Settle")
	}
}

func TestBusEventActsAsHeartbeat(t *testing.T) {
	bus := eventbus.New()
	defer bus.Close()
	w := NewWithConfig(bus, nil, Config{MinIdleTimeout: 10 * time.Millisecond})
	defer w.Close()

	var fired atomic.Int64
	bus.Subscribe("tap", eventbus.Filter{Sources: []eventbus.Source{eventbus.SourceKernel}}, func(env eventbus.Envelope) {
		if e, ok := env.Payload.(protocol.ErrorEvent); ok && e.Code == "watchdog_timeout" {
			fired.Add(1)
		}
	})

	w.Watch(WatchOptions{ExecutionID: "exec-4", SessionID: "s", Timeout: 80 * time.Millisecond})
	for i := 0; i < 4; i++ {
		time.Sleep(40 * time.Millisecond)
		bus.Publish(eventbus.Envelope{
			Source:    eventbus.SourceSession,
			SessionID: "s",
			Topic:     protocol.EventTypeLog,
			Payload: protocol.LogEvent{
				Event: protocol.Event{
					Type:        protocol.EventTypeLog,
					SessionID:   "s",
					RuntimeMeta: protocol.RuntimeMeta{ExecutionID: "exec-4"},
				},
				Message: "tick",
			},
		})
	}
	w.Settle("exec-4")
	time.Sleep(120 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatalf("timeout fired despite bus heartbeats")
	}
}
