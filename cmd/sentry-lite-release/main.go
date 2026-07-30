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
	flag.Parse()
	if *version == "" {
		fmt.Fprintln(os.Stderr, "usage: sentry-lite-release -version=1.2.3 [-project=1] [-ref=abc] [-url=...]")
		os.Exit(2)
	}
	body, _ := json.Marshal(map[string]any{
		"project_id": *projectID,
		"version":    *version,
		"ref":        *ref,
		"url":        *url,
	})
	res, err := http.Post(*apiURL+"/api/internal/releases", "application/json", bytes.NewReader(body))
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
