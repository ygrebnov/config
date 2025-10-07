package config

import (
	"context"

	pip "github.com/ygrebnov/config/pipeline"
)

// DefaultPipeline constructs the default stage-based pipeline:
//  1. Model.SetDefaults (if model is configured)
//  2. File ops: load; if persistent and missing, create; if non-persistent and missing, ignore
//  3. Environment variable overrides
//  4. Model.Validate (if model is configured, honoring the Provider's validation strategy)
//
// Stages are prebuilt once per Provider instance and then reused to avoid
// per-initialization allocations.
func (m *Provider[T]) DefaultPipeline() *pip.Pipeline[T] {
	// Lazily build and cache stages
	m.defaultStagesOnce.Do(func() {
		var stages []pip.Stage[T]
		// 1) Model defaults (optional)
		if m.modelInit != nil {
			stages = append(stages, pip.StageFromSource(&pip.ModelDefaultsSource[T]{
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
		// 2) File ops via FileSource (honors persistence)
		stages = append(stages, pip.StageFromSource(&pip.FileSource[T]{
			Path:         func() string { return m.configPath },
			Persist:      func() bool { return m.persist },
			EnsurePath:   func(p string) error { return EnsurePath(p) },
			LoadFromFile: func(p string, t *T) error { return loadFromFile(p, t) },
			WriteToFile:  func(p string, t *T) error { return writeToFile(p, t) },
			Streams:      m.streams,
			OnCreated: func(p string) {
				m.fileCreated = true
				if m.streams != nil && m.streams.Out() != nil {
					_, _ = m.streams.Out().Write([]byte("config: created new config at " + p + "\n"))
				}
			},
			OnLoaded: func(p string) {
				if m.persist && m.streams != nil && m.streams.Out() != nil {
					_, _ = m.streams.Out().Write([]byte("config: loaded from " + p + "\n"))
				}
			},
		}))
		// 3) Env overrides
		stages = append(stages, pip.NewStage[T]("env", func(ctx context.Context, t *T) (bool, error) {
			_ = ctx
			m.loadFromEnv(t, m.envSetStrategy)
			return true, nil
		}))
		// 4) Model validation (optional)
		if m.modelInit != nil {
			stages = append(stages, pip.StageFromFinalizer(&pip.ModelValidationFinalizer[T]{
				Model:     func() pip.Model { return m.model },
				Strategy:  pip.ValidationStrategy(m.validationStrategy),
				ReduceErr: func(err error, _ pip.ValidationStrategy) error { return m.applyValidationStrategy(err) },
			}))
		}
		m.defaultStages = stages
	})
	// Compose a new pipeline instance with cached stages (pipeline is lightweight).
	return pip.New[T]().AddStages(m.defaultStages...)
}
