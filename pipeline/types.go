// Package pipeline defines the core configuration pipeline abstractions
// (stages engine, sources/finalizers adapters) and strategy enums used by
// the config Provider.
package pipeline

import (
	"context"
	"time"
)

// Stage represents an executable unit in the pipeline engine.
// A stage may mutate the target and returns whether it applied any change.
type Stage[T any] interface {
	Name() string
	Execute(ctx context.Context, target *T) (applied bool, err error)
}

// StageFunc is a functional adapter to implement Stage.
type StageFunc[T any] func(ctx context.Context, target *T) (bool, error)

type funcStage[T any] struct {
	name string
	fn   StageFunc[T]
}

func (s funcStage[T]) Name() string                                    { return s.name }
func (s funcStage[T]) Execute(ctx context.Context, t *T) (bool, error) { return s.fn(ctx, t) }
func NewStage[T any](name string, fn StageFunc[T]) Stage[T]            { return funcStage[T]{name: name, fn: fn} }

// Backward-compatible adapters for Source/Finalizer to Stage.

type Source[T any] interface {
	Name() string
	Load(ctx context.Context, target *T) (applied bool, err error)
}

type Finalizer[T any] interface {
	Name() string
	Run(ctx context.Context, target *T) error
}

type sourceStage[T any] struct{ s Source[T] }

func (w sourceStage[T]) Name() string                                    { return w.s.Name() }
func (w sourceStage[T]) Execute(ctx context.Context, t *T) (bool, error) { return w.s.Load(ctx, t) }

func StageFromSource[T any](s Source[T]) Stage[T] { return sourceStage[T]{s: s} }

type finalizerStage[T any] struct{ f Finalizer[T] }

func (w finalizerStage[T]) Name() string { return w.f.Name() }
func (w finalizerStage[T]) Execute(ctx context.Context, t *T) (bool, error) {
	return false, w.f.Run(ctx, t)
}

func StageFromFinalizer[T any](f Finalizer[T]) Stage[T] { return finalizerStage[T]{f: f} }

// SetStrategy controls how a source writes values into the target.
// Override: always overwrite target values when the source provides a value.
// FillZero: write only if the target field is currently zero.
type SetStrategy int

const (
	SetOverride SetStrategy = iota
	SetFillZero
)

// ValidationStrategy controls how validation results are surfaced.
// AllErrors: return the aggregated error containing all issues.
// FirstError: reduce to the first encountered issue.
type ValidationStrategy int

const (
	ValidateAllErrors ValidationStrategy = iota
	ValidateFirstError
)

// StageResult captures execution metadata for a stage.
type StageResult struct {
	Name     string
	Applied  bool
	Duration time.Duration
	Err      error
}

// Pipeline orchestrates ordered execution of stages.
type Pipeline[T any] struct {
	stages []Stage[T]
}

// New constructs an empty Pipeline instance.
func New[T any]() *Pipeline[T] { return &Pipeline[T]{} }

// AddStages appends one or more stages in order.
func (p *Pipeline[T]) AddStages(st ...Stage[T]) *Pipeline[T] {
	p.stages = append(p.stages, st...)
	return p
}

// Stages returns a shallow copy of the current stage list for external composition.
func (p *Pipeline[T]) Stages() []Stage[T] {
	out := make([]Stage[T], len(p.stages))
	copy(out, p.stages)
	return out
}

// Backward-compatible helpers: wrap sources/finalizers as stages.
func (p *Pipeline[T]) AddSources(srcs ...Source[T]) *Pipeline[T] {
	for _, s := range srcs {
		p.stages = append(p.stages, StageFromSource[T](s))
	}
	return p
}
func (p *Pipeline[T]) AddFinalizers(fns ...Finalizer[T]) *Pipeline[T] {
	for _, f := range fns {
		p.stages = append(p.stages, StageFromFinalizer[T](f))
	}
	return p
}

// Execute runs all stages sequentially and stops at first error.
func (p *Pipeline[T]) Execute(ctx context.Context, target *T) (results []StageResult, err error) {
	for _, st := range p.stages {
		start := time.Now()
		applied, e := st.Execute(ctx, target)
		results = append(results, StageResult{Name: st.Name(), Applied: applied, Duration: time.Since(start), Err: e})
		if e != nil {
			return results, e
		}
	}
	return results, nil
}
