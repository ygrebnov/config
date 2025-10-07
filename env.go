// Package config: environment variable reflection & application helpers.
//
// This file provides applyEnv and related utilities that walk a user config
// struct and override fields from environment variables derived from struct
// tags (`env:""`) or SCREAMING_SNAKE_CASE field names.
package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Environment variable application logic split from util.go for clarity.
// applyEnv now delegates to a precomputed metadata fast-path walker to reduce
// reflection churn and string concatenations.
func applyEnv(v reflect.Value, prefix string, _ []string, strategy SetStrategy) {
	applyEnvWithMeta(v, prefix, strategy)
}

func isZero(v reflect.Value) bool { return v.IsZero() }

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

func hasAnyEnvWithPrefix(prefix string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// toScreamingSnake converts CamelCase / PascalCase to SCREAMING_SNAKE preserving digit groups.
func toScreamingSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && isBoundary(rune(s[i-1]), r) {
			b.WriteByte('_')
		}
		b.WriteRune(toUpper(r))
	}
	return b.String()
}

func isBoundary(prev, curr rune) bool {
	return (prev >= 'a' && prev <= 'z') && (curr >= 'A' && curr <= 'Z')
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}
