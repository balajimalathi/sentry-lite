package process

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skndan/sentry-lite/internal/alerts"
	"github.com/skndan/sentry-lite/internal/bus"
	"github.com/skndan/sentry-lite/internal/fingerprint"
	"github.com/skndan/sentry-lite/internal/ingest"
	"github.com/skndan/sentry-lite/internal/store"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Worker struct {
	Store   *store.Store
	Bus     *bus.Bus
	DataDir string
	Alerts  *alerts.Dispatcher
}

func (w *Worker) Run(ctx context.Context) {
	log.Println("processor started")
	for {
		if ctx.Err() != nil {
			return
		}
		fetches := w.Bus.Consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if e.Err == context.Canceled || e.Err == context.DeadlineExceeded {
					return
				}
				log.Printf("poll error: %v", e)
			}
		}
		fetches.EachRecord(func(r *kgo.Record) {
			if err := w.handle(ctx, r.Value); err != nil {
				log.Printf("process error: %v", err)
				return
			}
			log.Printf("processed event offset=%d", r.Offset)
		})
		if err := w.Bus.Consumer.CommitUncommittedOffsets(ctx); err != nil {
			log.Printf("commit: %v", err)
		}
	}
}

func (w *Worker) handle(ctx context.Context, value []byte) error {
	var msg ingest.IngestMessage
	if err := json.Unmarshal(value, &msg); err != nil {
		return err
	}

	rawPath, err := w.writePayload(msg.ProjectID, msg.EventID, msg.Payload)
	if err != nil {
		return err
	}

	norm, err := Normalize(msg.Payload)
	if err != nil {
		return err
	}
	if norm.EventID == "" {
		norm.EventID = msg.EventID
	}

	frames := make([]fingerprint.Frame, 0, len(norm.Frames))
	for _, f := range norm.Frames {
		frames = append(frames, fingerprint.Frame{
			Filename: f.Filename,
			Function: f.Function,
			AbsPath:  f.AbsPath,
			Module:   f.Module,
			InApp:    f.InApp,
		})
	}

	fp := fingerprint.Compute(norm.Fingerprint, norm.ExceptionType, norm.Message, frames)
	culprit := norm.Culprit
	if culprit == "" {
		culprit = fingerprint.Culprit(frames)
	}
	title := norm.Title
	if title == "" {
		if norm.ExceptionType != "" && norm.Message != "" {
			title = norm.ExceptionType + ": " + truncate(norm.Message, 120)
		} else if norm.ExceptionType != "" {
			title = norm.ExceptionType
		} else if norm.Message != "" {
			title = truncate(norm.Message, 160)
		} else {
			title = "Untitled"
		}
	}

	summary, _ := json.Marshal(map[string]any{
		"exception_type": norm.ExceptionType,
		"message":        norm.Message,
		"culprit":        culprit,
		"frames":         norm.Frames,
		"user":           norm.User,
		"request":        norm.Request,
		"tags":           norm.Tags,
		"breadcrumbs":    norm.Breadcrumbs,
	})

	result, err := w.Store.UpsertEvent(ctx, store.UpsertEventInput{
		EventID:       norm.EventID,
		ProjectID:     msg.ProjectID,
		Fingerprint:   fp,
		Title:         title,
		Culprit:       culprit,
		Level:         norm.Level,
		Timestamp:     norm.Timestamp,
		Environment:   norm.Environment,
		Release:       norm.Release,
		Platform:      norm.Platform,
		Message:       norm.Message,
		ExceptionType: norm.ExceptionType,
		UserID:        norm.User.ID,
		UserEmail:     norm.User.Email,
		RawPath:       rawPath,
		PayloadJSON:   string(summary),
		Tags:          norm.Tags,
	})
	if err != nil {
		return err
	}

	if norm.Release != "" {
		if _, err := w.Store.UpsertRelease(ctx, msg.ProjectID, norm.Release, "", ""); err != nil {
			log.Printf("upsert release: %v", err)
		}
	}

	if w.Alerts != nil && result != nil {
		base := alerts.Event{
			ProjectID: msg.ProjectID,
			IssueID:   result.IssueID,
			Title:     title,
			Culprit:   culprit,
			Summary:   truncate(norm.Message, 200),
		}
		if result.IsNew {
			base.Trigger = "new_issue"
			w.Alerts.Handle(ctx, base)
		}
		if result.Regressed {
			base.Trigger = "regressed_issue"
			w.Alerts.Handle(ctx, base)
		}
		base.Trigger = "error_volume"
		w.Alerts.Handle(ctx, base)
	}
	return nil
}

func (w *Worker) writePayload(projectID int64, eventID string, payload []byte) (string, error) {
	dir := filepath.Join(w.DataDir, "events", fmt.Sprintf("%d", projectID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, eventID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

type Normalized struct {
	EventID       string
	Timestamp     time.Time
	Platform      string
	Environment   string
	Release       string
	Level         string
	Logger        string
	Message       string
	ExceptionType string
	Culprit       string
	Title         string
	Fingerprint   []string
	Tags          map[string]string
	Frames        []Frame
	User          User
	Request       map[string]any
	Breadcrumbs   []any
}

type Frame struct {
	Filename string `json:"filename"`
	Function string `json:"function"`
	AbsPath  string `json:"abs_path"`
	Module   string `json:"module"`
	LineNo   int    `json:"lineno"`
	ColNo    int    `json:"colno"`
	InApp    bool   `json:"in_app"`
	Context  []any  `json:"context_line,omitempty"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Username string `json:"username"`
}

func Normalize(payload []byte) (*Normalized, error) {
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	n := &Normalized{
		Tags:  map[string]string{},
		Level: "error",
	}
	if id, ok := raw["event_id"].(string); ok {
		n.EventID = strings.ReplaceAll(id, "-", "")
	}
	n.Timestamp = parseTimestamp(raw["timestamp"])
	n.Platform = asString(raw["platform"])
	n.Environment = asString(raw["environment"])
	n.Release = asString(raw["release"])
	if lvl := asString(raw["level"]); lvl != "" {
		n.Level = lvl
	}
	n.Logger = asString(raw["logger"])
	n.Culprit = asString(raw["culprit"])
	n.Message = extractMessage(raw["message"])

	if tags, ok := raw["tags"].(map[string]any); ok {
		for k, v := range tags {
			n.Tags[k] = fmt.Sprint(v)
		}
	} else if tagList, ok := raw["tags"].([]any); ok {
		for _, item := range tagList {
			if pair, ok := item.([]any); ok && len(pair) >= 2 {
				n.Tags[fmt.Sprint(pair[0])] = fmt.Sprint(pair[1])
			}
		}
	}
	if n.Environment == "" {
		n.Environment = n.Tags["environment"]
	}
	if n.Release == "" {
		n.Release = n.Tags["release"]
	}

	if fp, ok := raw["fingerprint"].([]any); ok {
		for _, p := range fp {
			n.Fingerprint = append(n.Fingerprint, fmt.Sprint(p))
		}
	}

	if user, ok := raw["user"].(map[string]any); ok {
		n.User.ID = asString(user["id"])
		n.User.Email = asString(user["email"])
		n.User.Username = asString(user["username"])
		if n.User.ID != "" {
			n.Tags["user.id"] = n.User.ID
		}
	}
	if req, ok := raw["request"].(map[string]any); ok {
		n.Request = req
	}
	if bc, ok := raw["breadcrumbs"].(map[string]any); ok {
		if vals, ok := bc["values"].([]any); ok {
			n.Breadcrumbs = vals
		}
	} else if bc, ok := raw["breadcrumbs"].([]any); ok {
		n.Breadcrumbs = bc
	}

	n.ExceptionType, n.Message, n.Frames = extractException(raw)
	return n, nil
}

func extractException(raw map[string]any) (exType, message string, frames []Frame) {
	ex, ok := raw["exception"].(map[string]any)
	if !ok {
		return "", asString(raw["message"]), nil
	}
	values, ok := ex["values"].([]any)
	if !ok || len(values) == 0 {
		return "", "", nil
	}
	// Use the last (innermost / most relevant) exception
	last, ok := values[len(values)-1].(map[string]any)
	if !ok {
		return "", "", nil
	}
	exType = asString(last["type"])
	message = asString(last["value"])
	if st, ok := last["stacktrace"].(map[string]any); ok {
		if fvals, ok := st["frames"].([]any); ok {
			for _, fv := range fvals {
				fm, ok := fv.(map[string]any)
				if !ok {
					continue
				}
				frames = append(frames, Frame{
					Filename: asString(fm["filename"]),
					Function: asString(fm["function"]),
					AbsPath:  asString(fm["abs_path"]),
					Module:   asString(fm["module"]),
					LineNo:   asInt(fm["lineno"]),
					ColNo:    asInt(fm["colno"]),
					InApp:    asBool(fm["in_app"]),
				})
			}
		}
	}
	return exType, message, frames
}

func extractMessage(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if m := asString(t["formatted"]); m != "" {
			return m
		}
		return asString(t["message"])
	default:
		return ""
	}
}

func parseTimestamp(v any) time.Time {
	switch t := v.(type) {
	case float64:
		sec := int64(t)
		nsec := int64((t - float64(sec)) * 1e9)
		return time.Unix(sec, nsec).UTC()
	case string:
		if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return ts.UTC()
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%v", t)
	case json.Number:
		return t.String()
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func asBool(v any) bool {
	b, _ := v.(bool)
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
