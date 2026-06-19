package config

import (
	"context"
)

// Load is a convenience wrapper over a Controller construction and calling its Load method.
func Load[T any](ctx context.Context, dst *T, opts ...Option) error {
	c, err := NewController[T](opts...)
	if err != nil {
		return err
	}

	return c.Load(ctx, dst)
}
