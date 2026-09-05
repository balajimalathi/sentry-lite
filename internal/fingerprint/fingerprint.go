package fingerprint

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

const (
	ConfigV1         = "sentry-lite:v1"
	Config20260901   = "sentry-lite:2026-09-01"
	DefaultNewConfig = Config20260901
)

const (
	VariantV1        = "v1"
	VariantApp       = "app"
	VariantSystem    = "system"
	VariantException = "exception"
	VariantMessage   = "message"
	VariantCustom    = "custom"
	VariantRule      = "rule"
)

var hexID = regexp.MustCompile(`[0-9a-fA-F]{8,}`)
var digits = regexp.MustCompile(`\d+`)
var varRe = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)

type Frame struct {
	Filename string
	Function string
	AbsPath  string
	Module   string
	InApp    bool
}

type Variant struct {
	Hash string
	Kind string
}

type Result struct {
	Primary  string
	Variant  string
	Variants []Variant
	Title    string
}

type Input struct {
	Config         string
	Rules          []Rule
	SDKFingerprint []string
	ExceptionType  string
	Message        string
	Logger         string
	Level          string
	Transaction    string
	Tags           map[string]string
	Frames         []Frame
}

func ValidConfig(id string) bool {
	id = strings.TrimSpace(id)
	return id == ConfigV1 || id == Config20260901
}

func NormalizeConfig(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ConfigV1
	}
	return id
}

// Compute is the legacy v1 hash (explicit fingerprint or exception+top-frame+message).
func Compute(explicit []string, exceptionType, message string, frames []Frame) string {
	return Group(Input{
		Config:         ConfigV1,
		SDKFingerprint: explicit,
		ExceptionType:  exceptionType,
		Message:        message,
		Frames:         frames,
	}).Primary
}

func Group(in Input) Result {
	if in.Tags == nil {
		in.Tags = map[string]string{}
	}
	defaults := defaultVariants(in)

	parts := in.SDKFingerprint
	kind := VariantCustom
	title := ""
	if rule := Match(in.Rules, in); rule != nil {
		parts = rule.Fingerprint
		kind = VariantRule
		if rule.Title != "" {
			title = expandVars(rule.Title, in, defaults)
		}
	}

	if !hasCustomParts(parts) {
		if title != "" {
			defaults.Title = title
		}
		return defaults
	}
	return expandCustom(parts, defaults, kind, title, in)
}

func defaultVariants(in Input) Result {
	switch NormalizeConfig(in.Config) {
	case Config20260901:
		return defaultNew(in)
	default:
		return defaultV1(in)
	}
}

func defaultV1(in Input) Result {
	frameKey := ""
	if f := topFrame(in.Frames); f != nil {
		frameKey = frameFile(*f) + ":" + f.Function
	}
	h := hash(fmt.Sprintf("%s|%s|%s", in.ExceptionType, frameKey, normalizeMessage(in.Message)))
	return Result{
		Primary:  h,
		Variant:  VariantV1,
		Variants: []Variant{{Hash: h, Kind: VariantV1}},
	}
}

func defaultNew(in Input) Result {
	if len(in.Frames) > 0 {
		appFrames := inAppFrames(in.Frames)
		if len(appFrames) == 0 {
			appFrames = in.Frames
		}
		appH := hash(fmt.Sprintf("%s|%s", in.ExceptionType, stackKey(appFrames)))
		sysH := hash(fmt.Sprintf("%s|%s", in.ExceptionType, stackKey(in.Frames)))
		variants := []Variant{{Hash: appH, Kind: VariantApp}}
		if sysH != appH {
			variants = append(variants, Variant{Hash: sysH, Kind: VariantSystem})
		}
		return Result{Primary: appH, Variant: VariantApp, Variants: variants}
	}
	if in.ExceptionType != "" && strings.TrimSpace(in.Message) != "" {
		h := hash(fmt.Sprintf("%s|%s", in.ExceptionType, normalizeMessage(in.Message)))
		return Result{
			Primary:  h,
			Variant:  VariantException,
			Variants: []Variant{{Hash: h, Kind: VariantException}},
		}
	}
	h := hash(fmt.Sprintf("%s|%s", in.Logger, normalizeMessage(in.Message)))
	return Result{
		Primary:  h,
		Variant:  VariantMessage,
		Variants: []Variant{{Hash: h, Kind: VariantMessage}},
	}
}

func hasCustomParts(parts []string) bool {
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || isDefaultVar(p) {
			continue
		}
		return true
	}
	return false
}

func expandCustom(parts []string, defaults Result, kind, title string, in Input) Result {
	expanded := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if isDefaultVar(p) {
			expanded = append(expanded, defaults.Primary)
			continue
		}
		expanded = append(expanded, expandVars(p, in, defaults))
	}
	if len(expanded) == 0 {
		if title != "" {
			defaults.Title = title
		}
		return defaults
	}
	h := hash(strings.Join(expanded, "|"))
	return Result{
		Primary:  h,
		Variant:  kind,
		Title:    title,
		Variants: []Variant{{Hash: h, Kind: kind}},
	}
}

func isDefaultVar(p string) bool {
	p = strings.TrimSpace(p)
	return p == "{{ default }}" || p == "{{default}}"
}

func expandVars(s string, in Input, defaults Result) string {
	return varRe.ReplaceAllStringFunc(s, func(m string) string {
		name := strings.TrimSpace(varRe.FindStringSubmatch(m)[1])
		return varValue(name, in, defaults)
	})
}

func varValue(name string, in Input, defaults Result) string {
	switch name {
	case "default":
		return defaults.Primary
	case "error.type", "type":
		return in.ExceptionType
	case "error.value", "value":
		return in.Message
	case "message":
		return in.Message
	case "logger":
		return in.Logger
	case "level":
		return in.Level
	case "transaction":
		return in.Transaction
	case "stack.abs_path", "path":
		if f := topFrame(in.Frames); f != nil {
			if f.AbsPath != "" {
				return f.AbsPath
			}
			return f.Filename
		}
		return ""
	case "stack.filename":
		if f := topFrame(in.Frames); f != nil {
			return f.Filename
		}
		return ""
	case "stack.function", "function":
		if f := topFrame(in.Frames); f != nil {
			return f.Function
		}
		return ""
	case "stack.module", "module":
		if f := topFrame(in.Frames); f != nil {
			return f.Module
		}
		return ""
	default:
		if strings.HasPrefix(name, "tags.") {
			return in.Tags[strings.TrimPrefix(name, "tags.")]
		}
		return ""
	}
}

func inAppFrames(frames []Frame) []Frame {
	out := make([]Frame, 0, len(frames))
	for _, f := range frames {
		if f.InApp {
			out = append(out, f)
		}
	}
	return out
}

func stackKey(frames []Frame) string {
	parts := make([]string, 0, len(frames))
	for _, f := range frames {
		parts = append(parts, frameFile(f)+":"+f.Function)
	}
	return strings.Join(parts, "|")
}

func frameFile(f Frame) string {
	file := f.Filename
	if file == "" {
		file = f.AbsPath
	}
	if file == "" {
		file = f.Module
	}
	return normalizePath(file)
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
