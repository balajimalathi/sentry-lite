package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	apiURL := flag.String("api", env("SENTRY_LITE_API", "http://localhost:8080"), "API base URL")
	projectID := flag.Int64("project", 1, "project id")
	version := flag.String("version", "", "release version (required)")
	ref := flag.String("ref", "", "git ref")
	url := flag.String("url", "", "release URL")
	token := flag.String("token", env("SENTRY_LITE_TOKEN", ""), "admin token (Authorization: Bearer)")
	flag.Parse()
	if *version == "" {
		fmt.Fprintln(os.Stderr, "usage: sentry-lite-release -version=1.2.3 [-project=1] [-token=...]")
		os.Exit(2)
	}
	body, _ := json.Marshal(map[string]any{
		"project_id": *projectID,
		"version":    *version,
		"ref":        *ref,
		"url":        *url,
	})
	req, err := http.NewRequest(http.MethodPost, *apiURL+"/api/internal/releases", bytes.NewReader(body))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	if *token != "" {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		fmt.Fprintln(os.Stderr, "status", res.Status)
		os.Exit(1)
	}
	fmt.Println("release created/updated:", *version)
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
