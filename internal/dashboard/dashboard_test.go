package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mobilevc/internal/eventbus"
	"mobilevc/internal/feedback"
	"mobilevc/internal/memory"
)

type stubKernel struct{}

func (stubKernel) ActiveSessions() []string { return []string{"sess-A"} }
func (stubKernel) ConsoleExec(_ context.Context, sessionID, _, _ string) (string, error) {
	if sessionID == "" {
		return "new-sess", nil
	}
	return sessionID, nil
}

type stubFeedback struct{}

func (stubFeedback) Pending() []feedback.Suggestion                                 { return nil }
func (stubFeedback) Decide(context.Context, string, feedback.Decision, string) (feedback.Record, error) {
	return feedback.Record{}, nil
}
func (stubFeedback) History() []feedback.Record { return nil }
func (stubFeedback) Stats() feedback.Stats      { return feedback.Stats{} }

func newHandler(t *testing.T) (*Handler, memory.Store) {
	t.Helper()
	bus := eventbus.New()
	t.Cleanup(func() { _ = bus.Close() })
	store := memory.NewMemStore()
	return NewHandler(bus, store, stubKernel{}, stubFeedback{}), store
}

func do(h *Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, "/dashboard/"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMemoryUpsertGetDeleteCycle(t *testing.T) {
	h, store := newHandler(t)

	// Create.
	rec := do(h, http.MethodPost, "memory", map[string]any{
		"id": "u1", "type": "long_term", "title": "note", "content": "hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got, _ := store.Get(context.Background(), "u1"); got.Title != "note" {
		t.Fatalf("not persisted: %+v", got)
	}

	// Get single.
	rec = do(h, http.MethodGet, "memory/u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}
	var got memory.Entry
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.ID != "u1" {
		t.Fatalf("got=%+v", got)
	}

	// List.
	rec = do(h, http.MethodGet, "memory", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d", rec.Code)
	}

	// Update — title changes, CreatedAt preserved.
	rec = do(h, http.MethodPost, "memory", map[string]any{
		"id": "u1", "type": "long_term", "title": "note v2", "content": "hello",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("update status=%d", rec.Code)
	}
	updated, _ := store.Get(context.Background(), "u1")
	if updated.Title != "note v2" {
		t.Fatalf("title not updated: %+v", updated)
	}
	if updated.CreatedAt != got.CreatedAt {
		t.Fatalf("CreatedAt mutated: was %v now %v", got.CreatedAt, updated.CreatedAt)
	}

	// Delete.
	rec = do(h, http.MethodDelete, "memory/u1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status=%d", rec.Code)
	}
	gone, _ := store.Get(context.Background(), "u1")
	if gone.ID != "" {
		t.Fatalf("entry not deleted: %+v", gone)
	}
}

func TestMemoryGetMissing(t *testing.T) {
	h, _ := newHandler(t)
	rec := do(h, http.MethodGet, "memory/does-not-exist", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestMemoryAutoIDOnUpsert(t *testing.T) {
	h, store := newHandler(t)
	rec := do(h, http.MethodPost, "memory", map[string]any{
		"title": "auto", "content": "x",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var entry memory.Entry
	_ = json.Unmarshal(rec.Body.Bytes(), &entry)
	if entry.ID == "" {
		t.Fatalf("ID not auto-assigned: %+v", entry)
	}
	got, _ := store.Get(context.Background(), entry.ID)
	if got.ID != entry.ID {
		t.Fatalf("entry not persisted")
	}
}
