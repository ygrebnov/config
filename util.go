package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrInaccessiblePath        = errors.New("inaccessible path")
	ErrCannotCreateDirectories = errors.New("cannot create directories")
)

// EnsurePath ensures the directories for a file path exist and the path
// does not already exist as a directory.
func EnsurePath(p string) error {
	info, err := os.Stat(p)
	switch {
	case err == nil:
		if info.IsDir() {
			return ErrInaccessiblePath
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return ErrInaccessiblePath
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ErrCannotCreateDirectories
	}
	return nil
}

func loadFromFile(path string, cfg interface{}) error {
	if path == "" {
		return nil
	}
	ext := filepath.Ext(path)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return fmt.Errorf("%w: %s", ErrUnsupportedConfigFileType, ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)
	default:
		err = yaml.Unmarshal(data, cfg)
	}
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrParse, path, err)
	}
	return nil
}

func applyEnv(v reflect.Value, prefix string, segments []string) {
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
		if sf.PkgPath != "" {
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
			applyEnv(field, prefix, append(segments, seg))
		case reflect.String:
			if s, ok := getString(envName); ok && field.CanSet() {
				field.SetString(s)
			}
		case reflect.Bool:
			if b, ok := getBool(envName); ok && field.CanSet() {
				field.SetBool(b)
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Type() == reflect.TypeOf(time.Duration(0)) {
				if d, ok := getDuration(envName); ok && field.CanSet() {
					field.SetInt(int64(d))
				}
			} else if n, ok := getInt(envName); ok && field.CanSet() {
				field.SetInt(n)
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if n, ok := getInt(envName); ok && field.CanSet() && n >= 0 {
				field.SetUint(uint64(n))
			}
		case reflect.Pointer:
			elem := field.Type().Elem()
			switch elem.Kind() {
			case reflect.Struct:
				// Allocate *struct only if there is at least one nested env var present
				// for this segment (e.g., APP_PINNER_*). This avoids allocating when no
				// relevant env vars are set.
				base := buildEnvName(prefix, append(segments, seg)) + "_"
				if hasAnyEnvWithPrefix(base) {
					if field.IsNil() && field.CanSet() {
						field.Set(reflect.New(elem))
					}
					applyEnv(field, prefix, append(segments, seg))
				}
			case reflect.String:
				if s, ok := getString(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
					}
					field.Elem().SetString(s)
				}
			case reflect.Bool:
				if b, ok := getBool(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
					}
					field.Elem().SetBool(b)
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if elem == reflect.TypeOf(time.Duration(0)) {
					if d, ok := getDuration(envName); ok && field.CanSet() {
						if field.IsNil() {
							field.Set(reflect.New(elem))
						}
						field.Elem().SetInt(int64(d))
					}
				} else if n, ok := getInt(envName); ok && field.CanSet() {
					if field.IsNil() {
						field.Set(reflect.New(elem))
					}
					field.Elem().SetInt(n)
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if n, ok := getInt(envName); ok && field.CanSet() && n >= 0 {
					if field.IsNil() {
						field.Set(reflect.New(elem))
					}
					field.Elem().SetUint(uint64(n))
				}
			}
		}
	}
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

func hasAnyEnvWithPrefix(prefix string) bool {
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

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
	// Split words only on lower→upper case transitions (e.g., ApiKey → API_KEY).
	// Do NOT split between letters and digits so that ApiKey2FA → API_KEY2FA.
	return (prev >= 'a' && prev <= 'z') && (curr >= 'A' && curr <= 'Z')
}

func toUpper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - 'a' + 'A'
	}
	return r
}

func writeToFile(path string, cfg interface{}) (retErr error) {
	// Guard against panics from encoders (e.g., yaml on unsupported kinds like func).
	defer func() {
		if r := recover(); r != nil {
			// Use current extension for context in the error message.
			ext := filepath.Ext(path)
			retErr = fmt.Errorf("%w as %s: %v", ErrFormat, ext, r)
		}
	}()

	ext := filepath.Ext(path)
	if ext != "" && ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return fmt.Errorf("%w: %s", ErrUnsupportedConfigFileType, ext)
	}
	var data []byte
	var err error
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(cfg, "", "  ")
	default:
		data, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return fmt.Errorf("%w as %s: %w", ErrFormat, ext, err)
	}
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "temp-config-*"+ext)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("%w %s: %w", ErrWrite, path, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}
	return
}
