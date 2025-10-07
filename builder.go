package config

import (
	"context"
	"sync"

	modellib "github.com/ygrebnov/model"

	pip "github.com/ygrebnov/config/pipeline"
	"github.com/ygrebnov/config/streams"
)

// Builder is a minimal, pipeline-centric alternative to Provider.
// It holds only execution controls and the list of stages to run.
type Builder[T any] struct {
	mu       sync.RWMutex
	initOnce sync.Once
	initErr  error
	stages   []pip.Stage[T]
}

// BuilderOption configures a Builder by appending stages.
type BuilderOption[T any] func(*Builder[T])

// NewBuilder constructs a Builder and applies all given options (which append stages).
func NewBuilder[T any](opts ...BuilderOption[T]) *Builder[T] {
	b := &Builder[T]{}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// Build executes the configured stages on the provided target. If no stages were
// configured, Build is a no-op.
func (b *Builder[T]) Build(ctx context.Context, target *T) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(b.stages) == 0 {
		// Minimal by default: do nothing when no stages are provided.
		return nil
	}
	pl := pip.New[T]().AddStages(b.stages...)
	_, err := pl.Execute(ctx, target)
	return err
}

// ---- Builder Options backed by pre-defined stages ----

// WithBuilderFactory appends a stage that overwrites the target with the value produced by fn.
func WithBuilderFactory[T any](fn func() *T) BuilderOption[T] {
	return func(b *Builder[T]) { b.stages = append(b.stages, pip.StageFactory(fn)) }
}

// WithBuilderModelDefaults appends a stage that applies defaults using the provided model init.
// The modellib.Model[T] from github.com/ygrebnov/model satisfies pipeline.Model.
func WithBuilderModelDefaults[T any](init func(*T) (*modellib.Model[T], error)) BuilderOption[T] {
	return func(b *Builder[T]) {
		b.stages = append(b.stages, pip.StageModelDefaults(func(c *T) (pip.Model, error) {
			return init(c)
		}))
	}
}

// WithBuilderFileOps appends a file operations stage using config helpers for IO.
// path resolves the file path; persist controls create-on-missing; onCreated/onLoaded
// may be nil; streams route user-facing messages.
func WithBuilderFileOps[T any](
	path func() string,
	persist func() bool,
	streams streams.IOStreams,
	onCreated func(string),
	onLoaded func(string),
) BuilderOption[T] {
	return func(b *Builder[T]) {
		b.stages = append(b.stages, pip.StageFileOps[T](
			path,
			persist,
			EnsurePath,
			loadFromFileT[T],
			writeToFileT[T],
			streams,
			onCreated,
			onLoaded,
		))
	}
}

// WithBuilderEnv appends an environment overrides stage using the config package's logic.
func WithBuilderEnv[T any](prefix string, strategy SetStrategy) BuilderOption[T] {
	return func(b *Builder[T]) {
		b.stages = append(b.stages, pip.StageEnv[T](
			func() string { return prefix },
			pip.SetStrategy(strategy),
		))
	}
}

// WithBuilderModelValidation appends a model validation stage.
func WithBuilderModelValidation[T any](
	model func() pip.Model,
	strategy ValidationStrategy,
	reduce func(error, ValidationStrategy) error,
) BuilderOption[T] {
	return func(b *Builder[T]) {
		b.stages = append(b.stages, pip.StageModelValidation[T](
			model,
			pip.ValidationStrategy(strategy),
			func(err error, _ pip.ValidationStrategy) error { return reduce(err, strategy) },
		))
	}
}

// WithBuilderModelValidateInit appends a validation stage that constructs a model using init
// at execution time and validates the current target.
func WithBuilderModelValidateInit[T any](init func(*T) (*modellib.Model[T], error), reduce func(error, ValidationStrategy) error, strategy ValidationStrategy) BuilderOption[T] {
	return func(b *Builder[T]) {
		b.stages = append(b.stages, pip.NewStage[T]("model-validate", func(ctx context.Context, t *T) (bool, error) {
			_ = ctx
			if init == nil || t == nil {
				return false, nil
			}
			mdl, err := init(t)
			if err != nil || mdl == nil {
				return false, err
			}
			if err := mdl.Validate(); err != nil {
				if reduce != nil {
					return false, reduce(err, strategy)
				}
				return false, err
			}
			return false, nil
		}))
	}
}
