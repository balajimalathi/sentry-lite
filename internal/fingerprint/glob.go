package fingerprint

import (
	"regexp"
	"strings"
	"sync"
)

var globCache sync.Map // string → *regexp.Regexp

func globKey(pattern string, pathMode bool) string {
	if pathMode {
		return "p:" + pattern
	}
	return "g:" + pattern
}

func compileGlob(pattern string, pathMode bool) *regexp.Regexp {
	key := globKey(pattern, pathMode)
	if v, ok := globCache.Load(key); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(globToRegexp(pattern, pathMode))
	globCache.Store(key, re)
	return re
}

func matchGlob(pattern, value string, pathMode, caseInsensitive bool) bool {
	if caseInsensitive {
		pattern = strings.ToLower(pattern)
		value = strings.ToLower(value)
	}
	return compileGlob(pattern, pathMode).MatchString(value)
}

func globToRegexp(pattern string, pathMode bool) string {
	var b strings.Builder
	b.WriteString("^")
	i := 0
	for i < len(pattern) {
		if pattern[i] == '*' {
			if pathMode && i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					b.WriteString("/?")
					i++
				}
				continue
			}
			if pathMode {
				b.WriteString("[^/]*")
			} else {
				b.WriteString(".*")
			}
			i++
			continue
		}
		if pattern[i] == '?' {
			if pathMode {
				b.WriteString("[^/]")
			} else {
				b.WriteString(".")
			}
			i++
			continue
		}
		if pattern[i] == '\\' && i+1 < len(pattern) {
			b.WriteString(regexp.QuoteMeta(string(pattern[i+1])))
			i += 2
			continue
		}
		b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		i++
	}
	b.WriteString("$")
	return b.String()
}
