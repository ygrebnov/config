package config

import (
	"context"
	"encoding/json"
	"fmt"
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
	From(ctx context.Context) (string, error)
	To(ctx context.Context, path string) error
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

func newModelFileCodec[T any](
	binding *model.Binding[T],
	store storeService,
) *fsPkg.Codec {
	return &fsPkg.Codec{
		Decode: func(path string, data []byte) error {
			obj := new(T)
			if err := unmarshalConfigurationFile(path, data, obj); err != nil {
				return err
			}

			return binding.WriteValues(
				obj,
				storeValueSink{store: store},
			)
		},
		Encode: func(path string) ([]byte, error) {
			obj := new(T)
			if err := binding.ApplyValues(
				obj,
				storeValueSource{store: store},
			); err != nil {
				return nil, err
			}

			return marshalConfigurationFile(path, obj)
		},
	}
}

func unmarshalConfigurationFile(
	path string,
	data []byte,
	obj any,
) error {
	if filepath.Ext(path) == ".json" {
		return json.Unmarshal(data, obj)
	}

	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return err
	}

	return unmarshalYAMLValue(reflect.ValueOf(obj).Elem(), value)
}

func marshalConfigurationFile(path string, obj any) ([]byte, error) {
	if filepath.Ext(path) == ".json" {
		return json.Marshal(obj)
	}

	value, err := marshalYAMLValue(reflect.ValueOf(obj))
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(value)
}

func unmarshalYAMLValue(dst reflect.Value, value any) error {
	if value == nil {
		dst.SetZero()
		return nil
	}

	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}

		return unmarshalYAMLValue(dst.Elem(), value)
	}

	if dst.Kind() == reflect.Struct && !implementsJSONUnmarshaler(dst) {
		values, ok := value.(map[string]any)
		if !ok {
			return unmarshalJSONValue(dst, value)
		}

		for i := 0; i < dst.NumField(); i++ {
			field := dst.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}

			name, included, inline := yamlFieldName(field)
			if !included {
				continue
			}

			fieldValue := dst.Field(i)
			if inline {
				if err := unmarshalYAMLValue(fieldValue, values); err != nil {
					return err
				}
				continue
			}

			value, found := values[name]
			if !found {
				continue
			}

			if err := unmarshalYAMLValue(fieldValue, value); err != nil {
				return err
			}
		}

		return nil
	}

	switch dst.Kind() {
	case reflect.Slice:
		values, ok := value.([]any)
		if !ok {
			return unmarshalJSONValue(dst, value)
		}

		dst.Set(reflect.MakeSlice(dst.Type(), len(values), len(values)))
		for i, item := range values {
			if err := unmarshalYAMLValue(dst.Index(i), item); err != nil {
				return err
			}
		}

		return nil

	case reflect.Array:
		values, ok := value.([]any)
		if !ok {
			return unmarshalJSONValue(dst, value)
		}

		for i := range min(len(values), dst.Len()) {
			if err := unmarshalYAMLValue(dst.Index(i), values[i]); err != nil {
				return err
			}
		}

		return nil

	case reflect.Map:
		values, ok := value.(map[string]any)
		if !ok {
			return unmarshalJSONValue(dst, value)
		}

		dst.Set(reflect.MakeMapWithSize(dst.Type(), len(values)))
		for key, item := range values {
			mapKey, err := yamlMapKey(dst.Type().Key(), key)
			if err != nil {
				return err
			}

			mapValue := reflect.New(dst.Type().Elem()).Elem()
			if err := unmarshalYAMLValue(mapValue, item); err != nil {
				return err
			}

			dst.SetMapIndex(mapKey, mapValue)
		}

		return nil
	}

	return unmarshalJSONValue(dst, value)
}

func marshalYAMLValue(src reflect.Value) (any, error) {
	if src.Kind() == reflect.Pointer {
		if src.IsNil() {
			return nil, nil
		}

		return marshalYAMLValue(src.Elem())
	}

	if implementsJSONMarshaler(src) {
		return marshalJSONValue(src)
	}

	switch src.Kind() {
	case reflect.Struct:
		values := make(map[string]any)
		for i := 0; i < src.NumField(); i++ {
			field := src.Type().Field(i)
			if field.PkgPath != "" {
				continue
			}

			name, included, inline := yamlFieldName(field)
			if !included {
				continue
			}

			fieldValue := src.Field(i)
			if yamlTagOption(field, "omitempty") && fieldValue.IsZero() {
				continue
			}

			value, err := marshalYAMLValue(fieldValue)
			if err != nil {
				return nil, err
			}

			if inline {
				if value == nil {
					continue
				}

				nestedValues, ok := value.(map[string]any)
				if !ok {
					return nil, fmt.Errorf(
						"marshal inline YAML field %s as a mapping",
						field.Name,
					)
				}

				for key, nested := range nestedValues {
					values[key] = nested
				}
				continue
			}

			values[name] = value
		}

		return values, nil

	case reflect.Slice, reflect.Array:
		values := make([]any, src.Len())
		for i := 0; i < src.Len(); i++ {
			value, err := marshalYAMLValue(src.Index(i))
			if err != nil {
				return nil, err
			}
			values[i] = value
		}

		return values, nil

	case reflect.Map:
		values := make(map[string]any, src.Len())
		iter := src.MapRange()
		for iter.Next() {
			value, err := marshalYAMLValue(iter.Value())
			if err != nil {
				return nil, err
			}
			values[fmt.Sprint(iter.Key().Interface())] = value
		}

		return values, nil

	default:
		return src.Interface(), nil
	}
}

func unmarshalJSONValue(dst reflect.Value, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dst.Addr().Interface())
}

func marshalJSONValue(src reflect.Value) (any, error) {
	data, err := json.Marshal(src.Interface())
	if err != nil {
		return nil, err
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

func yamlMapKey(t reflect.Type, value string) (reflect.Value, error) {
	key := reflect.New(t).Elem()
	if t.Kind() == reflect.String {
		key.SetString(value)
		return key, nil
	}

	if err := unmarshalJSONValue(key, value); err != nil {
		return reflect.Value{}, err
	}

	return key, nil
}

func implementsJSONUnmarshaler(v reflect.Value) bool {
	jsonUnmarshaler := reflect.TypeFor[json.Unmarshaler]()
	return v.CanAddr() && v.Addr().Type().Implements(jsonUnmarshaler)
}

func implementsJSONMarshaler(v reflect.Value) bool {
	jsonMarshaler := reflect.TypeFor[json.Marshaler]()
	return v.CanInterface() && v.Type().Implements(jsonMarshaler)
}

func yamlFieldName(
	field reflect.StructField,
) (name string, included bool, inline bool) {
	tag := field.Tag.Get("yaml")
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return "", false, false
	}

	for _, option := range parts[1:] {
		if option == "inline" {
			inline = true
		}
	}

	name = parts[0]
	if name == "" {
		name = strings.ToLower(field.Name)
	}

	return name, true, inline
}

func yamlTagOption(field reflect.StructField, option string) bool {
	for _, value := range strings.Split(field.Tag.Get("yaml"), ",")[1:] {
		if value == option {
			return true
		}
	}

	return false
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

	bindingOptions := make([]model.Option, 0, 1)
	if cfg.envPrefix != "" {
		bindingOptions = append(
			bindingOptions,
			model.WithEnvPrefix(cfg.envPrefix),
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

	// Load file values into a temporary store. After model normalization, a new
	// store replaces it so stale or unknown source keys are not retained.
	loadedStore := storePkg.New()
	loader := fsPkg.New(
		fsCfg,
		loadedStore,
		cfg.streams,
		newModelFileCodec(binding, loadedStore),
	)

	path, err := loader.From(ctx)
	if err != nil {
		return nil, err
	}

	obj := new(T)

	if err := binding.ApplyValues(
		obj,
		storeValueSource{store: loadedStore},
	); err != nil {
		return nil, initializationError(err)
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

	fs := fsPkg.New(
		fsCfg,
		store,
		cfg.streams,
		newModelFileCodec(binding, store),
	)

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
func (c *Controller[T]) Get(name string) (any, bool) {
	return c.store.Get(name)
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

	return c.fs.To(ctx, options.path)
}
