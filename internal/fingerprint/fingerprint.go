package fingerprint

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

var hexID = regexp.MustCompile(`[0-9a-fA-F]{8,}`)
var digits = regexp.MustCompile(`\d+`)

type Frame struct {
	Filename string
	Function string
	AbsPath  string
	Module   string
	InApp    bool
}

// Compute returns a stable grouping hash.
// Prefer explicit fingerprint parts when provided.
func Compute(explicit []string, exceptionType, message string, frames []Frame) string {
	if len(explicit) > 0 {
		parts := make([]string, 0, len(explicit))
		for _, p := range explicit {
			p = strings.TrimSpace(p)
			if p != "" && p != "{{ default }}" {
				parts = append(parts, p)
			}
		}
		if len(parts) > 0 {
			return hash(strings.Join(parts, "|"))
		}
	}

	frameKey := ""
	if f := topFrame(frames); f != nil {
		file := f.Filename
		if file == "" {
			file = f.AbsPath
		}
		if file == "" {
			file = f.Module
		}
		frameKey = normalizePath(file) + ":" + f.Function
	}

	msg := normalizeMessage(message)
	raw := fmt.Sprintf("%s|%s|%s", exceptionType, frameKey, msg)
	return hash(raw)
}

func topFrame(frames []Frame) *Frame {
	// Prefer the most recent in-app frame (Sentry frames are usually oldest→newest)
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i].InApp {
			return &frames[i]
		}
	}
	if len(frames) > 0 {
		return &frames[len(frames)-1]
	}
	return nil
}

func Culprit(frames []Frame) string {
	f := topFrame(frames)
	if f == nil {
		return ""
	}
	file := f.Filename
	if file == "" {
		file = f.AbsPath
	}
	if f.Function != "" {
		return file + " in " + f.Function
	}
	return file
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	parts := strings.Split(p, "/")
	if len(parts) > 3 {
		parts = parts[len(parts)-3:]
	}
	return strings.Join(parts, "/")
}

func normalizeMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	msg = hexID.ReplaceAllString(msg, "<hex>")
	msg = digits.ReplaceAllString(msg, "<num>")
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

func hash(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
