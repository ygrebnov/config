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
// applyEnv walks a struct value and applies environment variable overrides
// based on the `env` tag or SCREAMING_SNAKE_CASE field names.
func applyEnv(v reflect.Value, prefix string, segments []string, strategy SetStrategy) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		tag := sf.Tag.Get(envVarTagName)
		if tag == "-" {
			continue
		}
		seg := tag
		if seg == "" {
			seg = toScreamingSnake(sf.Name)
		}
		field := v.Field(i)
		envName := buildEnvName(prefix, append(segments, seg))
		switch field.Kind() {
		case reflect.Struct:
			applyEnv(field, prefix, append(segments, seg), strategy)
		case reflect.String:
			if s, ok := getString(envName); ok && field.CanSet() {
				if strategy == SetOverride || isZero(field) {
					field.SetString(s)
				}
			}
		case reflect.Bool:
			if b, ok := getBool(envName); ok && field.CanSet() {
				if strategy == SetOverride || isZero(field) {
					field.SetBool(b)
				}
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if d, ok := getDuration(envName); ok && field.CanSet() {
					if strategy == SetOverride || isZero(field) {
						field.SetInt(int64(d))
					}
				}
			} else if n, ok := getInt(envName); ok && field.CanSet() {
				if strategy == SetOverride || isZero(field) {
					field.SetInt(n)
				}
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if n, ok := getInt(envName); ok && field.CanSet() && n >= 0 {
				if strategy == SetOverride || isZero(field) {
					field.SetUint(uint64(n))
				}
			}
		case reflect.Pointer:
			elem := field.Type().Elem()
			switch elem.Kind() {
			case reflect.Struct:
				base := buildEnvName(prefix, append(segments, seg)) + "_"
				if hasAnyEnvWithPrefix(base) {
					if field.IsNil() && field.CanSet() {
						field.Set(reflect.New(elem))
					}
					applyEnv(field, prefix, append(segments, seg), strategy)
				}
			case reflect.String:
				if s, ok := getString(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
						field.Elem().SetString(s)
					} else if strategy == SetOverride {
						field.Elem().SetString(s)
					}
				}
			case reflect.Bool:
				if b, ok := getBool(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
						field.Elem().SetBool(b)
					} else if strategy == SetOverride {
						field.Elem().SetBool(b)
					}
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if elem == reflect.TypeOf(time.Duration(0)) {
					if d, ok := getDuration(envName); ok && field.CanSet() {
						if field.IsNil() {
							field.Set(reflect.New(elem))
							field.Elem().SetInt(int64(d))
						} else if strategy == SetOverride {
							field.Elem().SetInt(int64(d))
						}
					}
				} else if n, ok := getInt(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
						field.Elem().SetInt(n)
					} else if strategy == SetOverride {
						field.Elem().SetInt(n)
					}
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if n, ok := getInt(envName); ok && field.CanSet() && n >= 0 {
					if field.IsNil() {
						field.Set(reflect.New(elem))
						field.Elem().SetUint(uint64(n))
					} else if strategy == SetOverride {
						field.Elem().SetUint(uint64(n))
					}
				}
			}
		}
	}
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
