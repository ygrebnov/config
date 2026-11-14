package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ygrebnov/config"
	"github.com/ygrebnov/config/pkg/errors"
)

func TestController_SaveWithoutPath(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("Controller constructor error: %v", err)
	}
	cfg := smallCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := controller.Save(context.Background()); !errors.Is(err, errors.ErrPathNotConfigured) {
		t.Fatalf("expected ErrPathNotConfigured, got %v", err)
	}
}

func TestController_SaveWithCustomPath(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	controller.Set("Name", "saved")
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := controller.Save(
		context.Background(),
		config.WithSavePath(path),
	); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	if saved := readFile(t, path); !strings.Contains(saved, "name: saved\n") {
		t.Fatalf("custom-path configuration = %q", saved)
	}
}

func TestController_SaveCreatesMissingExplicitFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	controller, err := config.NewController[smallCfg](config.WithPath(path))
	if err != nil {
		t.Fatalf("NewController() missing file error: %v", err)
	}
	controller.Set("Name", "created")
	if err := controller.Save(context.Background()); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if saved := readFile(t, path); !strings.Contains(saved, "name: created\n") {
		t.Fatalf("saved configuration = %q", saved)
	}
}

func TestController_SaveRejectsMissingCustomDirectory(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "missing", "config.yaml")
	err = controller.Save(context.Background(), config.WithSavePath(path))
	if !errors.Is(err, errors.ErrConfigurationDirectoryNotFound) {
		t.Fatalf("Save() error = %v, want ErrConfigurationDirectoryNotFound", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatalf("Save() created custom directory: %v", err)
	}
}

func TestController_SaveRejectsNilContext(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	if err := controller.Save(nil); !errors.Is(err, errors.ErrNilContext) {
		t.Fatalf("Save(nil) error = %v, want ErrNilContext", err)
	}
}

func TestController_SaveRejectsCancelledContext(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := controller.Save(ctx); err != context.Canceled {
		t.Fatalf("Save(cancelled context) error = %v, want %v", err, context.Canceled)
	}
}

func TestController_SaveReturnsConfigurationObjectError(t *testing.T) {
	controller, err := config.NewController[smallCfg]()
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	controller.Set("Name", []string{"invalid"})
	path := filepath.Join(t.TempDir(), "config.json")
	err = controller.Save(context.Background(), config.WithSavePath(path))
	if err == nil {
		t.Fatal("Save() error = nil, want configuration object error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected no config file after configuration object error, got %v", statErr)
	}
}

func TestController_SaveReturnsMarshalError(t *testing.T) {
	type unmarshalableCfg struct {
		Value func()
	}

	controller, err := config.NewController[unmarshalableCfg]()
	if err != nil {
		t.Fatalf("NewController() error: %v", err)
	}

	controller.Set("Value", func() {})
	path := filepath.Join(t.TempDir(), "config.json")
	err = controller.Save(context.Background(), config.WithSavePath(path))
	if err == nil {
		t.Fatal("Save() error = nil, want marshal error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("expected no config file after marshal error, got %v", statErr)
	}
}
