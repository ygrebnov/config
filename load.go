package config

import (
	"context"
)

// Load is a convenience wrapper over a Controller construction and calling its Load method.
//
// WithPath sets an explicit config file path. It takes precedence over env and
// app-name resolution.
//
// WithEnvPrefix sets the configuration file path to ${PREFIX}_CONFIG_PATH when
// non-empty. It also enables ${PREFIX}_{FULL_NAME} configuration values. Model
// snapshots matching environment variables during controller construction.
//
// WithValidationRules registers custom model validation rules used during
// initialization and Load.
//
// WithAppName sets the configuration file path to a well-known OS-specific user
// config location: <user-config-dir>/<app>/config.yml.
//
// WithStreams allows configuration operation messages to be written to the
// provided streams.
func Load[T any](ctx context.Context, dst *T, opts ...Option) error {
	c, err := NewController[T](opts...)
	if err != nil {
		return err
	}

	return c.Load(ctx, dst)
}
