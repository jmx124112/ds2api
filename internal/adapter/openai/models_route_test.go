package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGetModelRouteDirectOnly(t *testing.T) {
	h := &Handler{}
	r := chi.NewRouter()
	RegisterRoutes(r, h)

	t.Run("direct", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-flash", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("direct_pro", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-pro", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("legacy_alias_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-chat", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("vision_alias_accepted", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-vision-chat", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for vision alias, got %d body=%s", rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); !strings.Contains(body, `"id":"deepseek-vision-chat"`) {
			t.Fatalf("expected returned model id deepseek-vision-chat, got body=%s", body)
		}
	})

	t.Run("third_party_alias_rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4.1", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for alias, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestGetModelRouteNotFound(t *testing.T) {
	h := &Handler{}
	r := chi.NewRouter()
	RegisterRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/not-exists", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
