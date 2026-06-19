package config

import (
	"context"
	"encoding/json"
	"sync"

	kitstreams "github.com/pumpingbytes/go-kit/streams"
	"github.com/ygrebnov/errorc"
	"github.com/ygrebnov/model"
	"gopkg.in/yaml.v3"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"

	fsPkg "github.com/ygrebnov/config/internal/fs"
	storePkg "github.com/ygrebnov/config/internal/store"
)

// Controller provides load/get/set/save operations for a single config object of type T.
// Controller operates on its own representation of the configuration object.
// It does not mutate the user-provided object. It can only load configuration into that object.
//
// Each new instance of the Controller is independent and can be used to manage a different configuration object.
//
// If you need to just load the configuration from the provided or well-known OS-specific location,
// it is recommended to use Load.
//
// A type-safe version of Controller, ControllerTyped[T any], is planned for future releases.
type Controller[T any] struct {
	mu sync.Mutex // Load vs Save.

	envPrefix string
	path      string // path to file holding configuration.

	binding *model.Binding[T]

	store storeService // internal store.
	fs    fsService    // filesystem operations.

	streams kitstreams.IOStreams
}

type storeService interface {
	Set(key string, value any)
	Get(key string) (any, bool)

	GetJSON() ([]byte, error)
}

type fsService interface {
	From(ctx context.Context) (string, error)
	To(context.Context, string) error
}

// NewController constructs a controller configured with the provided options.
//
// The constructor is heavy and can return an error. It initializes store and filesystem services,
// resolves configuration source file path and attempts to load configuration from that file into its store.
//
// Below-mentioned options modify Controller behavior.
//
// WithPath sets an explicit config file path. It takes precedence over env and app-name resolution.
//
// WithEnvPrefix sets configuration file path to ${PREFIX}_CONFIG_PATH, if it is non-empty.
// It also enables ${PREFIX}_{FULL_NAME} configuration settings resolving.
// It takes precedence over app-name resolution.
//
// WithAppName sets configuration file path to a well-known OS-specific user config location:
// <user-config-dir>/<app>/config.yml.
//
// WithStreams option allows writing configuration operations messages to the provided streams.
//
// NewControllerCtx alternative constructor allows providing a context for cancellation and timeout control.
func NewController[T any](opts ...Option) (*Controller[T], error) {
	return NewControllerCtx[T](context.Background(), opts...)
}

// NewControllerCtx is the same constructor as NewController,
// but allows providing a context for cancellation and timeout control.
func NewControllerCtx[T any](ctx context.Context, opts ...Option) (*Controller[T], error) {
	if ctx == nil {
		return nil, configerrors.ErrNilContext
	}

	cfg := applyOptions(opts...)
	// TODO: add WithValidationRules option (just pass through model.validation rules)

	// create binding
	binding, err := model.NewBinding[T](model.WithEnvPrefix(cfg.envPrefix))
	if err != nil {
		return nil, err
	}

	// initialize store
	store := storePkg.New()

	fsCfg := &fsPkg.Config{
		Path:      cfg.path,
		EnvPrefix: cfg.envPrefix,
		AppName:   cfg.appName,
	}
	fs := fsPkg.New(fsCfg, store, cfg.streams)

	path, err := fs.From(ctx)
	if err != nil {
		return nil, err
	}

	b, err := store.GetJSON()
	if err != nil {
		return nil, err
	}

	// apply defaults via struct tags and environment variables and validate
	obj := new(T)

	err = yaml.Unmarshal(b, obj)
	if err != nil {
		return nil, errorc.With(
			configerrors.ErrCannotInitializeConfigurationObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	err = binding.ValidateWithDefaults(ctx, obj)
	if err != nil {
		return nil, errorc.With(
			configerrors.ErrCannotInitializeConfigurationObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	// load object back to store
	bytes, err := yaml.Marshal(obj)
	if err != nil {
		return nil, errorc.With(
			configerrors.ErrCannotInitializeConfigurationObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	err = store.FromBytes(bytes)
	if err != nil {
		return nil, errorc.With(
			configerrors.ErrCannotInitializeConfigurationObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	return &Controller[T]{
		envPrefix: cfg.envPrefix,
		store:     store,
		fs:        fs,
		path:      path,
		binding:   binding,
		streams:   cfg.streams,
	}, nil
}

// Load populates provided object with data from the store.
func (c *Controller[T]) Load(ctx context.Context, obj *T) error {
	if ctx == nil {
		return configerrors.ErrNilContext
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if obj == nil {
		return configerrors.ErrNilTarget
	}

	b, err := c.store.GetJSON()
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		return errorc.With(
			configerrors.ErrCannotLoadConfigurationIntoProvidedObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	err = c.binding.Validate(ctx, obj)
	if err != nil {
		return errorc.With(
			configerrors.ErrCannotLoadConfigurationIntoProvidedObject,
			errorc.Error(configkeys.Cause, err),
		)
	}

	return nil
}

// Get returns a configuration option value by dotted name.
func (c *Controller[T]) Get(name string) (any, bool) {
	return c.store.Get(name)
}

// Set updates a configuration option value by dotted name.
//
// Note: value is updated only in the internal store. To persist it, call Save.
func (c *Controller[T]) Set(name string, value any) {
	c.store.Set(name, value)
}

type saveOptions struct {
	path string
}

type SaveOption func(*saveOptions)

func WithSavePath(path string) SaveOption {
	return func(opts *saveOptions) {
		opts.path = path
	}
}

// Save writes configuration to disk.
//
// If no path is provided via WithSavePath, it will use the path configured in the Controller.
// ErrPathNotConfigured error is returned if the path is empty/not resolved.
func (c *Controller[T]) Save(ctx context.Context, opts ...SaveOption) error {
	if ctx == nil {
		return configerrors.ErrNilContext
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	options := &saveOptions{path: c.path}
	for _, opt := range opts {
		opt(options)
	}

	if options.path == "" {
		return configerrors.ErrPathNotConfigured
	}

	return c.fs.To(ctx, options.path)
}
