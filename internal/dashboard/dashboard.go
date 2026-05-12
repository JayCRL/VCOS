// Package dashboard provides a lightweight HTTP observability surface for
// AgentOS — event stream (SSE), active sessions, memory browser, scheduler
// decisions, and the evolution feedback loop (proposals + accept/reject). It
// is the "可视化与监控台" from the architecture diagram.
//
// Mount it alongside the WebSocket gateway:
//
//	mux := http.NewServeMux()
//	mux.Handle("/ws", gatewayHandler)
//	mux.Handle("/dashboard/", dashboard.NewHandler(bus, memStore, kernel, feedback))
package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"mobilevc/internal/eventbus"
	"mobilevc/internal/feedback"
	"mobilevc/internal/memory"
)

//go:embed index.html
var indexHTML embed.FS

// Kernel is the subset of the agent kernel needed by the dashboard.
type Kernel interface {
	ActiveSessions() []string
	ConsoleExec(ctx context.Context, sessionID, message, cwd string) (string, error)
}

// FeedbackAPI is the feedback surface the dashboard needs.
type FeedbackAPI interface {
	Pending() []feedback.Suggestion
	Decide(ctx context.Context, suggestionID string, decision feedback.Decision, adjustedText string) (feedback.Record, error)
	History() []feedback.Record
	Stats() feedback.Stats
}

// Handler serves dashboard HTTP endpoints.
type Handler struct {
	Bus      eventbus.Bus
	MemStore memory.Store
	Kernel   Kernel
	Feedback FeedbackAPI

	mu           sync.RWMutex
	recentEvents []eventbus.Envelope
	maxEvents    int

	decisions    []DecisionRecord
	maxDecisions int
}

// DecisionRecord captures a scheduler decision for the dashboard.
type DecisionRecord struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"sessionId,omitempty"`
	Outcome   string    `json:"outcome"`
	Reason    string    `json:"reason,omitempty"`
	Conflict  string    `json:"conflict,omitempty"`
}

// NewHandler creates a dashboard handler.
func NewHandler(bus eventbus.Bus, memStore memory.Store, kernel Kernel, fb FeedbackAPI) *Handler {
	h := &Handler{
		Bus:          bus,
		MemStore:     memStore,
		Kernel:       kernel,
		Feedback:     fb,
		maxEvents:    500,
		maxDecisions: 200,
	}
	bus.Subscribe("dashboard", eventbus.Filter{}, func(env eventbus.Envelope) {
		h.recordEvent(env)
	})
	return h
}

// RecordDecision stores a scheduler decision for later retrieval.
func (h *Handler) RecordDecision(sessionID, outcome, reason, conflict string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.decisions = append(h.decisions, DecisionRecord{
		Time:      time.Now(),
		SessionID: sessionID,
		Outcome:   outcome,
		Reason:    reason,
		Conflict:  conflict,
	})
	if len(h.decisions) > h.maxDecisions {
		h.decisions = h.decisions[len(h.decisions)-h.maxDecisions:]
	}
}

func (h *Handler) recordEvent(env eventbus.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recentEvents = append(h.recentEvents, env)
	if len(h.recentEvents) > h.maxEvents {
		h.recentEvents = h.recentEvents[len(h.recentEvents)-h.maxEvents:]
	}
}

// ServeHTTP routes to sub-handlers based on path prefix.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/dashboard")
	path = strings.TrimPrefix(path, "/")

	switch {
	case path == "" || path == "/":
		h.handleIndex(w, r)
	case path == "events" || strings.HasPrefix(path, "events"):
		h.handleEvents(w, r)
	case path == "sessions":
		h.handleSessions(w, r)
	case path == "memory" || strings.HasPrefix(path, "memory/"):
		h.handleMemory(w, r, strings.TrimPrefix(path, "memory"))
	case path == "decisions":
		h.handleDecisions(w, r)
	case path == "console/exec":
		h.handleConsoleExec(w, r)
	case strings.HasPrefix(path, "feedback/"):
		h.handleFeedback(w, r, strings.TrimPrefix(path, "feedback/"))
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleConsoleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"sessionId"`
		Message   string `json:"message"`
		CWD       string `json:"cwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sessionID, err := h.Kernel.ConsoleExec(r.Context(), req.SessionID, req.Message, req.CWD)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sessionId": sessionID, "ok": "true"})
}

func (h *Handler) handleFeedback(w http.ResponseWriter, r *http.Request, sub string) {
	if h.Feedback == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "feedback not configured"})
		return
	}
	switch {
	case sub == "pending" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.Feedback.Pending())
	case sub == "history" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.Feedback.History())
	case sub == "stats" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, h.Feedback.Stats())
	case sub == "decide" && r.Method == http.MethodPost:
		var req struct {
			SuggestionID string `json:"suggestionId"`
			Decision     string `json:"decision"`
			AdjustedText string `json:"adjustedText,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		rec, err := h.Feedback.Decide(r.Context(), req.SuggestionID, feedback.Decision(req.Decision), req.AdjustedText)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, rec)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := fs.ReadFile(indexHTML, "index.html")
	if err != nil {
		http.Error(w, "dashboard HTML not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("stream") == "1" {
		h.handleSSE(w, r)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	start := len(h.recentEvents) - 100
	if start < 0 {
		start = 0
	}
	writeJSON(w, http.StatusOK, h.recentEvents[start:])
}

func (h *Handler) handleSessions(w http.ResponseWriter, _ *http.Request) {
	sessions := h.Kernel.ActiveSessions()
	type sessionInfo struct {
		ID string `json:"id"`
	}
	items := make([]sessionInfo, len(sessions))
	for i, id := range sessions {
		items[i] = sessionInfo{ID: id}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) handleMemory(w http.ResponseWriter, r *http.Request, sub string) {
	ctx := r.Context()
	sub = strings.TrimPrefix(sub, "/")

	// Read: GET /memory (list) or GET /memory/{id} (single).
	if r.Method == http.MethodGet {
		if sub == "" {
			h.handleMemoryList(w, r)
			return
		}
		entry, err := h.MemStore.Get(ctx, sub)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if entry.ID == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, entry)
		return
	}

	// Write: POST /memory — create or update a memory entry. This is the
	// "记忆编辑器" surface from the architecture diagram.
	if r.Method == http.MethodPost && sub == "" {
		h.handleMemoryUpsert(w, r)
		return
	}

	// Delete: DELETE /memory/{id}.
	if r.Method == http.MethodDelete && sub != "" {
		if err := h.MemStore.Delete(ctx, sub); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"ok": "true", "id": sub})
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleMemoryList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	typeFilter := r.URL.Query().Get("type")
	f := memory.Filter{Limit: 500}
	if typeFilter != "" {
		f.Types = []memory.Type{memory.Type(typeFilter)}
	}
	entries, err := h.MemStore.Query(ctx, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	count, _ := h.MemStore.Count(ctx, f)
	writeJSON(w, http.StatusOK, map[string]any{
		"total":   count,
		"entries": entries,
	})
}

func (h *Handler) handleMemoryUpsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string         `json:"id"`
		Type      string         `json:"type"`
		Domain    string         `json:"domain,omitempty"`
		Title     string         `json:"title"`
		Content   string         `json:"content"`
		SessionID string         `json:"sessionId,omitempty"`
		CWD       string         `json:"cwd,omitempty"`
		Metadata  map[string]any `json:"metadata,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = "dashboard-" + time.Now().UTC().Format("20060102T150405.000000000")
	}
	if req.Type == "" {
		req.Type = string(memory.TypeShortTerm)
	}
	now := time.Now().UTC()
	entry := memory.Entry{
		ID:        req.ID,
		Type:      memory.Type(req.Type),
		Domain:    memory.Domain(req.Domain),
		Title:     req.Title,
		Content:   req.Content,
		Source:    memory.SourceUser,
		SessionID: req.SessionID,
		CWD:       req.CWD,
		Metadata:  req.Metadata,
		UpdatedAt: now,
	}
	// Preserve original CreatedAt when updating an existing entry.
	existing, _ := h.MemStore.Get(r.Context(), req.ID)
	if !existing.CreatedAt.IsZero() {
		entry.CreatedAt = existing.CreatedAt
	} else {
		entry.CreatedAt = now
	}
	if err := h.MemStore.Upsert(r.Context(), entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *Handler) handleDecisions(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	writeJSON(w, http.StatusOK, h.decisions)
}

func (h *Handler) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan eventbus.Envelope, 64)
	sub := h.Bus.Subscribe("dashboard-sse", eventbus.Filter{}, func(env eventbus.Envelope) {
		select {
		case ch <- env:
		default:
		}
	})
	defer sub.Close()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case env := <-ch:
			data, _ := json.Marshal(env)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
