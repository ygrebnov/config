package config

import (
	"context"
	"fmt"
	"sync"

	kitstreams "github.com/pumpingbytes/go-kit/streams"
	"github.com/ygrebnov/errorc"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
)

// Controller provides load/get/set/save operations for a single config target.
type Controller[T any] struct {
	mu       sync.RWMutex
	settings settings[T]
	target   *T
	path     string
	access   *accessMeta
	state    *loadState
}

// NewController constructs a controller configured with the provided options.
func NewController[T any](opts ...Option[T]) *Controller[T] {
	return &Controller[T]{settings: applyOptions(opts...), state: &loadState{}}
}

// Load populates dst and binds it to the controller for future Get/Set/Save calls.
func (c *Controller[T]) Load(ctx context.Context, dst *T) error {
	if err := validateTarget(dst); err != nil {
		return err
	}

	meta, err := getAccessMeta(dst)
	if err != nil {
		return err
	}

	c.mu.Lock()
	state := c.state
	if state == nil {
		state = &loadState{}
		c.state = state
	}
	settings := c.settings
	c.mu.Unlock()

	if err := validateSettings(settings); err != nil {
		return err
	}

	state.once.Do(func() {
		state.result, state.err = loadInto(ctx, dst, settings)
	})
	if state.err != nil {
		return state.err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.target = dst
	c.path = state.result.path
	c.access = meta
	c.state = state
	return nil
}

// Get returns a config option by dotted name.
func (c *Controller[T]) Get(name string) (any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.target == nil || c.access == nil {
		return nil, configerrors.ErrControllerNotLoaded
	}
	return c.access.get(c.target, name)
}

// Set updates a config option by dotted name.
func (c *Controller[T]) Set(name string, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.target == nil || c.access == nil {
		return configerrors.ErrControllerNotLoaded
	}
	return c.access.set(c.target, name, value)
}

// Save persists the currently bound config target.
func (c *Controller[T]) Save(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.target == nil {
		return configerrors.ErrControllerNotLoaded
	}

	path := c.path
	if path == "" {
		resolved, err := resolveConfigPath(c.settings)
		if err != nil {
			return err
		}
		path = resolved
	}
	if path == "" {
		return configerrors.ErrPathNotConfigured
	}

	if err := EnsurePath(path); err != nil {
		return err
	}
	if err := writeToFile(path, c.target); err != nil {
		return err
	}

	c.path = path
	writeOut(c.settings.streams, "config: saved to %s\n", path)
	return nil
}

func writeOut(streams kitstreams.IOStreams, format string, args ...any) {
	if streams == nil || streams.Out() == nil {
		return
	}
	_, _ = fmt.Fprintf(streams.Out(), format, args...)
}

func writeErr(streams kitstreams.IOStreams, format string, args ...any) {
	if streams == nil || streams.ErrOut() == nil {
		return
	}
	_, _ = fmt.Fprintf(streams.ErrOut(), format, args...)
}

func optionNotFound(name string) error {
	return errorc.With(configerrors.ErrOptionNotFound, errorc.String(configkeys.OptionPath, name))
}
