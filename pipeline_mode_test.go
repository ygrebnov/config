package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modellib "github.com/ygrebnov/model"
)

// TestPipelineMode_BehaviorParity covers representative scenarios from the legacy
// Get() path to ensure pipeline mode preserves semantics (ordering, file create/load,
// env overrides, model defaults & validation, error surfaces).
func TestPipelineMode_BehaviorParity(t *testing.T) {
	td := t.TempDir()
	// Normalize user config dir resolution path
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	var outBuf, errBuf bytes.Buffer
	streams := fakeStreams{out: &outBuf, errOut: &errBuf}

	// local helpers
	write := func(rel string, data string) string {
		p := filepath.Join(td, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		return p
	}

	t.Run("create missing env override path (persistent)", func(t *testing.T) {
		outBuf.Reset()
		errBuf.Reset()
		p := write("envdir/.keep", "") // ensure parent; file itself will differ
		_ = p
		cfgPath := filepath.Join(td, "env-override", "config.yaml")
		t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
		prov := New[testCfg2](
			WithPipelineMode[testCfg2](),
			WithEnvPrefix[testCfg2]("MYAPP"),
			WithPersistence[testCfg2]("ignored"),
			WithDefaultFn[testCfg2](defFn),
			WithStreams[testCfg2](streams),
		)
		cfg, path, created, err := prov.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatalf("expected fileCreated=true")
		}
		if !strings.HasSuffix(path, filepath.Join("env-override", "config.yaml")) {
			t.Fatalf("unexpected path: %s", path)
		}
		if cfg.Name != "default" || cfg.Count != 1 {
			t.Fatalf("cfg mismatch: %+v", cfg)
		}
		if !strings.Contains(outBuf.String(), "created new config") {
			t.Fatalf("expected created message, got %q", outBuf.String())
		}
	})

	t.Run("load existing env override path", func(t *testing.T) {
		outBuf.Reset()
		errBuf.Reset()
		cfgPath := write("present/config.yaml", "name: fromfile\ncount: 7\n")
		t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
		prov := New[testCfg2](
			WithPipelineMode[testCfg2](),
			WithEnvPrefix[testCfg2]("MYAPP"),
			WithPersistence[testCfg2]("anything"),
			WithDefaultFn[testCfg2](defFn),
			WithStreams[testCfg2](streams),
		)
		cfg, path, created, err := prov.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created {
			t.Fatalf("expected fileCreated=false")
		}
		if !strings.HasSuffix(path, filepath.Join("present", "config.yaml")) {
			t.Fatalf("unexpected path: %s", path)
		}
		if cfg.Name != "fromfile" || cfg.Count != 7 {
			t.Fatalf("cfg mismatch: %+v", cfg)
		}
		if !strings.Contains(outBuf.String(), "loaded from") {
			t.Fatalf("expected loaded message, got %q", outBuf.String())
		}
	})

	t.Run("parse error surfaces", func(t *testing.T) {
		outBuf.Reset()
		errBuf.Reset()
		cfgPath := write("bad/config.yaml", "name: [unclosed\n")
		t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
		prov := New[testCfg2](
			WithPipelineMode[testCfg2](),
			WithEnvPrefix[testCfg2]("MYAPP"),
			WithPersistence[testCfg2]("irrelevant"),
			WithDefaultFn[testCfg2](defFn),
			WithStreams[testCfg2](streams),
		)
		_, _, _, err := prov.Get()
		if err == nil || !strings.Contains(err.Error(), "parse config file") {
			t.Fatalf("expected parse error, got %v", err)
		}
	})

	t.Run("persistent create via UserConfigDir", func(t *testing.T) {
		outBuf.Reset()
		errBuf.Reset()
		t.Setenv("MYAPP_CONFIG_PATH", "")
		prov := New[testCfg2](
			WithPipelineMode[testCfg2](),
			WithPersistence[testCfg2]("newapp2"),
			WithEnvPrefix[testCfg2]("MYAPP"),
			WithDefaultFn[testCfg2](defFn),
			WithStreams[testCfg2](streams),
		)
		cfg, path, created, err := prov.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatalf("expected fileCreated=true")
		}
		if !strings.HasSuffix(path, filepath.Join("xdg", "newapp2", "config.yml")) {
			t.Fatalf("unexpected path: %s", path)
		}
		if cfg.Name != "default" || cfg.Count != 1 {
			t.Fatalf("cfg mismatch: %+v", cfg)
		}
		if !strings.Contains(outBuf.String(), "created new config") {
			t.Fatalf("expected created message")
		}
	})

	t.Run("env overrides after file load", func(t *testing.T) {
		outBuf.Reset()
		errBuf.Reset()
		cfgPath := write("over/config.yaml", "name: fromfile\ncount: 2\ndur: 1s\n")
		t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
		t.Setenv("MYAPP_NAME", "fromenv")
		t.Setenv("MYAPP_COUNT", "9")
		t.Setenv("MYAPP_DUR", "3s")
		prov := New[testCfg2](
			WithPipelineMode[testCfg2](),
			WithPersistence[testCfg2]("overapp"),
			WithEnvPrefix[testCfg2]("MYAPP"),
			WithDefaultFn[testCfg2](defFn),
			WithStreams[testCfg2](streams),
		)
		cfg, _, created, err := prov.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created {
			t.Fatalf("expected fileCreated=false (existing env path)")
		}
		if cfg.Name != "fromenv" || cfg.Count != 9 || cfg.Dur != 3*time.Second {
			t.Fatalf("env overrides not applied: %+v", cfg)
		}
	})

	t.Run("model defaults + validation ok", func(t *testing.T) {
		// file with only name; port default then env override
		cfgPath := write("model_ok/config.yaml", "name: fromfile\n")
		t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
		t.Setenv("MYAPP_PORT", "9090")
		prov := New[mCfg](
			WithPipelineMode[mCfg](),
			WithEnvPrefix[mCfg]("MYAPP"),
			WithDefaultFn[mCfg](func() *mCfg { return &mCfg{} }),
			WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }),
		)
		cfg, path, created, err := prov.Get()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created {
			t.Fatalf("expected fileCreated=false existing env path")
		}
		if !strings.HasSuffix(path, filepath.Join("model_ok", "config.yaml")) {
			t.Fatalf("unexpected path: %s", path)
		}
		if cfg.Name != "fromfile" || cfg.Port != 9090 {
			t.Fatalf("unexpected cfg: %+v", cfg)
		}
	})

	t.Run("model validation error surfaces", func(t *testing.T) {
		t.Setenv("MYAPP_NAME", "")
		t.Setenv("MYAPP_PORT", "0")
		prov := New[mCfg](
			WithPipelineMode[mCfg](),
			WithEnvPrefix[mCfg]("MYAPP"),
			WithDefaultFn[mCfg](func() *mCfg { return &mCfg{} }),
			WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }),
		)
		_, _, _, err := prov.Get()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		var ve *modellib.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("expected validation error, got %T: %v", err, err)
		}
		msg := err.Error()
		if !strings.Contains(msg, "nonempty") || !strings.Contains(msg, "nonzero") {
			t.Fatalf("missing expected rule substrings: %q", msg)
		}
	})
}
