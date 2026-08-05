package tests

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	modelerrors "github.com/ygrebnov/model/pkg/errors"

	"github.com/ygrebnov/config"
	configerrors "github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/log"
	pathPkg "github.com/ygrebnov/config/pkg/path"
	"github.com/ygrebnov/config/pkg/types"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name          string
		opts          []config.Option
		before        func(t *testing.T)
		expectedCfg   cfg
		expectedError error
	}{
		{
			name:        "no options, only object itself",
			expectedCfg: cfg{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.before != nil {
				tt.before(t)
			}

			obj := cfg{}
			err := config.Load(context.Background(), &obj, tt.opts...)
			if tt.expectedError != nil {
				if err == nil {
					t.Fatalf("expected Load to return an error, but got none")
				}
				if !errors.Is(err, tt.expectedError) {
					t.Fatalf("expected Load to return error: %v, got %v", tt.expectedError, err)
				}
			} else if err != nil {
				t.Fatalf("expected Load to return no error, but got: %v", err)
			}

			checkCfg(t, obj, tt.expectedCfg)
		})
	}
}

type testLogger struct {
	out string
}

func (l *testLogger) Log(_ log.Level, message string, fields ...log.Field) {
	l.out += message + ","
	for _, f := range fields {
		l.out += fmt.Sprintf(" %s=%v", f.Key, f.Value)
	}
}

func TestLoad_ExplicitPath_CurrentStateFileEnvValidation(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: fromfile\ncount: 3\ndur: 1s\n")

	t.Setenv("APP_NAME", "fromenv")
	t.Setenv("APP_COUNT", "9")
	t.Setenv("APP_DUR", "3s")

	cfg := smallCfg{Name: "default", Count: 1}
	logger := &testLogger{}
	err := config.Load(context.Background(), &cfg,
		config.WithPath(path),
		config.WithEnvPrefix("APP"),
		config.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	expected := smallCfg{Name: "fromenv", Count: 9, Dur: types.Duration(3 * time.Second)}
	if cfg.Name != expected.Name {
		t.Errorf("unexpected cfg.Name value, got: %v want: %v", cfg.Name, expected.Name)
	}
	if cfg.Count != expected.Count {
		t.Errorf("unexpected cfg.Count value, got: %v want: %v", cfg.Count, expected.Count)
	}
	if cfg.Dur != expected.Dur {
		t.Errorf("unexpected cfg.Dur value, got: %v want: %v", cfg.Dur, expected.Dur)
	}

	expectedOut := "loaded configuration, path=" + path
	if !strings.Contains(logger.out, expectedOut) {
		t.Fatalf("unexpected out message, got %q, want: %s", logger.out, expectedOut)
	}
}

func TestLoad_WellKnownPath(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", td)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	path := filepath.Join(td, "myapp", pathPkg.DefaultConfigFilename)
	writeFile(t, path, "name: usercfg\ncount: 5\n")

	cfg := smallCfg{}
	err := config.Load(context.Background(), &cfg, config.WithAppName("myapp"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "usercfg" || cfg.Count != 5 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	td := t.TempDir()

	cfg := smallCfg{Name: "default", Count: 1}

	err := config.Load(context.Background(), &cfg, config.WithPath(filepath.Join(td, "missing.yaml")))
	if err == nil {
		t.Fatalf("expected Load() to return an error, but got none")
	}

	if !errors.Is(err, configerrors.ErrConfigurationFileNotFound) {
		t.Fatalf("expected error to be ErrConfigurationFileNotFound, got %v", err)
	}

	expectedName := "default"
	if cfg.Name != expectedName {
		t.Fatalf("expected name to be: %s, got %s", expectedName, cfg.Name)
	}

	expectedCount := 1
	if cfg.Count != expectedCount {
		t.Fatalf("expected count to be: %d, got %d", expectedCount, cfg.Count)
	}
}

func TestLoad_ModelDefaultsAndValidation(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "model.yaml")
	writeFile(t, path, "name: fromfile\n")
	t.Setenv("APP_PORT", "9090")

	cfg := tinyCfg{}
	err := config.Load(context.Background(), &cfg,
		config.WithPath(path),
		config.WithEnvPrefix("APP"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Name != "fromfile" || cfg.Port != 9090 {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoad_ValidationError(t *testing.T) {
	t.Setenv("APP_NAME", "")
	t.Setenv("APP_PORT", "0")

	cfg := tinyCfg{}
	err := config.Load(context.Background(), &cfg, config.WithEnvPrefix("APP"))
	if err == nil {
		t.Fatalf("expected configuration loading error")
	}
	if !errors.Is(err, configerrors.ErrCannotInitializeConfigurationObject) {
		t.Fatalf("expected ErrCannotLoadConfigurationIntoProvidedObject, got %v", err)
	}

	expectedErrorMsg := `cannot initialize configuration object, cause: - Field "Name": rule "min": rule constraint violated (length=1)
- Field "Port": rule "min": rule constraint violated (value=1)
- Field "Port": rule "nonzero": rule constraint violated, rule.name: nonzero`
	if err.Error() != expectedErrorMsg {
		t.Fatalf("expected error message %q, got %q", expectedErrorMsg, err.Error())
	}
}

func TestLoad_InvalidTarget(t *testing.T) {
	var x int
	err := config.Load(context.Background(), &x)
	if !errors.Is(err, modelerrors.ErrTypeParamNotStruct) {
		t.Fatalf("expected ErrTypeParamNotStruct, got %v", err)
	}
}

func TestLoad_SameTarget_ValuesAreOverwritten(t *testing.T) {
	td := t.TempDir()
	pathA := filepath.Join(td, "a.yaml")
	pathB := filepath.Join(td, "b.yaml")
	writeFile(t, pathA, "name: one\n")
	writeFile(t, pathB, "name: two\n")

	cfg := smallCfg{}
	if err := config.Load(context.Background(), &cfg, config.WithPath(pathA)); err != nil {
		t.Fatalf("first Load() error: %v", err)
	}

	if err := config.Load(context.Background(), &cfg, config.WithPath(pathB)); err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	expectedName := "two"
	if cfg.Name != expectedName {
		t.Fatalf("expected name to be: %s, got %s", expectedName, cfg.Name)
	}
}

func TestLoad_Model(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "model-tags.yaml")
	writeFile(t, path, "name: fromfile\n")
	t.Setenv("APP_PORT", "9090")

	cfg := tinyCfg{}
	err := config.Load(context.Background(), &cfg,
		config.WithPath(path),
		config.WithEnvPrefix("APP"),
	)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	expectedName := "fromfile"
	expectedPort := 9090
	if cfg.Name != expectedName {
		t.Fatalf("expected name to be: %s, got %s", expectedName, cfg.Name)
	}
	if cfg.Port != expectedPort {
		t.Fatalf("expected port to be: %d, got %d", expectedPort, cfg.Port)
	}
}
