package config

import (
	pip "github.com/ygrebnov/config/pipeline"
)

// DefaultPipeline constructs the default stage-based pipeline:
//  1. Model.SetDefaults (if model is configured)
//  2. File ops: load; if persistent and missing, create; if non-persistent and missing, ignore
//  3. Environment variable overrides
//  4. Model.Validate (if model is configured, honoring the Provider's validation strategy)
func (m *Provider[T]) DefaultPipeline() *pip.Pipeline[T] {
	pl := pip.New[T]()
	// 1) Model defaults (optional)
	if m.modelInit != nil {
		pl = pl.AddStages(pip.StageModelDefaults[T](func(c *T) (pip.Model, error) {
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
	// 2) File ops via FileSource (honors persistence); skip when no path is resolved.
	if m.configPath != "" {
		pl = pl.AddStages(pip.StageFileOps[T](
			func() string { return m.configPath },
			func() bool { return m.persist },
			EnsurePath,
			loadFromFileT[T],
			writeToFileT[T],
			m.streams,
			func(p string) { m.fileCreated = true },
			func(string) {},
		))
	}
	// 3) Env overrides
	pl = pl.AddStages(pip.StageEnv[T](
		func() string { return m.envPrefix },
		func(t *T, _ string, strat pip.SetStrategy) {
			m.loadFromEnv(t, SetStrategy(strat))
		},
		pip.SetStrategy(m.envSetStrategy),
	))
	// 4) Model validation (optional)
	if m.modelInit != nil {
		pl = pl.AddStages(pip.StageModelValidation[T](
			func() pip.Model { return m.model },
			pip.ValidationStrategy(m.validationStrategy),
			func(err error, _ pip.ValidationStrategy) error { return m.applyValidationStrategy(err) },
		))
	}
	return pl
}
