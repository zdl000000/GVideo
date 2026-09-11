package httpapi

import (
	"net/http"
)

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	created, err := h.service.Register(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, created.Token)
	w.Header().Set("X-CSRF-Token", created.CSRFToken)
	writeJSON(w, r, http.StatusCreated, map[string]any{"user": created.User, "csrf_token": created.CSRFToken})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	created, err := h.service.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, created.Token)
	w.Header().Set("X-CSRF-Token", created.CSRFToken)
	writeJSON(w, r, http.StatusOK, map[string]any{"user": created.User, "csrf_token": created.CSRFToken})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			h.writeError(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	writeJSON(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	session := sessionFrom(r.Context())
	writeJSON(w, r, http.StatusOK, map[string]any{"user": session.User, "csrf_token": session.CSRFToken})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		writeProblem(w, r, http.StatusUnauthorized, "Authentication required")
		return
	}
	if err := h.service.ChangePassword(r.Context(), sessionFrom(r.Context()).User.ID, cookie.Value, input.CurrentPassword, input.NewPassword); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"changed": true})
}
