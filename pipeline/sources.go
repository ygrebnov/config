// Package pipeline defines the core configuration pipeline abstractions
// (sources, finalizers, execution engine) and strategy enums used by
// the config Provider.
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ygrebnov/config/streams"
)

// Model is the minimal interface required from a validation/defaults model
// used by the model defaults and validation components.
type Model interface {
	SetDefaults() error
	Validate() error
}

// ModelDefaultsSource constructs a model via Init and applies defaults.
type ModelDefaultsSource[T any] struct {
	Init func(*T) (Model, error)
	// internal cache is left to the caller/provider; this source is stateless.
}

func (s *ModelDefaultsSource[T]) Name() string { return "model-defaults" }

func (s *ModelDefaultsSource[T]) Load(ctx context.Context, target *T) (bool, error) {
	_ = ctx
	if s == nil || s.Init == nil || target == nil {
		return false, nil
	}
	mdl, err := s.Init(target)
	if err != nil || mdl == nil {
		return false, err
	}
	if err := mdl.SetDefaults(); err != nil {
		return false, err
	}
	return true, nil
}

// FileSource loads config from file via injected functions; may create if missing when Persist() is true.
type FileSource[T any] struct {
	Path         func() string
	Persist      func() bool
	EnsurePath   func(string) error
	LoadFromFile func(string, *T) error
	WriteToFile  func(string, *T) error
	Streams      streams.IOStreams
	OnCreated    func(path string)
	OnLoaded     func(path string)
}

func (s *FileSource[T]) Name() string { return "file" }

func (s *FileSource[T]) Load(ctx context.Context, target *T) (bool, error) {
	_ = ctx
	if s == nil || target == nil {
		return false, nil
	}
	path := ""
	if s.Path != nil {
		path = s.Path()
	}
	if path == "" {
		return false, nil
	}
	if err := s.LoadFromFile(path, target); err != nil {
		// Only create on missing file; propagate all other errors (e.g., parse errors)
		if s.Persist != nil && s.Persist() && errors.Is(err, os.ErrNotExist) {
			if s.EnsurePath != nil {
				if e := s.EnsurePath(path); e != nil {
					return false, e
				}
			}
			if s.WriteToFile != nil {
				if e := s.WriteToFile(path, target); e != nil {
					return false, e
				}
			}
			if s.Streams != nil && s.Streams.Out() != nil {
				fmt.Fprintf(s.Streams.Out(), "config: created new config at %s\n", path)
			}
			if s.OnCreated != nil {
				s.OnCreated(path)
			}
			return true, nil
		}
		return false, err
	}
	if s.Streams != nil && s.Streams.Out() != nil {
		fmt.Fprintf(s.Streams.Out(), "config: loaded from %s\n", path)
	}
	if s.OnLoaded != nil {
		s.OnLoaded(path)
	}
	return true, nil
}

// EnvSource applies environment overrides using Apply with the provided prefix and strategy.
type EnvSource[T any] struct {
	Prefix   func() string
	Apply    func(*T, string, SetStrategy)
	Strategy SetStrategy
}

func (s *EnvSource[T]) Name() string { return "env" }

func (s *EnvSource[T]) Load(ctx context.Context, target *T) (bool, error) {
	_ = ctx
	if s == nil || target == nil || s.Apply == nil {
		return false, nil
	}
	prefix := ""
	if s.Prefix != nil {
		prefix = s.Prefix()
	}
	s.Apply(target, prefix, s.Strategy)
	return true, nil
}

// ModelValidationFinalizer validates the target using the provided Model and strategy.
type ModelValidationFinalizer[T any] struct {
	Model     func() Model
	Strategy  ValidationStrategy
	ReduceErr func(err error, strategy ValidationStrategy) error
}

func (f *ModelValidationFinalizer[T]) Name() string { return "model-validation" }

func (f *ModelValidationFinalizer[T]) Run(ctx context.Context, target *T) error {
	_ = ctx
	if f == nil || f.Model == nil {
		return nil
	}
	m := f.Model()
	if m == nil {
		return nil
	}
	if err := m.Validate(); err != nil {
		if f.ReduceErr != nil {
			return f.ReduceErr(err, f.Strategy)
		}
		return err
	}
	return nil
}
