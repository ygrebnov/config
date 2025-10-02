package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
)

// modelDefaultsSource applies model-based defaults if a model initializer was provided.
// It constructs the model (if not already), calls SetDefaults, and records provider.model.
// It is idempotent under the Provider's sync.Once guarded initialization.
type modelDefaultsSource[T any] struct{ provider *Provider[T] }

func (s *modelDefaultsSource[T]) Name() string { return "model-defaults" }

func (s *modelDefaultsSource[T]) Load(ctx context.Context, target *T) (bool, error) { // nolint: revive
	_ = ctx
	if s.provider.modelInit == nil {
		return false, nil
	}
	if s.provider.model != nil { // already initialized (defensive)
		return false, nil
	}
	mdl, err := s.provider.modelInit(target)
	if err != nil {
		return false, err
	}
	if mdl == nil { // Treat nil silently (mirrors existing tests that return (nil,nil)).
		return false, nil
	}
	s.provider.model = mdl
	if err := s.provider.model.SetDefaults(); err != nil {
		return false, err
	}
	return true, nil
}

// fileSource loads config from file and creates it if missing in persistent mode.
type fileSource[T any] struct{ provider *Provider[T] }

func (s *fileSource[T]) Name() string { return "file" }

func (s *fileSource[T]) Load(ctx context.Context, target *T) (bool, error) { // nolint: revive
	_ = ctx
	p := s.provider
	if p.configPath == "" {
		return false, nil // non-persistent and no override env var set
	}
	err := loadFromFile(p.configPath, target)
	switch {
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return false, err
	case err != nil && errors.Is(err, os.ErrNotExist) && p.persist:
		if pe := EnsurePath(p.configPath); pe != nil {
			return false, errors.Join(ErrEnsureConfigDir, pe)
		}
		if we := writeToFile(p.configPath, target); we != nil {
			return false, errors.Join(ErrWrite, we)
		}
		p.fileCreated = true
		if p.streams != nil && p.streams.Out() != nil {
			fmt.Fprintf(p.streams.Out(), "config: created new config at %s\n", p.configPath)
		}
		return true, nil
	case err == nil:
		if p.persist && p.streams != nil && p.streams.Out() != nil {
			fmt.Fprintf(p.streams.Out(), "config: loaded from %s\n", p.configPath)
		}
		return true, nil
	}
	return false, nil
}

// envSource applies environment overrides using struct tags or field names.
type envSource[T any] struct{ provider *Provider[T] }

func (s *envSource[T]) Name() string { return "env" }

func (s *envSource[T]) Load(ctx context.Context, target *T) (bool, error) { // nolint: revive
	_ = ctx
	if target == nil {
		return false, nil
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return false, nil
	}
	applyEnv(rv.Elem(), s.provider.envPrefix, nil)
	return true, nil // we cannot cheaply detect if any field changed without diffing; return true to indicate executed.
}

// modelValidationFinalizer performs model validation after all sources.
type modelValidationFinalizer[T any] struct{ provider *Provider[T] }

func (f *modelValidationFinalizer[T]) Name() string { return "model-validation" }

func (f *modelValidationFinalizer[T]) Run(ctx context.Context, target *T) error { // nolint: revive
	_ = ctx
	if f.provider.model == nil {
		return nil
	}
	return f.provider.model.Validate()
}
