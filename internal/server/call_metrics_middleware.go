package server

import (
	"net/http"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"ds2api/internal/chathistory"
	"ds2api/internal/config"
)

const (
	metricsExcludeSourceHeader = "X-Ds2-Source"
	metricsExcludeSourceValue  = "admin-webui-api-tester"
)

var trackedBusinessPaths = map[string]struct{}{
	"/v1/chat/completions":                {},
	"/v1/responses":                       {},
	"/v1/files":                           {},
	"/v1/embeddings":                      {},
	"/anthropic/v1/messages":              {},
	"/anthropic/v1/messages/count_tokens": {},
	"/v1/messages":                        {},
	"/messages":                           {},
	"/v1/messages/count_tokens":           {},
	"/messages/count_tokens":              {},
}

func businessCallMetrics(store *chathistory.Store) func(http.Handler) http.Handler {
	if store == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !shouldRecordBusinessCall(r) {
				next.ServeHTTP(w, r)
				return
			}

			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			if err := store.RecordCall(status); err != nil {
				config.Logger.Warn("[chat_history] record call metric failed", "path", strings.TrimSpace(r.URL.Path), "status", status, "error", err)
			}
		})
	}
}

func shouldRecordBusinessCall(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodPost {
		return false
	}
	if strings.TrimSpace(r.Header.Get(metricsExcludeSourceHeader)) == metricsExcludeSourceValue {
		return false
	}
	path := strings.TrimSpace(r.URL.Path)
	if _, ok := trackedBusinessPaths[path]; ok {
		return true
	}
	return isGeminiBusinessPath(path)
}

func isGeminiBusinessPath(path string) bool {
	if strings.HasPrefix(path, "/v1beta/models/") || strings.HasPrefix(path, "/v1/models/") {
		return strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent")
	}
	return false
}
