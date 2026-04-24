package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	authn "ds2api/internal/auth"
	"ds2api/internal/config"
)

func (h *Handler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := authn.VerifyAdminRequestWithStore(r, h.Store); err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": err.Error()})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) bootstrap(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": strings.TrimSpace(h.Store.AdminPasswordHash()) == "",
	})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.Store.AdminPasswordHash()) == "" {
		writeJSON(w, http.StatusPreconditionRequired, map[string]any{
			"detail":         "admin password is not initialized",
			"setup_required": true,
		})
		return
	}

	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	adminKey, _ := req["admin_key"].(string)
	expireHours := intFrom(req["expire_hours"])
	if !authn.VerifyAdminCredential(adminKey, h.Store) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": "Invalid admin key"})
		return
	}
	token, err := authn.CreateJWTWithStore(expireHours, h.Store)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	if expireHours <= 0 {
		expireHours = h.Store.AdminJWTExpireHours()
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "token": token, "expires_in": expireHours * 3600})
}

func (h *Handler) setup(w http.ResponseWriter, r *http.Request) {
	if strings.TrimSpace(h.Store.AdminPasswordHash()) != "" {
		writeJSON(w, http.StatusConflict, map[string]any{
			"detail": "admin password has already been initialized",
		})
		return
	}

	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	password := strings.TrimSpace(fieldString(req, "password"))
	if password == "" {
		password = strings.TrimSpace(fieldString(req, "new_password"))
	}
	if len(password) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"detail": "password must be at least 4 characters",
		})
		return
	}

	now := time.Now().Unix()
	hash := authn.HashAdminPassword(password)
	if err := h.Store.Update(func(c *config.Config) error {
		if strings.TrimSpace(c.Admin.PasswordHash) != "" {
			return errors.New("admin password has already been initialized")
		}
		c.Admin.PasswordHash = hash
		c.Admin.JWTValidAfterUnix = now
		return nil
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already been initialized") {
			writeJSON(w, http.StatusConflict, map[string]any{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	token, err := authn.CreateJWTWithStore(0, h.Store)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	expireHours := h.Store.AdminJWTExpireHours()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":        true,
		"setup_required": false,
		"token":          token,
		"expires_in":     expireHours * 3600,
	})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": "No credentials provided"})
		return
	}
	token := strings.TrimSpace(header[7:])
	payload, err := authn.VerifyJWTWithStore(token, h.Store)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"detail": err.Error()})
		return
	}
	exp, _ := payload["exp"].(float64)
	remaining := int64(exp) - time.Now().Unix()
	if remaining < 0 {
		remaining = 0
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "expires_at": int64(exp), "remaining_seconds": remaining})
}

func (h *Handler) getVercelConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"has_token":  strings.TrimSpace(os.Getenv("VERCEL_TOKEN")) != "",
		"project_id": strings.TrimSpace(os.Getenv("VERCEL_PROJECT_ID")),
		"team_id":    nilIfEmpty(strings.TrimSpace(os.Getenv("VERCEL_TEAM_ID"))),
	})
}
