package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/skndan/sentry-lite/internal/store"
)

type Handler struct {
	Store *store.Store
}

func (h *Handler) Routes(r chi.Router) {
	r.Get("/api/internal/projects", h.ListProjects)
	r.Get("/api/internal/issues", h.ListIssues)
	r.Get("/api/internal/issues/{id}", h.GetIssue)
	r.Get("/api/internal/issues/{id}/events", h.ListEvents)
	r.Patch("/api/internal/issues/{id}", h.UpdateIssue)
	r.Get("/api/internal/meta", h.Meta)
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.Store.ListProjects(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []store.Project{}
	}
	writeJSON(w, projects)
}

func (h *Handler) ListIssues(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var projectID int64
	if v := q.Get("project_id"); v != "" {
		projectID, _ = strconv.ParseInt(v, 10, 64)
	}
	issues, err := h.Store.ListIssues(r.Context(), store.IssueListFilter{
		ProjectID:   projectID,
		Environment: q.Get("environment"),
		Release:     q.Get("release"),
		Query:       q.Get("q"),
		Limit:       100,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if issues == nil {
		issues = []store.Issue{}
	}
	writeJSON(w, issues)
}

func (h *Handler) GetIssue(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	iss, err := h.Store.GetIssue(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if iss == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	latest, err := h.Store.GetLatestEvent(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"issue":        iss,
		"latest_event": latest,
	})
}

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	events, err := h.Store.ListEventsForIssue(r.Context(), id, 50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if events == nil {
		events = []store.Event{}
	}
	writeJSON(w, events)
}

func (h *Handler) UpdateIssue(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch body.Status {
	case "open", "resolved", "ignored":
	default:
		http.Error(w, "invalid status", http.StatusBadRequest)
		return
	}
	if err := h.Store.UpdateIssueStatus(r.Context(), id, body.Status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	iss, err := h.Store.GetIssue(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, iss)
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"seed_public_key": store.SeedPublicKey,
		"seed_dsn_hint":   "http://" + store.SeedPublicKey + "@localhost:8080/1",
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
