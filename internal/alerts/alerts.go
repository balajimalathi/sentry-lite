package alerts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/skndan/sentry-lite/internal/store"
)

type Dispatcher struct {
	Store   *store.Store
	APIBase string // e.g. http://localhost:8080 for issue links
	From    string // SMTP from
	SMTP    string // host:port, empty disables email
}

type Event struct {
	Trigger   string // new_issue | regressed_issue | error_volume
	ProjectID int64
	IssueID   int64
	Title     string
	Culprit   string
	Summary   string
}

func (d *Dispatcher) Handle(ctx context.Context, ev Event) {
	rules, err := d.Store.ListEnabledAlertRules(ctx, ev.ProjectID, ev.Trigger)
	if err != nil {
		log.Printf("alerts list: %v", err)
		return
	}
	for _, rule := range rules {
		if ev.Trigger == "error_volume" {
			since := time.Now().Add(-time.Duration(rule.WindowSec) * time.Second)
			n, err := d.Store.CountEventsSince(ctx, ev.ProjectID, since)
			if err != nil || n < rule.Threshold {
				continue
			}
			ev.Summary = fmt.Sprintf("%d events in last %ds (threshold %d)", n, rule.WindowSec, rule.Threshold)
		}
		if err := d.deliver(ctx, rule, ev); err != nil {
			log.Printf("alert deliver rule=%d: %v", rule.ID, err)
			_ = d.Store.RecordAlertDelivery(ctx, rule.ID, ev.IssueID, "error", err.Error())
		} else {
			_ = d.Store.RecordAlertDelivery(ctx, rule.ID, ev.IssueID, "ok", "")
		}
	}
}

func (d *Dispatcher) deliver(ctx context.Context, rule store.AlertRule, ev Event) error {
	base := strings.TrimRight(d.APIBase, "/")
	link := fmt.Sprintf("%s/issues/%d", base, ev.IssueID)
	if ev.Trigger == "cron_missed" || ev.IssueID == 0 {
		link = base + "/crons"
	}
	body := map[string]any{
		"rule":       rule.Name,
		"trigger":    ev.Trigger,
		"project_id": ev.ProjectID,
		"issue_id":   ev.IssueID,
		"title":      ev.Title,
		"culprit":    ev.Culprit,
		"summary":    ev.Summary,
		"issue_url":  link,
		"url":        link,
	}
	switch rule.Channel {
	case "slack":
		label := "View issue"
		if ev.Trigger == "cron_missed" {
			label = "View crons"
		}
		return d.slack(ctx, rule.Target, fmt.Sprintf("*%s*\n%s\n<%s|%s>", ev.Title, ev.Summary, link, label))
	case "email":
		return d.email(rule.Target, fmt.Sprintf("[sentry-lite] %s", ev.Title),
			fmt.Sprintf("%s\n%s\n%s\n", ev.Title, ev.Summary, link))
	case "webhook":
		return d.webhook(ctx, rule, body)
	case "telegram":
		token, chatID, err := ParseTelegramTarget(rule.Target)
		if err != nil {
			return err
		}
		return SendTelegram(ctx, token, chatID,
			fmt.Sprintf("sentry-lite: %s\n%s\n%s", ev.Title, ev.Summary, link))
	default:
		return fmt.Errorf("unknown channel %s", rule.Channel)
	}
}

// ParseTelegramTarget expects "botToken|chatId".
func ParseTelegramTarget(target string) (token, chatID string, err error) {
	parts := strings.SplitN(target, "|", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("telegram target must be botToken|chatId")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// SendTelegram posts a message via Bot API.
func SendTelegram(ctx context.Context, token, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload, _ := json.Marshal(map[string]any{
		"chat_id": chatID,
		"text":    text,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("telegram status %d", res.StatusCode)
	}
	return nil
}

// SendTelegramConnectTest sends the first-connect sample message.
func SendTelegramConnectTest(ctx context.Context, target string) error {
	token, chatID, err := ParseTelegramTarget(target)
	if err != nil {
		return err
	}
	return SendTelegram(ctx, token, chatID, "sentry-lite connected — Telegram alerts are ready.")
}

func (d *Dispatcher) slack(ctx context.Context, url, text string) error {
	payload, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("slack status %d", res.StatusCode)
	}
	return nil
}

func (d *Dispatcher) email(to, subject, body string) error {
	if d.SMTP == "" {
		return fmt.Errorf("SMTP not configured (set ALERT_SMTP)")
	}
	from := d.From
	if from == "" {
		from = "sentry-lite@localhost"
	}
	msg := []byte("To: " + to + "\r\nSubject: " + subject + "\r\n\r\n" + body)
	return smtp.SendMail(d.SMTP, nil, from, []string{to}, msg)
}

func (d *Dispatcher) webhook(ctx context.Context, rule store.AlertRule, body map[string]any) error {
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rule.Target, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if rule.Secret != "" {
		mac := hmac.New(sha256.New, []byte(rule.Secret))
		mac.Write(raw)
		req.Header.Set("X-Sentry-Lite-Signature", hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", res.StatusCode)
	}
	return nil
}
