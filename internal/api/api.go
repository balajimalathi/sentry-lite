package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skndan/sentry-lite/internal/alerts"
	"github.com/skndan/sentry-lite/internal/auth"
	"github.com/skndan/sentry-lite/internal/store"
)

type Handler struct {
	Store      *store.Store
	PublicURL  string
	AdminToken string
}

func (h *Handler) Routes(r chi.Router) {
	r.Route("/api/internal", func(r chi.Router) {
		if h.AdminToken != "" {
			r.Use(auth.RequireAdmin(h.AdminToken))
		}
		r.Get("/projects", h.ListProjects)
		r.Post("/projects", h.CreateProject)
		r.Get("/facets", h.ListFacets)
		r.Get("/issues", h.ListIssues)
		r.Get("/issues/{id}", h.GetIssue)
		r.Get("/issues/{id}/events", h.ListEvents)
		r.Patch("/issues/{id}", h.UpdateIssue)

		r.Get("/releases", h.ListReleases)
		r.Post("/releases", h.CreateRelease)

		r.Get("/alerts", h.ListAlerts)
		r.Post("/alerts", h.CreateAlert)
		r.Delete("/alerts/{id}", h.DeleteAlert)

		r.Get("/transactions", h.ListTransactions)
		r.Get("/transaction", h.GetTransaction)
		r.Get("/traces/{traceID}", h.GetTrace)

		r.Get("/crons", h.ListCrons)
		r.Post("/crons", h.CreateCron)
		r.Patch("/crons/{id}", h.UpdateCron)
		r.Delete("/crons/{id}", h.DeleteCron)

		r.Get("/stats", h.GetStats)

		r.Get("/meta", h.Meta)
	})

	// Heartbeat check-in (URL token-authenticated; not admin token)
	r.Post("/api/cron/check-in/{token}", h.CronCheckIn)
	r.Post("/api/cron/check-in/{token}/", h.CronCheckIn)
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

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name           string   `json:"name"`
		Slug           string   `json:"slug"`
		AllowedOrigins []string `json:"allowed_origins"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	created, err := h.Store.CreateProject(r.Context(), body.Name, body.Slug, h.PublicURL, body.AllowedOrigins)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, created)
}

func (h *Handler) ListFacets(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID, _ = strconv.ParseInt(v, 10, 64)
	}
	facets, err := h.Store.ListFacets(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, facets)
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
	case "new_issue", "regressed_issue", "error_volume", "cron_missed":
	default:
		http.Error(w, "invalid trigger", http.StatusBadRequest)
		return
	}
	switch body.Channel {
	case "slack", "email", "webhook", "telegram":
	default:
		http.Error(w, "invalid channel", http.StatusBadRequest)
		return
	}
	if body.ProjectID <= 0 || body.Name == "" || body.Target == "" {
		http.Error(w, "project_id, name, target required", http.StatusBadRequest)
		return
	}
	if body.Channel == "telegram" {
		if err := alerts.SendTelegramConnectTest(r.Context(), body.Target); err != nil {
			http.Error(w, "telegram connect failed: "+err.Error(), http.StatusBadRequest)
			return
		}
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

func (h *Handler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.Store.DeleteAlertRule(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"ok": true})
}

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	projectID, _ := strconv.ParseInt(q.Get("project_id"), 10, 64)
	if projectID <= 0 {
		http.Error(w, "project_id required", http.StatusBadRequest)
		return
	}
	from := time.Now().UTC().Add(-24 * time.Hour)
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			from = t
		} else if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	list, err := h.Store.ListTransactionSummaries(r.Context(), projectID, from)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []store.TransactionSummary{}
	}
	writeJSON(w, list)
}

func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	projectID, _ := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if projectID <= 0 {
		http.Error(w, "project_id required", http.StatusBadRequest)
		return
	}
	samples, err := h.Store.GetTransactionDetail(r.Context(), projectID, name, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if samples == nil {
		samples = []store.Transaction{}
	}
	from := time.Now().UTC().Add(-24 * time.Hour)
	summaries, _ := h.Store.ListTransactionSummaries(r.Context(), projectID, from)
	var summary *store.TransactionSummary
	for i := range summaries {
		if summaries[i].Name == name {
			summary = &summaries[i]
			break
		}
	}
	writeJSON(w, map[string]any{
		"name":    name,
		"summary": summary,
		"samples": samples,
	})
}

func (h *Handler) GetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceID")
	if traceID == "" {
		http.Error(w, "bad trace id", http.StatusBadRequest)
		return
	}
	detail, err := h.Store.GetTrace(r.Context(), traceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if detail == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, detail)
}

func (h *Handler) ListCrons(w http.ResponseWriter, r *http.Request) {
	var projectID int64
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID, _ = strconv.ParseInt(v, 10, 64)
	}
	list, err := h.Store.ListCronMonitors(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []store.CronMonitor{}
	}
	writeJSON(w, list)
}

func (h *Handler) CreateCron(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProjectID   int64  `json:"project_id"`
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		ScheduleSec int64  `json:"schedule_sec"`
		GraceSec    int64  `json:"grace_sec"`
		Environment string `json:"environment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	m, err := h.Store.CreateCronMonitor(r.Context(), store.CreateCronInput{
		ProjectID:   body.ProjectID,
		Name:        body.Name,
		Slug:        body.Slug,
		ScheduleSec: body.ScheduleSec,
		GraceSec:    body.GraceSec,
		Environment: body.Environment,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, m)
}

func (h *Handler) UpdateCron(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	var body struct {
		Name        string `json:"name"`
		ScheduleSec int64  `json:"schedule_sec"`
		GraceSec    int64  `json:"grace_sec"`
		Environment string `json:"environment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	m, err := h.Store.UpdateCronMonitor(r.Context(), id, body.Name, body.ScheduleSec, body.GraceSec, body.Environment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, m)
}

func (h *Handler) DeleteCron(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	if err := h.Store.DeleteCronMonitor(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CronCheckIn(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	m, err := h.Store.GetCronMonitorByToken(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	status := "ok"
	var durationMS *float64
	if r.Body != nil && r.ContentLength != 0 {
		var body struct {
			Status     string   `json:"status"`
			DurationMS *float64 `json:"duration_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			if body.Status != "" {
				status = body.Status
			}
			durationMS = body.DurationMS
		}
	}
	updated, err := h.Store.RecordCronCheckin(r.Context(), m.ID, status, durationMS)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, updated)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var projectID int64
	if v := q.Get("project_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id < 0 {
			http.Error(w, "invalid project_id", http.StatusBadRequest)
			return
		}
		projectID = id
	}

	now := time.Now().UTC()
	to := now
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse(time.RFC3339Nano, v)
		}
		if err != nil {
			http.Error(w, "invalid to", http.StatusBadRequest)
			return
		}
		to = t.UTC()
	}
	from := to.Add(-24 * time.Hour)
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			t, err = time.Parse(time.RFC3339Nano, v)
		}
		if err != nil {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
		from = t.UTC()
	}

	interval, err := store.ParseStatsInterval(q.Get("interval"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stats, err := h.Store.DashboardStats(r.Context(), store.DashboardStatsFilter{
		ProjectID: projectID,
		From:      from,
		To:        to,
		Interval:  interval,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
