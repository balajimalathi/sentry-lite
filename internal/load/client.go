package load

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	http    *http.Client
	cfg     Config
	storeURL string
}

type SendResult struct {
	StatusCode int
	Latency    time.Duration
	Err        error
}

func NewClient(cfg Config) *Client {
	transport := &http.Transport{
		MaxIdleConns:        cfg.Workers * 2,
		MaxIdleConnsPerHost: cfg.Workers * 2,
		MaxConnsPerHost:     cfg.Workers * 2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		storeURL: fmt.Sprintf("%s/api/%d/store/", cfg.BaseURL, cfg.ProjectID),
	}
}

func (c *Client) authHeader() string {
	return fmt.Sprintf("Sentry sentry_key=%s", c.cfg.PublicKey)
}

func (c *Client) SendStore(ctx context.Context, body []byte) SendResult {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.storeURL, bytes.NewReader(body))
	if err != nil {
		return SendResult{Err: err, Latency: time.Since(start)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sentry-Auth", c.authHeader())

	res, err := c.http.Do(req)
	lat := time.Since(start)
	if err != nil {
		return SendResult{Err: err, Latency: lat}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return SendResult{StatusCode: res.StatusCode, Latency: lat}
}

func (c *Client) Health(ctx context.Context) (ok bool, latency time.Duration, err error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/healthz", nil)
	if err != nil {
		return false, 0, err
	}
	res, err := c.http.Do(req)
	lat := time.Since(start)
	if err != nil {
		return false, lat, err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode == http.StatusOK, lat, nil
}

type CronMonitor struct {
	ID    int64  `json:"id"`
	Token string `json:"token"`
	Slug  string `json:"slug"`
}

func (c *Client) EnsureCron(ctx context.Context) (string, error) {
	listURL := fmt.Sprintf("%s/api/internal/crons?project_id=%d", c.cfg.BaseURL, c.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return "", err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("list crons: %d %s", res.StatusCode, b)
	}
	var monitors []CronMonitor
	if err := json.NewDecoder(res.Body).Decode(&monitors); err != nil {
		return "", err
	}
	for _, m := range monitors {
		if m.Slug == "sample-heartbeat" && m.Token != "" {
			return m.Token, nil
		}
	}
	body, _ := json.Marshal(map[string]any{
		"project_id":    c.cfg.ProjectID,
		"name":          "Sample Heartbeat",
		"slug":          "sample-heartbeat",
		"schedule_sec":  60,
		"grace_sec":     30,
		"environment":   "development",
	})
	createURL := c.cfg.BaseURL + "/api/internal/crons"
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err = c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		return "", fmt.Errorf("create cron: %d %s", res.StatusCode, b)
	}
	var created CronMonitor
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", err
	}
	return created.Token, nil
}

func (c *Client) CronCheckIn(ctx context.Context, token string, status string) SendResult {
	start := time.Now()
	body, _ := json.Marshal(map[string]any{
		"status":      status,
		"duration_ms": 12 + time.Now().UnixNano()%40,
	})
	url := fmt.Sprintf("%s/api/cron/check-in/%s", c.cfg.BaseURL, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return SendResult{Err: err, Latency: time.Since(start)}
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	lat := time.Since(start)
	if err != nil {
		return SendResult{Err: err, Latency: lat}
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return SendResult{StatusCode: res.StatusCode, Latency: lat}
}
