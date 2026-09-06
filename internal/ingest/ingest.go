package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/skndan/sentry-lite/internal/bus"
	"github.com/skndan/sentry-lite/internal/store"
)

type Handler struct {
	Store     *store.Store
	Bus       *bus.Bus
	IngestRPS int
	limiters  sync.Map
}

type IngestMessage struct {
	ProjectID   int64           `json:"project_id"`
	EventID     string          `json:"event_id"`
	Kind        string          `json:"kind"` // error | transaction | log
	Payload     json.RawMessage `json:"payload"`
	Environment string          `json:"environment,omitempty"`
	Release     string          `json:"release,omitempty"`
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/api/{projectID}/envelope/", h.HandleEnvelope)
	r.Post("/api/{projectID}/envelope", h.HandleEnvelope)
	r.Post("/api/{projectID}/store/", h.HandleStore)
	r.Post("/api/{projectID}/store", h.HandleStore)
}

func (h *Handler) HandleEnvelope(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project", http.StatusBadRequest)
		return
	}
	if !h.authorize(r, projectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !h.allowIngest(projectID) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	body, err := readIngestBody(r)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	parsed, err := parseEnvelope(body)
	if err != nil {
		// Non-event envelopes (session, client_report, etc.) — ack without enqueue
		log.Printf("envelope skip: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
		return
	}

	ackID := parsed.EventID
	enqueued := 0
	for _, it := range parsed.Items {
		switch it.Type {
		case "event", "transaction":
			kind := "error"
			if it.Type == "transaction" {
				kind = "transaction"
			}
			payload := json.RawMessage(bytes.Clone(it.Payload))
			eventID := ""
			var m map[string]any
			if json.Unmarshal(payload, &m) == nil {
				if id, ok := m["event_id"].(string); ok {
					eventID = strings.ReplaceAll(id, "-", "")
				}
				if it.Type == "event" && kindFromPayload(m) == "transaction" {
					kind = "transaction"
				}
			}
			if eventID == "" {
				eventID = newEventID()
				if m == nil {
					m = map[string]any{}
				}
				m["event_id"] = eventID
				if b, err := json.Marshal(m); err == nil {
					payload = b
				}
			}
			if err := h.enqueue(r.Context(), projectID, eventID, kind, payload, "", ""); err != nil {
				log.Printf("enqueue: %v", err)
				http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
				return
			}
			if ackID == "" {
				ackID = eventID
			}
			enqueued++
		case "log":
			eventID := newEventID()
			if err := h.enqueue(r.Context(), projectID, eventID, "log", json.RawMessage(bytes.Clone(it.Payload)), parsed.Environment, parsed.Release); err != nil {
				log.Printf("enqueue: %v", err)
				http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
				return
			}
			if ackID == "" {
				ackID = eventID
			}
			enqueued++
		}
	}
	if enqueued == 0 {
		log.Printf("envelope skip: %v", errInvalidEnvelope)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
		return
	}
	if ackID == "" {
		ackID = newEventID()
	}
	writeEventID(w, ackID)
}

func (h *Handler) HandleStore(w http.ResponseWriter, r *http.Request) {
	projectID, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project", http.StatusBadRequest)
		return
	}
	if !h.authorize(r, projectID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !h.allowIngest(projectID) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	body, err := readIngestBody(r)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	eventID, _ := m["event_id"].(string)
	eventID = strings.ReplaceAll(eventID, "-", "")
	if eventID == "" {
		eventID = strings.ReplaceAll(uuid.NewString(), "-", "")
		m["event_id"] = eventID
		body, _ = json.Marshal(m)
	}

	if err := h.enqueue(r.Context(), projectID, eventID, kindFromPayload(m), body, "", ""); err != nil {
		log.Printf("enqueue: %v", err)
		http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
		return
	}
	writeEventID(w, eventID)
}

func (h *Handler) enqueue(ctx context.Context, projectID int64, eventID, kind string, payload []byte, environment, release string) error {
	if kind == "" {
		kind = "error"
	}
	msg := IngestMessage{
		ProjectID:   projectID,
		EventID:     eventID,
		Kind:        kind,
		Payload:     payload,
		Environment: environment,
		Release:     release,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return h.Bus.Produce(ctx, []byte(eventID), b)
}

func (h *Handler) authorize(r *http.Request, projectID int64) bool {
	key := extractPublicKey(r)
	if key == "" {
		return false
	}
	pk, err := h.Store.LookupProjectKey(r.Context(), key, projectID)
	if err != nil || pk == nil {
		return false
	}
	return true
}

func extractPublicKey(r *http.Request) string {
	auth := r.Header.Get("X-Sentry-Auth")
	if auth == "" {
		auth = r.URL.Query().Get("sentry_key")
		if auth != "" {
			return auth
		}
		// Authorization: sentry ... or Bearer-like
		if a := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(a), "sentry ") {
			auth = a[7:]
		}
	}
	for _, part := range strings.Split(auth, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "sentry_key=") {
			return strings.Trim(strings.TrimPrefix(part, "sentry_key="), `"'`)
		}
	}
	if strings.Contains(auth, "sentry_key=") {
		for _, part := range strings.Fields(auth) {
			if strings.HasPrefix(part, "sentry_key=") {
				return strings.Trim(strings.TrimPrefix(part, "sentry_key="), `"',`)
			}
		}
	}
	return ""
}

type envelopeItem struct {
	Type    string
	Payload []byte
}

type parsedEnvelope struct {
	EventID     string
	Environment string
	Release     string
	Items       []envelopeItem
}

func parseEnvelope(body []byte) (*parsedEnvelope, error) {
	// Envelope: <header>\n then repeating <item_header>\n<payload>\n
	offset := 0
	idx := bytes.IndexByte(body[offset:], '\n')
	if idx < 0 {
		return nil, errInvalidEnvelope
	}
	headerLine := body[offset : offset+idx]
	offset += idx + 1

	out := &parsedEnvelope{}
	out.EventID, out.Environment, out.Release = envelopeHeaderMeta(headerLine)

	for offset < len(body) {
		if body[offset] == '\n' {
			offset++
			continue
		}
		idx = bytes.IndexByte(body[offset:], '\n')
		if idx < 0 {
			break
		}
		itemHeader := body[offset : offset+idx]
		offset += idx + 1
		var itemHdr map[string]any
		if err := json.Unmarshal(itemHeader, &itemHdr); err != nil {
			continue
		}
		typ, _ := itemHdr["type"].(string)
		length := 0
		switch v := itemHdr["length"].(type) {
		case float64:
			length = int(v)
		}

		var payload []byte
		if length > 0 {
			end := offset + length
			if end > len(body) {
				return nil, errInvalidEnvelope
			}
			payload = body[offset:end]
			offset = end
			if offset < len(body) && body[offset] == '\n' {
				offset++
			}
		} else {
			idx = bytes.IndexByte(body[offset:], '\n')
			if idx < 0 {
				payload = body[offset:]
				offset = len(body)
			} else {
				payload = body[offset : offset+idx]
				offset += idx + 1
			}
		}
		out.Items = append(out.Items, envelopeItem{Type: typ, Payload: bytes.Clone(payload)})
	}
	if len(out.Items) == 0 {
		return nil, errInvalidEnvelope
	}
	return out, nil
}

func envelopeHeaderMeta(header []byte) (eventID, env, release string) {
	var m map[string]any
	if json.Unmarshal(header, &m) != nil {
		return "", "", ""
	}
	if id, ok := m["event_id"].(string); ok {
		eventID = strings.ReplaceAll(id, "-", "")
	}
	env = headerString(m["environment"])
	release = headerString(m["release"])
	if trace, ok := m["trace"].(map[string]any); ok {
		if env == "" {
			env = headerString(trace["environment"])
		}
		if release == "" {
			release = headerString(trace["release"])
		}
	}
	return eventID, env, release
}

func headerString(v any) string {
	s, _ := v.(string)
	return s
}

func newEventID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func kindFromPayload(m map[string]any) string {
	if t, ok := m["type"].(string); ok && t == "transaction" {
		return "transaction"
	}
	if _, ok := m["transaction"].(string); ok {
		if _, hasEx := m["exception"]; !hasEx {
			return "transaction"
		}
	}
	return "error"
}

var errInvalidEnvelope = &parseError{"no ingestible item"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func writeEventID(w http.ResponseWriter, eventID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": eventID})
}
