package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

	r.Get("/api/internal/releases", h.ListReleases)
	r.Post("/api/internal/releases", h.CreateRelease)

	r.Get("/api/internal/alerts", h.ListAlerts)
	r.Post("/api/internal/alerts", h.CreateAlert)

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
	tagKey, tagVal := q.Get("tag_key"), q.Get("tag_value")
	if tag := q.Get("tag"); tag != "" && tagKey == "" {
		if i := strings.IndexByte(tag, ':'); i > 0 {
			tagKey, tagVal = tag[:i], tag[i+1:]
		}
	}
	issues, err := h.Store.ListIssues(r.Context(), store.IssueListFilter{
		ProjectID:   projectID,
		Environment: q.Get("environment"),
		Release:     q.Get("release"),
		Query:       q.Get("q"),
		TagKey:      tagKey,
		TagValue:    tagVal,
		From:        q.Get("from"),
		To:          q.Get("to"),
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
		Status   *string `json:"status"`
		Assignee *string `json:"assignee"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Status != nil {
		switch *body.Status {
		case "open", "resolved", "ignored":
		default:
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}
		if err := h.Store.UpdateIssueStatus(r.Context(), id, *body.Status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if body.Assignee != nil {
		if err := h.Store.UpdateIssueAssignee(r.Context(), id, *body.Assignee); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	iss, err := h.Store.GetIssue(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, iss)
}

func (h *Handler) ListReleases(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID, _ = strconv.ParseInt(v, 10, 64)
	}
	if projectID <= 0 {
		http.Error(w, "project_id required", http.StatusBadRequest)
		return
	}
	releases, err := h.Store.ListReleases(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if releases == nil {
		releases = []store.Release{}
	}
	writeJSON(w, releases)
}

func (h *Handler) CreateRelease(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID int64  `json:"project_id"`
		Version   string `json:"version"`
		Ref       string `json:"ref"`
		URL       string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.ProjectID <= 0 || body.Version == "" {
		http.Error(w, "project_id and version required", http.StatusBadRequest)
		return
	}
	rel, err := h.Store.UpsertRelease(r.Context(), body.ProjectID, body.Version, body.Ref, body.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, rel)
}

func (h *Handler) ListAlerts(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID, _ = strconv.ParseInt(v, 10, 64)
	}
	rules, err := h.Store.ListAlertRules(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		rules = []store.AlertRule{}
	}
	writeJSON(w, rules)
}

func (h *Handler) CreateAlert(w http.ResponseWriter, r *http.Request) {
	var body store.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	switch body.Trigger {
	case "new_issue", "regressed_issue", "error_volume":
	default:
		http.Error(w, "invalid trigger", http.StatusBadRequest)
		return
	}
	switch body.Channel {
	case "slack", "email", "webhook":
	default:
		http.Error(w, "invalid channel", http.StatusBadRequest)
		return
	}
	if body.ProjectID <= 0 || body.Name == "" || body.Target == "" {
		http.Error(w, "project_id, name, target required", http.StatusBadRequest)
		return
	}
	body.Enabled = true
	rule, err := h.Store.CreateAlertRule(r.Context(), body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, rule)
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
