package pipeline

import (
	"context"

	"github.com/ygrebnov/config/streams"
)

// StageFactory creates a stage that overwrites the target with the value produced by fn.
// If fn returns nil, the stage is a no-op.
func StageFactory[T any](fn func() *T) Stage[T] {
	return NewStage[T]("factory", func(ctx context.Context, t *T) (bool, error) {
		_ = ctx
		if t == nil || fn == nil {
			return false, nil
		}
		src := fn()
		if src == nil {
			return false, nil
		}
		// Overwrite target value
		*t = *src
		return true, nil
	})
}

// StageModelDefaults applies defaults using the provided model initializer.
func StageModelDefaults[T any](init func(*T) (Model, error)) Stage[T] {
	return StageFromSource(&ModelDefaultsSource[T]{Init: init})
}

// StageFileOps constructs a file operations stage that loads from path and, when
// persist() is true, creates the file if it is missing. Creation/loaded events are
// surfaced via Streams and callbacks.
func StageFileOps[T any](
	path func() string,
	persist func() bool,
	ensurePath func(string) error,
	loadFromFile func(string, *T) error,
	writeToFile func(string, *T) error,
	streams streams.IOStreams,
	onCreated func(string),
	onLoaded func(string),
) Stage[T] {
	return StageFromSource(&FileSource[T]{
		Path:         path,
		Persist:      persist,
		EnsurePath:   ensurePath,
		LoadFromFile: loadFromFile,
		WriteToFile:  writeToFile,
		Streams:      streams,
		OnCreated:    onCreated,
		OnLoaded:     onLoaded,
	})
}

// StageEnv applies environment overrides using internal reflection logic with the provided prefix and strategy.
func StageEnv[T any](
	prefix func() string,
	strategy SetStrategy,
) Stage[T] {
	return StageFromSource(&EnvSource[T]{
		Prefix:   prefix,
		Strategy: strategy,
	})
}

// StageModelValidation validates the target using the provided Model accessor and strategy.
func StageModelValidation[T any](
	model func() Model,
	strategy ValidationStrategy,
	reduce func(error, ValidationStrategy) error,
) Stage[T] {
	return StageFromFinalizer(&ModelValidationFinalizer[T]{
		Model:     model,
		Strategy:  strategy,
		ReduceErr: reduce,
	})
}
