package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/chathistory"
)

func newMetricsTestStore(t *testing.T) *chathistory.Store {
	t.Helper()
	store := chathistory.New(filepath.Join(t.TempDir(), "chat_history.db"))
	if err := store.Err(); err != nil {
		t.Fatalf("chat history store unavailable: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close chat history store failed: %v", err)
		}
	})
	return store
}

func TestBusinessCallMetricsCountsWhitelistedRequests(t *testing.T) {
	store := newMetricsTestStore(t)

	r := chi.NewRouter()
	r.Use(businessCallMetrics(store))
	r.Post("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/v1/models/{model}:generateContent", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	okReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", okRec.Code)
	}

	failReq := httptest.NewRequest(http.MethodPost, "/v1/models/gemini-2.5-pro:generateContent", nil)
	failRec := httptest.NewRecorder()
	r.ServeHTTP(failRec, failReq)
	if failRec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", failRec.Code)
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshot.Stats.TotalCalls != 2 || snapshot.Stats.SuccessCalls != 1 || snapshot.Stats.FailedCalls != 1 {
		t.Fatalf("unexpected stats: %#v", snapshot.Stats)
	}
}

func TestBusinessCallMetricsSkipsExcludedSourceAndNonTrackedRequests(t *testing.T) {
	store := newMetricsTestStore(t)

	r := chi.NewRouter()
	r.Use(businessCallMetrics(store))
	r.Post("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Get("/v1/chat/completions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/admin/login", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	excludedReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	excludedReq.Header.Set(metricsExcludeSourceHeader, metricsExcludeSourceValue)
	excludedRec := httptest.NewRecorder()
	r.ServeHTTP(excludedRec, excludedReq)
	if excludedRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", excludedRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get route, got %d", getRec.Code)
	}

	nonTrackedReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	nonTrackedRec := httptest.NewRecorder()
	r.ServeHTTP(nonTrackedRec, nonTrackedReq)
	if nonTrackedRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for non tracked route, got %d", nonTrackedRec.Code)
	}

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	if snapshot.Stats.TotalCalls != 0 || snapshot.Stats.SuccessCalls != 0 || snapshot.Stats.FailedCalls != 0 {
		t.Fatalf("expected all stats to remain zero, got %#v", snapshot.Stats)
	}
}
