package main

import (
	"strings"
	"sync"
	"time"

	"mobilevc/desktop/notify"
	"mobilevc/kernel/eventbus"
	"mobilevc/protocol"
)

// notifyBridge listens to the kernel bus and raises desktop notifications
// on session lifecycle events (completion / failure) and pending feedback
// suggestions. Per-session de-duplication avoids notification floods.
type notifyBridge struct {
	notifier notify.Notifier
	bus      eventbus.Bus
	sub      eventbus.Subscription

	mu       sync.Mutex
	lastFire map[string]time.Time
}

func newNotifyBridge(notifier notify.Notifier) *notifyBridge {
	if notifier == nil {
		notifier = notify.Noop{}
	}
	return &notifyBridge{notifier: notifier, lastFire: make(map[string]time.Time)}
}

func (n *notifyBridge) Start(bus eventbus.Bus) {
	if n == nil || bus == nil {
		return
	}
	n.bus = bus
	n.sub = bus.Subscribe(
		"agentui-notify",
		eventbus.Filter{}, // all sessions
		n.handle,
	)
}

func (n *notifyBridge) Stop() {
	if n == nil {
		return
	}
	if n.sub != nil {
		n.sub.Close()
		n.sub = nil
	}
}

// shouldFire deduplicates: at most one notification per (sid, kind) per 3s.
func (n *notifyBridge) shouldFire(key string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	if t, ok := n.lastFire[key]; ok && now.Sub(t) < 3*time.Second {
		return false
	}
	n.lastFire[key] = now
	return true
}

func (n *notifyBridge) handle(env eventbus.Envelope) {
	sid := env.SessionID
	switch e := env.Payload.(type) {
	case protocol.ErrorEvent:
		if !n.shouldFire(sid + ":error") {
			return
		}
		_ = n.notifier.Alert("AgentOS · 错误", trim("session "+sid+": "+e.Message, 220))

	case protocol.AgentStateEvent:
		state := strings.ToLower(e.State)
		switch state {
		case "done", "completed", "success", "finished":
			if !n.shouldFire(sid + ":done") {
				return
			}
			_ = n.notifier.Notify("AgentOS · 任务完成", "session "+sid+" 已完成。")
		case "failed", "error":
			if !n.shouldFire(sid + ":failed") {
				return
			}
			_ = n.notifier.Alert("AgentOS · 任务失败", trim("session "+sid+": "+e.Message, 220))
		}

	case protocol.InteractionRequestEvent:
		if !n.shouldFire(sid + ":await") {
			return
		}
		title := e.Title
		if strings.TrimSpace(title) == "" {
			title = "有一个等待确认的动作"
		}
		_ = n.notifier.Notify("AgentOS · 待审决策", "session "+sid+": "+title)

	case protocol.PromptRequestEvent:
		if !n.shouldFire(sid + ":await") {
			return
		}
		msg := strings.TrimSpace(e.Message)
		if msg == "" {
			msg = "需要你的输入"
		}
		_ = n.notifier.Notify("AgentOS · 需要输入", "session "+sid+": "+trim(msg, 160))
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
