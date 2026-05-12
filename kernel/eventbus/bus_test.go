package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"mobilevc/protocol"
)

func TestPublishAssignsMonotonicCursor(t *testing.T) {
	bus := New()
	defer bus.Close()

	var got []int64
	var mu sync.Mutex
	done := make(chan struct{})
	want := 5

	sub := bus.Subscribe("seq", Filter{}, func(env Envelope) {
		mu.Lock()
		got = append(got, env.Cursor)
		if len(got) == want {
			close(done)
		}
		mu.Unlock()
	})
	defer sub.Close()

	for i := 0; i < want; i++ {
		bus.Publish(Envelope{Source: SourceUser, Topic: "ping"})
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for events")
	}

	mu.Lock()
	defer mu.Unlock()
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("cursor not monotonic: %v", got)
		}
	}
	if bus.LatestCursor() != int64(want) {
		t.Fatalf("LatestCursor=%d want %d", bus.LatestCursor(), want)
	}
}

func TestFilterMatching(t *testing.T) {
	bus := New()
	defer bus.Close()

	var userCount, sessionCount atomic.Int64
	wg := sync.WaitGroup{}
	wg.Add(3)

	subUser := bus.Subscribe("user-only", Filter{Sources: []Source{SourceUser}}, func(env Envelope) {
		userCount.Add(1)
		wg.Done()
	})
	defer subUser.Close()

	subSession := bus.Subscribe("session-only", Filter{Sources: []Source{SourceSession}}, func(env Envelope) {
		sessionCount.Add(1)
		wg.Done()
	})
	defer subSession.Close()

	bus.Publish(Envelope{Source: SourceUser})
	bus.Publish(Envelope{Source: SourceSession})
	bus.Publish(Envelope{Source: SourceSession})

	waitWG(t, &wg, time.Second)

	if userCount.Load() != 1 {
		t.Errorf("userCount=%d want 1", userCount.Load())
	}
	if sessionCount.Load() != 2 {
		t.Errorf("sessionCount=%d want 2", sessionCount.Load())
	}
}

func TestSlowSubscriberDropsAndCounts(t *testing.T) {
	bus := NewWithOptions(Options{SubscriberBuffer: 2, DeliverTimeout: 5 * time.Millisecond})
	defer bus.Close()

	block := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(block)
		}
	}()

	sub := bus.Subscribe("slow", Filter{}, func(env Envelope) {
		<-block
	}).(*subscription)

	for i := 0; i < 50; i++ {
		bus.Publish(Envelope{Source: SourceKernel, Topic: "x"})
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if sub.Dropped() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sub.Dropped() == 0 {
		t.Fatalf("expected drops, got 0")
	}

	close(block)
	released = true
}

func TestWrapSinkPublishesAndDelegates(t *testing.T) {
	bus := New()
	defer bus.Close()

	var bg []any
	var mu sync.Mutex
	rawSink := func(ev any) {
		mu.Lock()
		bg = append(bg, ev)
		mu.Unlock()
	}

	received := make(chan Envelope, 4)
	sub := bus.Subscribe("tap", Filter{Topics: []string{protocol.EventTypeLog}}, func(env Envelope) {
		received <- env
	})
	defer sub.Close()

	wrapped := WrapSink(rawSink, SourceSession, "sess-1", bus)
	wrapped(protocol.NewLogEvent("sess-1", "hello", "stdout"))
	wrapped(protocol.ProgressEvent{Event: protocol.NewBaseEvent(protocol.EventTypeProgress, "sess-1"), Message: "skip", Percent: 50})

	select {
	case env := <-received:
		if env.Source != SourceSession || env.SessionID != "sess-1" || env.Topic != protocol.EventTypeLog {
			t.Fatalf("unexpected envelope: %+v", env)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive log envelope")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bg) != 2 {
		t.Fatalf("rawSink got %d events want 2", len(bg))
	}
}

func TestCloseStopsDispatch(t *testing.T) {
	bus := New()
	got := make(chan Envelope, 1)
	sub := bus.Subscribe("x", Filter{}, func(env Envelope) {
		select {
		case got <- env:
		default:
		}
	})

	bus.Publish(Envelope{Source: SourceUser})
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("event not delivered")
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	sub.Close()
}

// helper.

func waitWG(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("timeout waiting on WaitGroup")
	}
}
