package config

import (
	pip "github.com/ygrebnov/config/pipeline"
)

// DefaultPipeline constructs the default stage-based pipeline:
//  0. Factory (no-op placeholder; Provider constructs cfg before pipeline)
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
		if st := m.stageFactory(); st != nil {
			stages = append(stages, st)
		}
		if st := m.stageModelDefaults(); st != nil {
			stages = append(stages, st)
		}
		if st := m.stageFileOps(); st != nil {
			stages = append(stages, st)
		}
		if st := m.stageEnv(); st != nil {
			stages = append(stages, st)
		}
		if st := m.stageModelValidation(); st != nil {
			stages = append(stages, st)
		}
		m.defaultStages = stages
	})
	return pip.New[T]().AddStages(m.defaultStages...)
}
