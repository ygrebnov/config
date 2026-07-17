package config

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	kitstreams "github.com/pumpingbytes/go-kit/streams"
	"github.com/ygrebnov/errorc"
	"github.com/ygrebnov/model"
	"github.com/ygrebnov/model/field"
	"gopkg.in/yaml.v3"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"

	fsPkg "github.com/ygrebnov/config/internal/fs"
	storePkg "github.com/ygrebnov/config/internal/store"
)

// Controller provides load/get/set/save operations for a single config object
// of type T. T must be a non-pointer struct type.
//
// Controller operates on its own representation of the configuration object.
// It does not retain or mutate a user-provided object. Load copies the current
// controller state into the provided object and validates it.
//
// Each Controller instance is independent and can manage a different
// configuration object.
//
// If configuration only needs to be loaded from a provided or well-known
// OS-specific location, use Load.
type Controller[T any] struct {
	mu sync.Mutex // Save serialization.

	envPrefix string
	path      string

	binding *model.Binding[T]

	store storeService
	fs    fsService

	streams kitstreams.IOStreams
}

type storeService interface {
	Set(key string, value any)
	Get(key string) (any, bool)
}

type fsService interface {
	From(ctx context.Context) (string, []byte, error)
	To(ctx context.Context, path string, data []byte) error
}

// storeValueSource adapts the controller store to field.ValueSource.
//
// Store keys must use model's exact exported schema paths, such as "Name",
// "Server.Host", and "Items[]".
type storeValueSource struct {
	store storeService
}

var _ field.ValueSource = storeValueSource{}

func (s storeValueSource) Get(name string) (any, bool, error) {
	value, found := s.store.Get(name)

	return value, found, nil
}

// storeValueSink adapts the controller store to field.ValueSink.
//
// WriteValues first writes the complete collection under a path such as
// "Items[]" and then visits element descendants using repeated paths such as
// "Items[].Name". The store keeps the complete collection value and ignores
// repeated descendant writes because a key-value store cannot represent them
// without concrete runtime indexes.
type storeValueSink struct {
	store storeService
}

var _ field.ValueSink = storeValueSink{}

func (s storeValueSink) Set(name string, value any) error {
	if strings.Contains(name, "[].") {
		return nil
	}

	value = nilStoreValue(value)
	s.store.Set(name, value)

	return nil
}

func nilStoreValue(value any) any {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return nil
	}

	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		if rv.IsNil() {
			return nil
		}
	}

	return value
}

func unmarshalConfigurationFile(
	path string,
	data []byte,
	obj any,
) error {
	if filepath.Ext(path) == ".json" {
		return json.Unmarshal(data, obj)
	}

	return yaml.Unmarshal(data, obj)
}

func marshalConfigurationFile(path string, obj any) ([]byte, error) {
	if filepath.Ext(path) == ".json" {
		return json.Marshal(obj)
	}

	return yaml.Marshal(obj)
}

// NewController constructs a controller configured with the provided options.
//
// The constructor is heavy and can return an error. It compiles model metadata,
// snapshots environment variables, initializes store and filesystem services,
// resolves and loads the configuration source, applies defaults and environment
// overrides, validates the resulting object, and normalizes it into the
// controller's internal store.
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
//
// NewControllerCtx is the context-aware alternative constructor.
func NewController[T any](opts ...Option) (*Controller[T], error) {
	return NewControllerCtx[T](context.Background(), opts...)
}

// NewControllerCtx is the same constructor as NewController, but allows a
// context to control cancellation and timeout.
func NewControllerCtx[T any](
	ctx context.Context,
	opts ...Option,
) (*Controller[T], error) {
	if ctx == nil {
		return nil, configerrors.ErrNilContext
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := applyOptions(opts...)

	bindingOptions := make([]model.Option, 0, 2)
	if cfg.envPrefix != "" {
		bindingOptions = append(
			bindingOptions,
			model.WithEnvPrefix(cfg.envPrefix),
		)
	}
	if len(cfg.rules) > 0 {
		bindingOptions = append(
			bindingOptions,
			model.WithRules(cfg.rules...),
		)
	}

	binding, err := model.NewBinding[T](bindingOptions...)
	if err != nil {
		return nil, err
	}

	fsCfg := &fsPkg.Config{
		Path:      cfg.path,
		EnvPrefix: cfg.envPrefix,
		AppName:   cfg.appName,
	}

	loader := fsPkg.New(fsCfg, cfg.streams)
	path, data, err := loader.From(ctx)
	if err != nil {
		return nil, err
	}

	obj := new(T)
	if data != nil {
		if err := unmarshalConfigurationFile(path, data, obj); err != nil {
			return nil, configurationFileParseError(path, err)
		}
	}

	if err := binding.ValidateWithDefaults(ctx, obj); err != nil {
		return nil, initializationError(err)
	}

	store := storePkg.New()
	if err := binding.WriteValues(
		obj,
		storeValueSink{store: store},
	); err != nil {
		return nil, initializationError(err)
	}

	fs := fsPkg.New(fsCfg, cfg.streams)

	return &Controller[T]{
		envPrefix: cfg.envPrefix,
		path:      path,
		binding:   binding,
		store:     store,
		fs:        fs,
		streams:   cfg.streams,
	}, nil
}

func initializationError(err error) error {
	return errorc.With(
		configerrors.ErrCannotInitializeConfigurationObject,
		errorc.Error(configkeys.Cause, err),
	)
}

func configurationFileParseError(path string, err error) error {
	return errorc.With(
		configerrors.ErrParse,
		errorc.String(configkeys.Path, path),
		errorc.Error(configkeys.Cause, err),
	)
}

// Load replaces obj with the current controller state and validates it.
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

	// ApplyValues leaves fields unchanged when the source does not contain a
	// value. Reset the target first so Load has replacement rather than merge
	// semantics.
	var zero T
	*obj = zero

	if err := c.binding.ApplyValues(
		obj,
		storeValueSource{store: c.store},
	); err != nil {
		return loadError(err)
	}

	if err := c.binding.Validate(ctx, obj); err != nil {
		return loadError(err)
	}

	return nil
}

func loadError(err error) error {
	return errorc.With(
		configerrors.ErrCannotLoadConfigurationIntoProvidedObject,
		errorc.Error(configkeys.Cause, err),
	)
}

// Get returns a configuration option value by exact exported model path.
//
// It returns ErrConfigurationOptionNotFound when name is not a known path.
// A configuration option with a nil value is returned as nil with no error.
func (c *Controller[T]) Get(name string) (any, error) {
	value, found := c.store.Get(name)
	if found {
		return value, nil
	}

	return nil, errorc.With(
		configerrors.ErrConfigurationOptionNotFound,
		errorc.String(configkeys.OptionPath, name),
	)
}

// Set updates a configuration option value by exact exported model path.
//
// The value is updated only in the internal store. Call Save to persist it.
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

// Save writes the controller's current internal store to disk.
//
// If no path is provided through WithSavePath, Save uses the path resolved for
// the Controller. ErrPathNotConfigured is returned when no path is available.
func (c *Controller[T]) Save(
	ctx context.Context,
	opts ...SaveOption,
) error {
	if ctx == nil {
		return configerrors.ErrNilContext
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	options := &saveOptions{
		path: c.path,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}

	if options.path == "" {
		return configerrors.ErrPathNotConfigured
	}

	obj := new(T)
	if err := c.binding.ApplyValues(
		obj,
		storeValueSource{store: c.store},
	); err != nil {
		return err
	}

	data, err := marshalConfigurationFile(options.path, obj)
	if err != nil {
		return err
	}

	return c.fs.To(ctx, options.path, data)
}
