package config

import (
	"context"
	"errors"
	"fmt"
	"os"

	pip "github.com/ygrebnov/config/pipeline"
)

// DefaultPipeline constructs the default stage-based pipeline:
//  1. Model.SetDefaults (if model is configured)
//  2. File read (non-persistent semantics: read if exists; ignore missing; never create)
//  3. Environment variable overrides
//  4. Model.Validate (if model is configured, honoring the Provider's validation strategy)
func (m *Provider[T]) DefaultPipeline() *pip.Pipeline[T] {
	pl := pip.New[T]()
	// 1) Model defaults
	if m.modelInit != nil {
		pl = pl.AddStages(pip.StageFromSource(&pip.ModelDefaultsSource[T]{
			Init: func(c *T) (pip.Model, error) {
				mdl, err := m.modelInit(c)
				if err != nil {
					return nil, err
				}
				if mdl != nil {
					m.model = mdl
				}
				return mdl, nil
			},
		}))
	}
	// 2) Non-persistent file read
	fileRead := pip.NewStage[T]("file-read", func(ctx context.Context, t *T) (bool, error) {
		_ = ctx
		if m.configPath == "" {
			return false, nil
		}
		if err := loadFromFile(m.configPath, t); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return false, nil
			}
			return false, err
		}
		if m.persist && m.streams != nil && m.streams.Out() != nil {
			fmt.Fprintf(m.streams.Out(), "config: loaded from %s\n", m.configPath)
		}
		return true, nil
	})
	pl = pl.AddStages(fileRead)
	// 3) Env overrides
	env := pip.NewStage[T]("env", func(ctx context.Context, t *T) (bool, error) {
		_ = ctx
		m.loadFromEnv(t, m.envSetStrategy)
		return true, nil
	})
	pl = pl.AddStages(env)
	// 4) Model validation
	if m.modelInit != nil {
		pl = pl.AddStages(pip.StageFromFinalizer(&pip.ModelValidationFinalizer[T]{
			Model:     func() pip.Model { return m.model },
			Strategy:  pip.ValidationStrategy(m.validationStrategy),
			ReduceErr: func(err error, _ pip.ValidationStrategy) error { return m.applyValidationStrategy(err) },
		}))
	}
	return pl
}
