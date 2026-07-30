package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/skndan/sentry-lite/internal/bus"
	"github.com/skndan/sentry-lite/internal/store"
)

type Handler struct {
	Store *store.Store
	Bus   *bus.Bus
}

type IngestMessage struct {
	ProjectID int64           `json:"project_id"`
	EventID   string          `json:"event_id"`
	Payload   json.RawMessage `json:"payload"`
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

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	eventJSON, eventID, err := extractEventFromEnvelope(body)
	if err != nil {
		log.Printf("envelope parse: %v", err)
		http.Error(w, "invalid envelope", http.StatusBadRequest)
		return
	}
	if eventID == "" {
		eventID = strings.ReplaceAll(uuid.NewString(), "-", "")
		var m map[string]any
		if json.Unmarshal(eventJSON, &m) == nil {
			m["event_id"] = eventID
			if b, err := json.Marshal(m); err == nil {
				eventJSON = b
			}
		}
	}

	if err := h.enqueue(r.Context(), projectID, eventID, eventJSON); err != nil {
		log.Printf("enqueue: %v", err)
		http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
		return
	}
	writeEventID(w, eventID)
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

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
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

	if err := h.enqueue(r.Context(), projectID, eventID, body); err != nil {
		log.Printf("enqueue: %v", err)
		http.Error(w, "enqueue failed", http.StatusServiceUnavailable)
		return
	}
	writeEventID(w, eventID)
}

func (h *Handler) enqueue(ctx context.Context, projectID int64, eventID string, payload []byte) error {
	msg := IngestMessage{ProjectID: projectID, EventID: eventID, Payload: payload}
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

func extractEventFromEnvelope(body []byte) (json.RawMessage, string, error) {
	// Envelope: <header>\n then repeating <item_header>\n<payload>\n
	offset := 0
	// skip envelope header line
	idx := bytes.IndexByte(body[offset:], '\n')
	if idx < 0 {
		return nil, "", errInvalidEnvelope
	}
	offset += idx + 1

	for offset < len(body) {
		if body[offset] == '\n' {
			offset++
			continue
		}
		idx = bytes.IndexByte(body[offset:], '\n')
		if idx < 0 {
			break
		}
		headerLine := body[offset : offset+idx]
		offset += idx + 1
		var itemHdr map[string]any
		if err := json.Unmarshal(headerLine, &itemHdr); err != nil {
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
				return nil, "", errInvalidEnvelope
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

		if typ == "event" {
			eventID := ""
			var m map[string]any
			if json.Unmarshal(payload, &m) == nil {
				if id, ok := m["event_id"].(string); ok {
					eventID = strings.ReplaceAll(id, "-", "")
				}
			}
			return json.RawMessage(bytes.Clone(payload)), eventID, nil
		}
	}
	return nil, "", errInvalidEnvelope
}

var errInvalidEnvelope = &parseError{"no event item"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

func writeEventID(w http.ResponseWriter, eventID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": eventID})
}
