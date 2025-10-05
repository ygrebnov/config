// Package pipeline defines the core configuration pipeline abstractions
// (sources, finalizers, execution engine) and strategy enums used by
// the config Provider.
package pipeline

import (
	"context"
	"time"
)

// Source represents a configuration data source that can mutate the target config.
// Implementations should perform idempotent operations (the Provider runs once).
// Applied indicates whether any change occurred (best effort).
type Source[T any] interface {
	Name() string
	Load(ctx context.Context, target *T) (applied bool, err error)
}

// Finalizer performs post-source checks (e.g., validation). It must avoid
// unbounded mutation. Any error aborts initialization.
type Finalizer[T any] interface {
	Name() string
	Run(ctx context.Context, target *T) error
}

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

// SourceResult captures execution metadata for a source.
type SourceResult struct {
	Name     string
	Applied  bool
	Duration time.Duration
	Err      error
}

// Pipeline orchestrates ordered execution of sources then finalizers.
// For now it is internal to the Provider; may be exposed in a builder later.
type Pipeline[T any] struct {
	sources    []Source[T]
	finalizers []Finalizer[T]
}

// New constructs an empty Pipeline instance.
func New[T any]() *Pipeline[T] { return &Pipeline[T]{} }

// AddSources appends one or more sources in order.
func (p *Pipeline[T]) AddSources(srcs ...Source[T]) *Pipeline[T] {
	p.sources = append(p.sources, srcs...)
	return p
}

// AddFinalizers appends one or more finalizers in order.
func (p *Pipeline[T]) AddFinalizers(fns ...Finalizer[T]) *Pipeline[T] {
	p.finalizers = append(p.finalizers, fns...)
	return p
}

// Execute runs all sources sequentially, then all finalizers. Stops at first error.
func (p *Pipeline[T]) Execute(ctx context.Context, target *T) (results []SourceResult, err error) {
	for _, s := range p.sources {
		applied, e := s.Load(ctx, target)
		results = append(results, SourceResult{Name: s.Name(), Applied: applied, Err: e})
		if e != nil {
			return results, e
		}
	}
	for _, f := range p.finalizers {
		if e := f.Run(ctx, target); e != nil {
			return results, e
		}
	}
	return results, nil
}
