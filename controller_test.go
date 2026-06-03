package config

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestController_LoadGetSetSave(t *testing.T) {
	td := t.TempDir()
	path := filepath.Join(td, "config.yaml")
	writeFile(t, path, "name: fromfile\ncount: 2\n")

	controller := NewController[testCfg2](WithPath[testCfg2](path))
	cfg := testCfg2{}
	if err := controller.Load(nil, &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	name, err := controller.Get("name")
	if err != nil {
		t.Fatalf("Get(name) error: %v", err)
	}
	if got := name.(string); got != "fromfile" {
		t.Fatalf("unexpected name: %v", name)
	}

	if err := controller.Set("count", "9"); err != nil {
		t.Fatalf("Set(count) error: %v", err)
	}
	if cfg.Count != 9 {
		t.Fatalf("cfg not updated: %+v", cfg)
	}

	if err := controller.Save(nil); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	reloaded := testCfg2{}
	if err := Load(nil, &reloaded, WithPath[testCfg2](path)); err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if reloaded.Count != 9 {
		t.Fatalf("expected persisted count=9, got %+v", reloaded)
	}
}

func TestController_GetSet_NestedOption(t *testing.T) {
	type nestedCfg struct {
		DB *struct {
			Host string `yaml:"host"`
		} `yaml:"db"`
	}

	controller := NewController[nestedCfg]()
	cfg := nestedCfg{}
	if err := controller.Load(nil, &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := controller.Set("db.host", "localhost"); err != nil {
		t.Fatalf("Set(db.host) error: %v", err)
	}
	if cfg.DB == nil || cfg.DB.Host != "localhost" {
		t.Fatalf("nested field not set: %+v", cfg)
	}
	value, err := controller.Get("db.host")
	if err != nil {
		t.Fatalf("Get(db.host) error: %v", err)
	}
	if value != "localhost" {
		t.Fatalf("unexpected nested value: %v", value)
	}
}

func TestController_SaveWithoutPath(t *testing.T) {
	controller := NewController[testCfg2]()
	cfg := testCfg2{}
	if err := controller.Load(nil, &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := controller.Save(nil); !errors.Is(err, configerrors.ErrPathNotConfigured) {
		t.Fatalf("expected ErrPathNotConfigured, got %v", err)
	}
}

func TestController_OptionNotFound(t *testing.T) {
	controller := NewController[testCfg2]()
	cfg := testCfg2{}
	if err := controller.Load(nil, &cfg); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if _, err := controller.Get("missing"); !errors.Is(err, configerrors.ErrOptionNotFound) {
		t.Fatalf("expected ErrOptionNotFound, got %v", err)
	}
}

func TestController_Concurrent_LoadGetSet(t *testing.T) {
	controller := NewController[testCfg2]()
	cfg := testCfg2{Name: "default"}

	const workers = 16
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := controller.Load(nil, &cfg); err != nil {
				t.Errorf("Load() error: %v", err)
				return
			}
			if _, err := controller.Get("name"); err != nil {
				t.Errorf("Get() error: %v", err)
			}
			if err := controller.Set("count", i); err != nil {
				t.Errorf("Set() error: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

func TestController_SameTarget_DifferentSettingsIsolated(t *testing.T) {
	td := t.TempDir()
	pathA := filepath.Join(td, "a.yaml")
	pathB := filepath.Join(td, "b.yaml")
	writeFile(t, pathA, "name: first\n")
	writeFile(t, pathB, "name: second\n")

	target := testCfg2{}
	controllerA := NewController[testCfg2](WithPath[testCfg2](pathA))
	controllerB := NewController[testCfg2](WithPath[testCfg2](pathB))

	if err := controllerA.Load(nil, &target); err != nil {
		t.Fatalf("controllerA.Load() error: %v", err)
	}
	if target.Name != "first" {
		t.Fatalf("unexpected target after controllerA load: %+v", target)
	}

	other := testCfg2{}
	if err := controllerB.Load(nil, &other); err != nil {
		t.Fatalf("controllerB.Load() error: %v", err)
	}
	if other.Name != "second" {
		t.Fatalf("unexpected target after controllerB load: %+v", other)
	}
}

func TestController_InvalidOptionValues(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ctrl *Controller[testCfg2]
	}{
		{name: "empty path", err: configerrors.ErrEmptyPath, ctrl: NewController[testCfg2](WithPath[testCfg2](""))},
		{name: "empty app name", err: configerrors.ErrEmptyAppName, ctrl: NewController[testCfg2](WithAppName[testCfg2](""))},
		{name: "empty env prefix", err: configerrors.ErrEmptyEnvPrefix, ctrl: NewController[testCfg2](WithEnvPrefix[testCfg2](""))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testCfg2{}
			if err := tt.ctrl.Load(nil, &cfg); !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

