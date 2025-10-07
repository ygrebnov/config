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

	"github.com/ygrebnov/config/streams"
)

func TestBuilder_BehaviorParity_Basic(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

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

	var outBuf, errBuf bytes.Buffer
	io := streams.Writers(&outBuf, &errBuf)

	type checkFn func(t *testing.T)

	tests := []struct {
		name  string
		setup func(t *testing.T) (builder *Builder[testCfg2], target *testCfg2, expectPath string)
		check checkFn
		error string
	}{
		{
			name: "create missing env override path (persistent)",
			setup: func(t *testing.T) (*Builder[testCfg2], *testCfg2, string) {
				outBuf.Reset()
				errBuf.Reset()
				cfgPath := filepath.Join(td, "env-override", "config.yaml")
				t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
				created := false
				b := NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](
						func() string { return os.Getenv("MYAPP_CONFIG_PATH") },
						func() bool { return true },
						io,
						func(string) { created = true },
						func(string) {},
					),
				)
				b = NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](func() string { return os.Getenv("MYAPP_CONFIG_PATH") }, func() bool { return true }, io, func(string) { created = true }, func(string) {}),
					WithBuilderEnv[testCfg2]("MYAPP", SetOverride),
				)
				_ = created
				return b, defFn(), cfgPath
			},
			check: func(t *testing.T) {
				if !strings.Contains(outBuf.String(), "created new config") {
					t.Fatalf("expected created message, got %q", outBuf.String())
				}
			},
		},
		{
			name: "load existing env override path",
			setup: func(t *testing.T) (*Builder[testCfg2], *testCfg2, string) {
				outBuf.Reset()
				errBuf.Reset()
				cfgPath := write("present/config.yaml", "name: fromfile\ncount: 7\n")
				t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
				b := NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](func() string { return os.Getenv("MYAPP_CONFIG_PATH") }, func() bool { return true }, io, nil, nil),
					WithBuilderEnv[testCfg2]("MYAPP", SetOverride),
				)
				return b, defFn(), cfgPath
			},
			check: func(t *testing.T) {
				if !strings.Contains(outBuf.String(), "loaded from") {
					t.Fatalf("expected loaded message, got %q", outBuf.String())
				}
			},
		},
		{
			name: "parse error surfaces",
			setup: func(t *testing.T) (*Builder[testCfg2], *testCfg2, string) {
				outBuf.Reset()
				errBuf.Reset()
				cfgPath := write("bad/config.yaml", "name: [unclosed\n")
				t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
				b := NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](func() string { return os.Getenv("MYAPP_CONFIG_PATH") }, func() bool { return true }, io, nil, nil),
				)
				return b, defFn(), cfgPath
			},
			error: "parse config file",
		},
		{
			name: "persistent create via UserConfigDir",
			setup: func(t *testing.T) (*Builder[testCfg2], *testCfg2, string) {
				outBuf.Reset()
				errBuf.Reset()
				t.Setenv("MYAPP_CONFIG_PATH", "")
				cfgPath := filepath.Join(td, "xdg", "newapp2", "config.yml")
				created := false
				b := NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](
						func() string { return cfgPath },
						func() bool { return true },
						io,
						func(string) { created = true },
						func(string) {},
					),
				)
				_ = created
				return b, defFn(), cfgPath
			},
			check: func(t *testing.T) {
				if !strings.Contains(outBuf.String(), "created new config") {
					t.Fatalf("expected created message")
				}
			},
		},
		{
			name: "env overrides after file load",
			setup: func(t *testing.T) (*Builder[testCfg2], *testCfg2, string) {
				outBuf.Reset()
				errBuf.Reset()
				cfgPath := write("over/config.yaml", "name: fromfile\ncount: 2\ndur: 1s\n")
				t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
				t.Setenv("MYAPP_NAME", "fromenv")
				t.Setenv("MYAPP_COUNT", "9")
				t.Setenv("MYAPP_DUR", "3s")
				b := NewBuilder[testCfg2](
					WithBuilderFileOps[testCfg2](func() string { return os.Getenv("MYAPP_CONFIG_PATH") }, func() bool { return true }, io, nil, nil),
					WithBuilderEnv[testCfg2]("MYAPP", SetOverride),
				)
				return b, defFn(), cfgPath
			},
			check: func(t *testing.T) {},
		},
	}

	for _, tt := range tests {
		// capture
		name := tt.name
		setup := tt.setup
		check := tt.check
		errorSub := tt.error
		t.Run(name, func(t *testing.T) {
			b, target, expectPath := setup(t)
			err := b.Build(nil, target)
			if errorSub != "" {
				if err == nil || !strings.Contains(err.Error(), errorSub) {
					t.Fatalf("expected error containing %q, got %v", errorSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if expectPath != "" {
				if _, statErr := os.Stat(expectPath); statErr != nil {
					t.Fatalf("expected path to exist: %s: %v", expectPath, statErr)
				}
			}
			switch tgt := any(target).(type) {
			case *testCfg2:
				if name == "load existing env override path" {
					if tgt.Name != "fromfile" || tgt.Count != 7 {
						t.Fatalf("cfg mismatch: %+v", tgt)
					}
				} else if name == "env overrides after file load" {
					if tgt.Name != "fromenv" || tgt.Count != 9 || tgt.Dur != 3*time.Second {
						t.Fatalf("env overrides not applied: %+v", tgt)
					}
				} else if name == "create missing env override path (persistent)" {
					if tgt.Name != "default" || tgt.Count != 1 {
						t.Fatalf("cfg mismatch: %+v", tgt)
					}
				}
			}
			if check != nil {
				check(t)
			}
		})
	}
}

func TestBuilder_BehaviorParity_Model(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

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

	// model defaults + validate ok
	cfgPath := write("model_ok/config.yaml", "name: fromfile\n")
	t.Setenv("MYAPP_CONFIG_PATH", cfgPath)
	t.Setenv("MYAPP_PORT", "9090")
	b := NewBuilder[mCfg](
		WithBuilderModelDefaults[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }),
		WithBuilderFileOps[mCfg](func() string { return os.Getenv("MYAPP_CONFIG_PATH") }, func() bool { return true }, streams.Discard(), nil, nil),
		WithBuilderEnv[mCfg]("MYAPP", SetOverride),
		WithBuilderModelValidateInit[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }, func(err error, _ ValidationStrategy) error { return err }, ValidateAllErrors),
	)
	cfg := &mCfg{}
	if err := b.Build(nil, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != "fromfile" || cfg.Port != 9090 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}

	// validation error surfaces
	t.Setenv("MYAPP_NAME", "")
	t.Setenv("MYAPP_PORT", "0")
	b2 := NewBuilder[mCfg](
		WithBuilderModelDefaults[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }),
		WithBuilderEnv[mCfg]("MYAPP", SetOverride),
		WithBuilderModelValidateInit[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) { return modellib.New(c) }, func(err error, _ ValidationStrategy) error { return err }, ValidateAllErrors),
	)
	cfg2 := &mCfg{}
	err := b2.Build(nil, cfg2)
	if err == nil || !strings.Contains(err.Error(), "nonempty") {
		t.Fatalf("expected validation error, got %v", err)
	}
	var ve *modellib.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected validation error type, got %T", err)
	}
}
