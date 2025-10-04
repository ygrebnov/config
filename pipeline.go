package config

import (
	"context"
	"time"
)

// Source represents a configuration data source that can mutate the target config.
// Implementations should perform idempotent operations (safe if called once in initialization path).
// The Provider ensures each source runs only once via its own sync.Once gate.
// Returned applied indicates whether the source actually changed any field values (best effort).
// Errors should be wrapped with context; failing sources abort initialization (fail-fast strategy).
type Source[T any] interface {
	Name() string
	Load(ctx context.Context, target *T) (applied bool, err error)
}

// Finalizer performs post-source checks (e.g., validation). It must not mutate
// the config (or should do so only in controlled, documented ways). Any error aborts init.
type Finalizer[T any] interface {
	Name() string
	Run(ctx context.Context, target *T) error
}

// SetStrategy controls how a source writes values into the target.
// Override: always overwrite target values when the source provides a value.
// FillZero: write only if the target field is currently zero (leave non-zero values intact).
// Note: At the moment this is used by EnvSource; Model defaults implicitly act like FillZero.
type SetStrategy int

const (
	SetOverride SetStrategy = iota
	SetFillZero
)

// ValidationStrategy controls how validation results are surfaced.
// AllErrors: return the aggregated *ValidationError containing all issues (default).
// FirstError: reduce to the first encountered issue (still return *ValidationError with one item).
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
// For now it is internal to the Provider; later we may expose a builder style API.
type Pipeline[T any] struct {
	sources    []Source[T]
	finalizers []Finalizer[T]
}

// Execute runs all sources sequentially, then all finalizers. Stops at first error.
func (p *Pipeline[T]) Execute(ctx context.Context, target *T) (results []SourceResult, err error) {
	start := time.Now()
	_ = start
	for _, s := range p.sources {
		st := time.Now()
		applied, e := s.Load(ctx, target)
		results = append(results, SourceResult{Name: s.Name(), Applied: applied, Duration: time.Since(st), Err: e})
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
