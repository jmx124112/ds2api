package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

<<<<<<< HEAD:internal/adapter/openai/models_route_test.go
func TestGetModelRouteDirectOnly(t *testing.T) {
	h := &Handler{}
=======
func TestGetModelRouteDirectAndAlias(t *testing.T) {
	h := &openAITestSurface{}
>>>>>>> upstream/main:internal/httpapi/openai/models_route_test.go
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	t.Run("direct", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-flash", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

<<<<<<< HEAD:internal/adapter/openai/models_route_test.go
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
=======
	t.Run("direct_nothinking", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-flash-nothinking", nil)
>>>>>>> upstream/main:internal/httpapi/openai/models_route_test.go
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

<<<<<<< HEAD:internal/adapter/openai/models_route_test.go
	t.Run("third_party_alias_rejected", func(t *testing.T) {
=======
	t.Run("direct_expert", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-pro", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("direct_vision", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/deepseek-v4-vision", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("alias", func(t *testing.T) {
>>>>>>> upstream/main:internal/httpapi/openai/models_route_test.go
		req := httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4.1", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for alias, got %d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("alias_nothinking", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/models/claude-sonnet-4-6-nothinking", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for nothinking alias, got %d body=%s", rec.Code, rec.Body.String())
		}
	})
}

func TestGetModelRouteNotFound(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/not-exists", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
