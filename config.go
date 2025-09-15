package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	modellib "github.com/ygrebnov/model"

	"github.com/ygrebnov/config/streams"
)

const (
	configFileName = "config.yml"
	envVarTagName  = "env"
)

// Exported error categories returned by this package. These are used with wrapping
// so callers can detect error classes using errors.Is/As.
//   - ErrEnsureConfigDir: failure to create parent directories for a config file.
//   - ErrUnsupportedConfigFileType: file extension is neither .yaml/.yml nor .json.
//   - ErrParse: failure to parse an existing config file.
//   - ErrFormat: failure to marshal a config to bytes (e.g., unsupported type).
//   - ErrWrite: failure to write the config file to disk.
var (
	ErrEnsureConfigDir           = errors.New("ensure config dir")
	ErrUnsupportedConfigFileType = errors.New("unsupported config file type")
	ErrParse                     = errors.New("parse config file")
	ErrFormat                    = errors.New("format config")
	ErrWrite                     = errors.New("write to config file")
)

// Provider manages the lifecycle of a configuration object of type T.
//
// A Provider[T] performs the following steps exactly once (it is safe to call Get
// from multiple goroutines):
//  1. Construct a new *T using the factory set via WithDefaultFn (or a zero-value fallback).
//  2. If WithModel is set, bind a model.Model[T] to the same *T and call SetDefaults()
//     to populate zero values using `default` struct tags.
//  3. Resolve the configuration file path from either ${ENV_PREFIX}_CONFIG_PATH or
//     a standard user config directory (if persistence is enabled with WithPersistence).
//  4. Load overrides from the resolved file if it exists (or create it if persistent and missing).
//  5. Apply environment overrides using `env` struct tags (or field name in SCREAMING_SNAKE_CASE).
//  6. If WithModel was set, validate the final object using model.Validate().
//
// Subsequent calls to Get() return the same pointer and metadata.
type Provider[T any] struct {
	mu          sync.RWMutex
	initOnce    sync.Once
	persist     bool
	dirName     string
	envPrefix   string
	configPath  string
	cfg         *T
	defaultFn   func() *T
	streams     streams.IOStreams
	fileCreated bool
	initErr     error
	modelInit   ModelInit[T]
	model       *modellib.Model[T]
}

// Option configures a Provider at construction time. Options are composable and
// can be passed to New in any order.
type Option[T any] func(*Provider[T])

// New constructs a Provider[T] and applies all given options.
// If no WithDefaultFn is provided, New uses a zero-value factory that returns
// a new *T with all fields zeroed.
func New[T any](opts ...Option[T]) *Provider[T] {
	p := &Provider[T]{}
	for _, opt := range opts {
		opt(p)
	}

	if p.defaultFn == nil {
		// Must be a pointer to a struct for reflection logic
		p.defaultFn = func() *T { var t T; return &t }
	}

	return p
}

// WithPersistence enables reading/writing the config file under a directory
// named `dirName` inside the OS user config directory (e.g. XDG_CONFIG_HOME/<dirName>/config.yml).
// The provider will attempt to create the file with defaults when it does not exist.
// Panics if dirName is empty.
func WithPersistence[T any](dirName string) Option[T] {
	return func(m *Provider[T]) {
		if dirName == "" {
			panic("config: WithPersistence: dirName cannot be empty")
		}
		m.persist = true
		m.dirName = dirName
	}
}

// WithEnvPrefix sets the prefix used for environment overrides, e.g. "MYAPP".
// When set, Provider also honors ${PREFIX}_CONFIG_PATH as an absolute path to
// the config file, which takes precedence over persistence.
// Panics if prefix is empty.
func WithEnvPrefix[T any](prefix string) Option[T] {
	return func(m *Provider[T]) {
		if prefix == "" {
			panic("config: WithEnvPrefix: prefix cannot be empty")
		}
		m.envPrefix = prefix
	}
}

// WithDefaultFn registers a factory that returns a new *T. The factory is invoked
// once during Get() to construct the base configuration object before any file
// or environment overrides are applied. Panics if fn is nil.
func WithDefaultFn[T any](fn func() *T) Option[T] {
	return func(m *Provider[T]) {
		if fn == nil {
			panic("config: WithDefaultFn: fn cannot be nil")
		}
		m.defaultFn = fn
	}
}

// WithStreams wires user-facing message streams (e.g., for "created new config"/
// "loaded from" notifications and non-fatal warnings). Pass adapters from the
// companion streams package to route output to buffers, logs, or io.Discard.
func WithStreams[T any](streams streams.IOStreams) Option[T] {
	return func(m *Provider[T]) {
		m.streams = streams
	}
}

// ModelInit is a constructor hook that binds a model.Model[T] to the Provider-managed
// *T. It allows the Provider to call SetDefaults() before file/env and Validate()
// after file/env. Return the constructed model.Model[T] or an error.
type ModelInit[T any] func(*T) (*modellib.Model[T], error)

// WithModel enables integration with github.com/ygrebnov/model. The provided init
// function is called exactly once during the first Get() to build a model.Model[T]
// bound to the Provider's *T. The Provider will then:
//   - call SetDefaults() before loading from file and env, and
//   - call Validate() after all overrides are applied.
//
// Panics if init is nil.
func WithModel[T any](init ModelInit[T]) Option[T] {
	return func(m *Provider[T]) {
		if init == nil {
			panic("config: WithModel: init cannot be nil")
		}
		m.modelInit = init
	}
}

// Get initializes and returns the final configuration pointer, the resolved file
// path (if any), whether the file was created on this run, and an error if initialization
// failed. Get is safe for concurrent use; initialization runs at most once.
func (m *Provider[T]) Get() (cfg *T, path string, fileCreated bool, err error) {
	m.initOnce.Do(func() {
		// 1) Construct default config instance
		m.cfg = m.defaultFn()

		// 2) Optionally construct model wrapper around config instance
		// to apply defaults before file/env operations.
		if m.modelInit != nil {
			mdl, err := m.modelInit(m.cfg)
			if err != nil {
				m.initErr = err
				return
			}
			m.model = mdl

			// Apply defaults before file/env, so they only fill zero values.
			if err := m.model.SetDefaults(); err != nil {
				m.initErr = err
				return
			}
		}

		// 3) Resolve config path. If this fails, abort initialization; otherwise continue
		// into file operations and env overrides.
		if err := m.resolveConfigPath(); err != nil {
			m.initErr = err
			return
		}

		// 4) File operations
		// Attempt to read from file if it exists. In persistent mode, create if missing.
		e := loadFromFile(m.configPath, m.cfg)
		switch {
		case e != nil && !errors.Is(e, os.ErrNotExist):
			m.initErr = e

		case e != nil && errors.Is(e, os.ErrNotExist) && m.persist:
			if pe := EnsurePath(m.configPath); pe != nil {
				m.initErr = errors.Join(ErrEnsureConfigDir, pe)
				return
			}

			if we := writeToFile(m.configPath, m.cfg); we != nil {
				m.initErr = errors.Join(ErrWrite, we)
				return
			}
			m.fileCreated = true
			if m.streams != nil && m.streams.Out() != nil {
				fmt.Fprintf(m.streams.Out(), "config: created new config at %s\n", m.configPath)
			}
		case e == nil && m.persist:
			if m.streams != nil && m.streams.Out() != nil {
				fmt.Fprintf(m.streams.Out(), "config: loaded from %s\n", m.configPath)
			}
		}

		// 5) Apply environment overrides
		m.loadFromEnv(m.cfg)

		// 6) Optionally apply model validation after file/env operations.
		if m.model != nil {
			if err := m.model.Validate(); err != nil {
				m.initErr = err
				return
			}
		}
	})

	// After once: return cached state or error
	if m.initErr != nil {
		return nil, "", false, m.initErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg, m.configPath, m.fileCreated, nil
}

func (m *Provider[T]) resolveConfigPath() error {
	if m.envPrefix != "" {
		if configPath := os.Getenv(m.envPrefix + "_CONFIG_PATH"); configPath != "" {
			m.configPath = configPath
			return nil
		}
	}
	if m.dirName == "" {
		// Non-persistent mode.
		return nil
	}
	// Prefer XDG_CONFIG_HOME explicitly when set, then fall back to os.UserConfigDir.
	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir == "" {
		var err error
		userConfigDir, err = os.UserConfigDir()
		if err != nil {
			// Critical when persistent; otherwise emit a note to streams if available.
			if m.persist {
				return fmt.Errorf("cannot determine user config dir: %w", err)
			}
			if m.streams != nil && m.streams.ErrOut() != nil {
				fmt.Fprintf(
					m.streams.ErrOut(),
					"config: warning: cannot determine user config dir (%v); proceeding without reading a config file\n",
					err,
				)
			}
			// Non-persistent: continue without setting a path.
			return nil
		}
	}
	m.configPath = filepath.Join(userConfigDir, m.dirName, configFileName)
	return nil
}

func (m *Provider[T]) loadFromEnv(cfg *T) {
	rv := reflect.ValueOf(cfg)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return
	}
	applyEnv(rv.Elem(), m.envPrefix, nil)
}
