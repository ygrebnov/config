package config

import (
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
	m.defaultStagesOnce.Do(func() {
		var stages []pip.Stage[T]
		// 1) Model defaults (optional)
		if m.modelInit != nil {
			stages = append(stages, pip.StageModelDefaults[T](func(c *T) (pip.Model, error) {
				mdl, err := m.modelInit(c)
				if err != nil {
					return nil, err
				}
				if mdl != nil {
					m.model = mdl
				}
				return mdl, nil
			}))
		}
		// 2) File ops via FileSource (honors persistence)
		stages = append(stages, pip.StageFileOps[T](
			func() string { return m.configPath },
			func() bool { return m.persist },
			func(p string) error { return EnsurePath(p) },
			func(p string, t *T) error { return loadFromFile(p, t) },
			func(p string, t *T) error { return writeToFile(p, t) },
			m.streams,
			func(p string) {
				m.fileCreated = true
			},
			func(p string) {
				// No-op here; streams handled inside file source
			},
		))
		// 3) Env overrides
		stages = append(stages, pip.StageEnv[T](
			func() string { return m.envPrefix },
			func(t *T, _ string, strat pip.SetStrategy) {
				m.loadFromEnv(t, SetStrategy(strat))
			},
			pip.SetStrategy(m.envSetStrategy),
		))
		// 4) Model validation (optional)
		if m.modelInit != nil {
			stages = append(stages, pip.StageModelValidation[T](
				func() pip.Model { return m.model },
				pip.ValidationStrategy(m.validationStrategy),
				func(err error, _ pip.ValidationStrategy) error { return m.applyValidationStrategy(err) },
			))
		}
		m.defaultStages = stages
	})
	return pip.New[T]().AddStages(m.defaultStages...)
}
