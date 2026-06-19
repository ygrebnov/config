// Package config: environment variable reflection & application helpers.
package config

/*
import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func applyEnvToTarget(target any, prefix string, strategy SetStrategy) {
	if target == nil {
		return
	}
	rv := reflect.ValueOf(target)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	applyEnv(rv, prefix, strategy)
}

// applyEnv delegates to the precomputed metadata fast-path walker to reduce reflection churn.
func applyEnv(v reflect.Value, prefix string, strategy SetStrategy) {
	applyEnvWithMeta(v, prefix, strategy)
}

func buildEnvName(prefix string, segments []string) string {
	switch {
	case prefix == "" && len(segments) == 0:
		return ""
	case prefix == "":
		return strings.Join(segments, "_")
	case len(segments) == 0:
		return prefix
	default:
		return prefix + "_" + strings.Join(segments, "_")
	}
}

func getString(name string) (string, bool) {
	v, ok := os.LookupEnv(name)
	return v, ok
}

func getInt(name string) (int64, bool) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func getBool(name string) (bool, bool) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return false, false
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, false
	}
	return b, true
}

func getDuration(name string) (time.Duration, bool) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return 0, false
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, false
	}
	return d, true
}

// toScreamingSnake converts ASCII CamelCase / PascalCase to SCREAMING_SNAKE preserving digit groups.
func toScreamingSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && isBoundary(s, i) {
			b.WriteByte('_')
		}
		b.WriteRune(toUpper(r))
	}
	return b.String()
}

func isBoundary(s string, i int) bool {
	prev := rune(s[i-1])
	curr := rune(s[i])
	next := rune(0)
	if i+1 < len(s) {
		next = rune(s[i+1])
	}

	if isLowerASCII(prev) && isUpperASCII(curr) {
		return true
	}

	if (isUpperASCII(prev) || isDigitASCII(prev)) && isUpperASCII(curr) && isLowerASCII(next) {
		return true
	}

	return false
}

func isLowerASCII(r rune) bool { return r >= 'a' && r <= 'z' }

func isUpperASCII(r rune) bool { return r >= 'A' && r <= 'Z' }

func isDigitASCII(r rune) bool { return r >= '0' && r <= '9' }

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}
*/
