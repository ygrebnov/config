package config

import (
	"strings"

	kitstreams "github.com/pumpingbytes/go-kit/streams"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

const (
	configFileName = "config.yml"
	envVarTagName  = "env"
)

// SetStrategy controls how environment values are applied to the target.
type SetStrategy int

const (
	SetOverride SetStrategy = iota
	SetFillZero
)

// ValidationStrategy controls how validation failures are surfaced.
type ValidationStrategy int

const (
	ValidateAllErrors ValidationStrategy = iota
	ValidateFirstError
)

// Option configures the config loading process.
type Option[T any] func(*settings[T])

type settings[T any] struct {
	path               string
	hasPath            bool
	appName            string
	hasAppName         bool
	envPrefix          string
	hasEnvPrefix       bool
	streams            kitstreams.IOStreams
	useModelBinding    bool
	envSetStrategy     SetStrategy
	validationStrategy ValidationStrategy
}

func defaultSettings[T any]() settings[T] {
	return settings[T]{
		envSetStrategy:     SetOverride,
		validationStrategy: ValidateAllErrors,
	}
}

func applyOptions[T any](opts ...Option[T]) settings[T] {
	cfg := defaultSettings[T]()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

// WithPath sets an explicit config file path. It takes precedence over env and app-name resolution.
func WithPath[T any](path string) Option[T] {
	path = strings.TrimSpace(path)
	return func(cfg *settings[T]) {
		cfg.hasPath = true
		cfg.path = path
	}
}

// WithAppName enables loading from a well-known user config location: <user-config-dir>/<app>/config.yml.
func WithAppName[T any](appName string) Option[T] {
	appName = strings.TrimSpace(appName)
	return func(cfg *settings[T]) {
		cfg.hasAppName = true
		cfg.appName = appName
	}
}

// WithEnvPrefix enables environment overrides and ${PREFIX}_CONFIG_PATH path resolution.
func WithEnvPrefix[T any](prefix string) Option[T] {
	prefix = strings.TrimSpace(prefix)
	return func(cfg *settings[T]) {
		cfg.hasEnvPrefix = true
		cfg.envPrefix = prefix
	}
}

// WithStreams wires user-facing message streams.
func WithStreams[T any](streams kitstreams.IOStreams) Option[T] {
	return func(cfg *settings[T]) {
		cfg.streams = streams
	}
}

// WithModel enables defaults and validation directly from github.com/ygrebnov/model tags using a cached Binding[T].
func WithModel[T any]() Option[T] {
	return func(cfg *settings[T]) {
		cfg.useModelBinding = true
	}
}

// WithEnvSetStrategy controls how environment values are applied.
func WithEnvSetStrategy[T any](strategy SetStrategy) Option[T] {
	return func(cfg *settings[T]) {
		cfg.envSetStrategy = strategy
	}
}

// WithValidationStrategy controls how validation errors are reduced.
func WithValidationStrategy[T any](strategy ValidationStrategy) Option[T] {
	return func(cfg *settings[T]) {
		cfg.validationStrategy = strategy
	}
}

func validateSettings[T any](cfg settings[T]) error {
	switch {
	case cfg.hasPath && cfg.path == "":
		return configerrors.ErrEmptyPath
	case cfg.hasAppName && cfg.appName == "":
		return configerrors.ErrEmptyAppName
	case cfg.hasEnvPrefix && cfg.envPrefix == "":
		return configerrors.ErrEmptyEnvPrefix
	default:
		return nil
	}
}

