package main

import (
	"context"
	"sync"
	"time"

	"mobilevc/kernel/eventbus"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventBridge fans out kernel bus events to the Wails frontend on the
// channel "agent:<sessionID>". One subscription per session, replaced
// on Attach.
type eventBridge struct {
	mu   sync.Mutex
	subs map[string]eventbus.Subscription
}

func newEventBridge() *eventBridge {
	return &eventBridge{subs: make(map[string]eventbus.Subscription)}
}

func (b *eventBridge) Attach(ctx context.Context, bus eventbus.Bus, sid string) {
	if bus == nil || sid == "" {
		return
	}
	channel := "agent:" + sid

	b.mu.Lock()
	if existing, ok := b.subs[sid]; ok {
		existing.Close()
		delete(b.subs, sid)
	}
	b.mu.Unlock()

	sub := bus.Subscribe(
		"agentui-"+sid,
		eventbus.Filter{SessionIDs: []string{sid}},
		func(env eventbus.Envelope) {
			wailsRuntime.EventsEmit(ctx, channel, map[string]any{
				"cursor":    env.Cursor,
				"source":    string(env.Source),
				"topic":     env.Topic,
				"sessionId": env.SessionID,
				"timestamp": env.Timestamp.UTC().Format(time.RFC3339Nano),
				"payload":   env.Payload,
			})
		},
	)

	b.mu.Lock()
	b.subs[sid] = sub
	b.mu.Unlock()
}

func (b *eventBridge) Detach(sid string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.subs[sid]; ok {
		s.Close()
		delete(b.subs, sid)
	}
}

func (b *eventBridge) DetachAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.subs {
		s.Close()
	}
	b.subs = make(map[string]eventbus.Subscription)
}

// busSinkFor returns an engine.EventSink that publishes every event from a
// session.Service onto the kernel bus, tagged with the given sessionID. The
// emitted payloads are protocol.* event structs from the session controller.
func busSinkFor(bus eventbus.Bus, sid string) func(any) {
	return func(ev any) {
		if bus == nil {
			return
		}
		bus.Publish(eventbus.Envelope{
			Source:    eventbus.SourceSession,
			Topic:     eventbus.TopicOf(ev),
			SessionID: sid,
			Timestamp: time.Now(),
			Payload:   ev,
		})
	}
}
