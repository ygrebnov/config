package config

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	modelvalidation "github.com/ygrebnov/model/validation"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestLoad_ExplicitPath_CurrentStateFileEnvValidation(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: fromfile\ncount: 3\ndur: 1s\n")

	t.Setenv("APP_NAME", "fromenv")
	t.Setenv("APP_COUNT", "9")
	t.Setenv("APP_DUR", "3s")

	var outBuf bytes.Buffer
	cfg := testCfg2{Name: "default", Count: 1}
	err := Load(nil, &cfg,
		WithPath[testCfg2](path),
		WithEnvPrefix[testCfg2]("APP"),
		WithStreams[testCfg2](fakeStreams{out: &outBuf}),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "fromenv" || cfg.Count != 9 || cfg.Dur != 3*time.Second {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if !strings.Contains(outBuf.String(), "loaded from") {
		t.Fatalf("expected load message, got %q", outBuf.String())
	}
}

func TestLoad_WellKnownPath(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", td)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	path := filepath.Join(td, "myapp", "config.yml")
	writeFile(t, path, "name: usercfg\ncount: 5\n")

	cfg := testCfg2{}
	err := Load(nil, &cfg, WithAppName[testCfg2]("myapp"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "usercfg" || cfg.Count != 5 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_MissingFile_NoError(t *testing.T) {
	td := t.TempDir()
	cfg := testCfg2{Name: "default", Count: 1}
	err := Load(nil, &cfg,
		WithPath[testCfg2](filepath.Join(td, "missing.yaml")),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "default" || cfg.Count != 1 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_ModelDefaultsAndValidation(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "model.yaml")
	writeFile(t, path, "name: fromfile\n")
	t.Setenv("APP_PORT", "9090")

	cfg := mCfg{}
	err := Load(nil, &cfg,
		WithPath[mCfg](path),
		WithEnvPrefix[mCfg]("APP"),
		WithModel[mCfg](),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "fromfile" || cfg.Port != 9090 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_ValidateFirstError(t *testing.T) {
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_PORT", "0")

	cfg := mCfg{}
	err := Load(nil, &cfg,
		WithEnvPrefix[mCfg]("APP"),
		WithModel[mCfg](),
		WithValidationStrategy[mCfg](ValidateFirstError),
	)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var ve *modelvalidation.Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected validation.Error, got %T", err)
	}
	if ve.Len() != 1 {
		t.Fatalf("expected one issue, got %d", ve.Len())
	}
}

func TestLoad_InvalidTarget(t *testing.T) {
	var x int
	err := Load(nil, &x)
	if !errors.Is(err, configerrors.ErrNotStructPointer) {
		t.Fatalf("expected ErrNotStructPointer, got %v", err)
	}
}

func TestLoad_Concurrent_Once(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: fromfile\ncount: 1\n")

	cfg := testCfg2{}

	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			err := Load(nil, &cfg, WithPath[testCfg2](path))
			if err != nil {
				t.Errorf("Load() error: %v", err)
			}
		}()
	}
	wg.Wait()

	if cfg.Name != "fromfile" || cfg.Count != 1 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_SameTarget_ReusesInitializedState(t *testing.T) {
	td := t.TempDir()
	pathA := filepath.Join(td, "a.yaml")
	pathB := filepath.Join(td, "b.yaml")
	writeFile(t, pathA, "name: one\n")
	writeFile(t, pathB, "name: two\n")

	cfg := testCfg2{}
	if err := Load(nil, &cfg, WithPath[testCfg2](pathA)); err != nil {
		t.Fatalf("first Load() error: %v", err)
	}

	if err := Load(nil, &cfg, WithPath[testCfg2](pathB)); err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	if cfg.Name != "one" {
		t.Fatalf("expected first initialized value to be preserved, got %+v", cfg)
	}
}

func TestLoad_Model(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "model-tags.yaml")
	writeFile(t, path, "name: fromfile\n")
	t.Setenv("APP_PORT", "9090")

	cfg := mCfg{}
	err := Load(nil, &cfg,
		WithPath[mCfg](path),
		WithEnvPrefix[mCfg]("APP"),
		WithModel[mCfg](),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "fromfile" || cfg.Port != 9090 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_InvalidOptionValues(t *testing.T) {
	if err := Load(nil, &testCfg2{}, WithPath[testCfg2]("")); !errors.Is(err, configerrors.ErrEmptyPath) {
		t.Fatalf("expected ErrEmptyPath, got %v", err)
	}
	if err := Load(nil, &testCfg2{}, WithAppName[testCfg2]("")); !errors.Is(err, configerrors.ErrEmptyAppName) {
		t.Fatalf("expected ErrEmptyAppName, got %v", err)
	}
	if err := Load(nil, &testCfg2{}, WithEnvPrefix[testCfg2]("")); !errors.Is(err, configerrors.ErrEmptyEnvPrefix) {
		t.Fatalf("expected ErrEmptyEnvPrefix, got %v", err)
	}
}
