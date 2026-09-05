package fingerprint

import (
	"fmt"
	"strings"
	"unicode"
)

type Matcher struct {
	Key      string
	Pattern  string
	Negate   bool
	PathMode bool
	CI       bool
	Frame    bool
}

type Rule struct {
	Matchers    []Matcher
	Fingerprint []string
	Title       string
}

func ParseRules(src string) ([]Rule, error) {
	var rules []Rule
	for i, line := range strings.Split(src, "\n") {
		line = stripComment(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rule, err := parseRule(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func Match(rules []Rule, in Input) *Rule {
	for i := range rules {
		if rules[i].match(in) {
			return &rules[i]
		}
	}
	return nil
}

func (r Rule) match(in Input) bool {
	var frameMs []Matcher
	for _, m := range r.Matchers {
		if m.Frame {
			frameMs = append(frameMs, m)
			continue
		}
		if !m.matchEvent(in) {
			return false
		}
	}
	if len(frameMs) == 0 {
		return true
	}
	if len(in.Frames) == 0 {
		return false
	}
	for _, f := range in.Frames {
		ok := true
		for _, m := range frameMs {
			if !m.matchFrame(f) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func (m Matcher) matchEvent(in Input) bool {
	hit := false
	switch m.Key {
	case "error.type", "type":
		hit = matchGlob(m.Pattern, in.ExceptionType, false, m.CI)
	case "error.value", "value":
		hit = matchGlob(m.Pattern, in.Message, false, m.CI)
	case "message":
		hit = matchGlob(m.Pattern, in.Message, false, m.CI)
	case "logger":
		hit = matchGlob(m.Pattern, in.Logger, false, m.CI)
	case "level":
		hit = matchGlob(m.Pattern, in.Level, false, m.CI)
	default:
		if strings.HasPrefix(m.Key, "tags.") {
			hit = matchGlob(m.Pattern, in.Tags[strings.TrimPrefix(m.Key, "tags.")], false, m.CI)
		}
	}
	if m.Negate {
		return !hit
	}
	return hit
}

func (m Matcher) matchFrame(f Frame) bool {
	hit := false
	switch m.Key {
	case "stack.abs_path", "path":
		hit = matchGlob(m.Pattern, f.AbsPath, true, m.CI) || matchGlob(m.Pattern, f.Filename, true, m.CI)
	case "stack.function", "function":
		hit = matchGlob(m.Pattern, f.Function, false, m.CI)
	case "stack.module", "module":
		hit = matchGlob(m.Pattern, f.Module, false, m.CI)
	case "app":
		want := strings.EqualFold(m.Pattern, "yes") || strings.EqualFold(m.Pattern, "true") || m.Pattern == "1"
		hit = f.InApp == want
	}
	if m.Negate {
		return !hit
	}
	return hit
}

func parseRule(line string) (Rule, error) {
	left, right, ok := splitArrow(line)
	if !ok {
		return Rule{}, fmt.Errorf("missing ->")
	}
	matchers, err := parseMatchers(strings.TrimSpace(left))
	if err != nil {
		return Rule{}, err
	}
	if len(matchers) == 0 {
		return Rule{}, fmt.Errorf("no matchers")
	}
	fp, title, err := parseFingerprintSide(strings.TrimSpace(right))
	if err != nil {
		return Rule{}, err
	}
	if len(fp) == 0 {
		return Rule{}, fmt.Errorf("empty fingerprint")
	}
	return Rule{Matchers: matchers, Fingerprint: fp, Title: title}, nil
}

func parseMatchers(s string) ([]Matcher, error) {
	toks, err := tokenizeMatchers(s)
	if err != nil {
		return nil, err
	}
	out := make([]Matcher, 0, len(toks))
	for _, tok := range toks {
		negate := false
		if strings.HasPrefix(tok, "!") {
			negate = true
			tok = tok[1:]
		}
		key, pat, ok := strings.Cut(tok, ":")
		if !ok || key == "" || pat == "" {
			return nil, fmt.Errorf("invalid matcher %q", tok)
		}
		pat = unquote(pat)
		m, err := newMatcher(key, pat, negate)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func newMatcher(key, pattern string, negate bool) (Matcher, error) {
	m := Matcher{Key: key, Pattern: pattern, Negate: negate}
	switch key {
	case "error.type", "type", "logger":
		m.CI = false
	case "error.value", "value", "message", "level":
		m.CI = true
	case "stack.abs_path", "path":
		m.CI = true
		m.PathMode = true
		m.Frame = true
	case "stack.function", "function":
		m.Frame = true
	case "stack.module", "module":
		m.Frame = true
	case "app":
		m.Frame = true
		m.CI = true
	default:
		if strings.HasPrefix(key, "tags.") && len(key) > 5 {
			m.CI = true
			return m, nil
		}
		return Matcher{}, fmt.Errorf("unknown matcher %q", key)
	}
	return m, nil
}

func parseFingerprintSide(s string) (parts []string, title string, err error) {
	title, rest := extractTitle(s)
	fields, err := splitComma(rest)
	if err != nil {
		return nil, "", err
	}
	for _, f := range fields {
		f = strings.TrimSpace(unquote(f))
		if f != "" {
			parts = append(parts, f)
		}
	}
	return parts, title, nil
}

func extractTitle(s string) (title, rest string) {
	rest = strings.TrimSpace(s)
	lower := strings.ToLower(rest)
	idx := strings.LastIndex(lower, "title=")
	if idx < 0 {
		return "", rest
	}
	after := strings.TrimSpace(rest[idx+len("title="):])
	if after == "" {
		return "", strings.TrimSpace(rest[:idx])
	}
	if after[0] == '"' {
		end := 1
		for end < len(after) {
			if after[end] == '\\' && end+1 < len(after) {
				end += 2
				continue
			}
			if after[end] == '"' {
				title = unquote(after[:end+1])
				return title, strings.TrimSpace(rest[:idx])
			}
			end++
		}
		return "", rest
	}
	return "", rest
}

func splitArrow(s string) (left, right string, ok bool) {
	inQ := false
	esc := false
	for i := 0; i < len(s)-1; i++ {
		c := s[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			inQ = !inQ
			continue
		}
		if !inQ && c == '-' && s[i+1] == '>' {
			return s[:i], s[i+2:], true
		}
	}
	return "", "", false
}

func tokenizeMatchers(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	inQ := false
	esc := false
	flush := func() {
		tok := strings.TrimSpace(b.String())
		if tok != "" {
			out = append(out, tok)
		}
		b.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			b.WriteByte(c)
			esc = false
			continue
		}
		if c == '\\' {
			b.WriteByte(c)
			esc = true
			continue
		}
		if c == '"' {
			inQ = !inQ
			b.WriteByte(c)
			continue
		}
		if !inQ && unicode.IsSpace(rune(c)) {
			flush()
			continue
		}
		b.WriteByte(c)
	}
	if inQ {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return out, nil
}

func splitComma(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	inQ := false
	brace := 0
	esc := false
	flush := func() {
		tok := strings.TrimSpace(b.String())
		if tok != "" {
			out = append(out, tok)
		}
		b.Reset()
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if esc {
			b.WriteByte(c)
			esc = false
			continue
		}
		if c == '\\' {
			b.WriteByte(c)
			esc = true
			continue
		}
		if c == '"' {
			inQ = !inQ
			b.WriteByte(c)
			continue
		}
		if !inQ && c == '{' {
			brace++
			b.WriteByte(c)
			continue
		}
		if !inQ && c == '}' && brace > 0 {
			brace--
			b.WriteByte(c)
			continue
		}
		if !inQ && brace == 0 && c == ',' {
			flush()
			continue
		}
		b.WriteByte(c)
	}
	if inQ {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	return out, nil
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `\\`, `\`)
		return inner
	}
	return s
}

func stripComment(line string) string {
	inQ := false
	esc := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if esc {
			esc = false
			continue
		}
		if c == '\\' {
			esc = true
			continue
		}
		if c == '"' {
			inQ = !inQ
			continue
		}
		if !inQ && c == '#' {
			return line[:i]
		}
	}
	return line
}
