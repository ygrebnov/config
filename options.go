package config

import (
	"math"
	"strings"

	sharedfs "github.com/ygrebnov/fs"
	"github.com/ygrebnov/model"

	"github.com/ygrebnov/config/pkg/log"
)

// Option configures the config loading process.
type Option func(*settings)

type settings struct {
	path        string
	appName     string
	envPrefix   string
	logger      log.Logger
	rules       []model.Rule
	maxFileSize int64
	optionErr   error
}

func applyOptions(opts ...Option) (settings, error) {
	logger, err := log.NewSilentLogger()
	if err != nil {
		return settings{}, err
	}

	cfg := settings{logger: logger}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.optionErr != nil {
		return settings{}, cfg.optionErr
	}
	return cfg, nil
}

// WithMaxFileSize limits the configuration file read to size bytes.
// The default is 1 MiB. Size must be positive and less than math.MaxInt64.
func WithMaxFileSize(size int64) Option {
	return func(cfg *settings) {
		if size <= 0 || size == math.MaxInt64 {
			cfg.optionErr = sharedfs.ErrInvalidFileSizeLimit
			return
		}
		cfg.maxFileSize = size
		cfg.optionErr = nil
	}
}

// WithPath sets an explicit config file path. It takes precedence over env and
// app-name resolution. The parent directory must exist; the file may be absent.
// Empty path is ignored.
func WithPath(path string) Option {
	path = strings.TrimSpace(path)
	return func(cfg *settings) {
		if path != "" {
			cfg.path = path
		}
	}
}

// WithAppName enables loading from a well-known user config location:
// <user-config-dir>/<app>/config.yml. The app name must be a single valid
// directory name, and the resolved config directory must be inside the user's
// home directory.
// Empty appName is ignored.
func WithAppName(appName string) Option {
	appName = strings.TrimSpace(appName)
	return func(cfg *settings) {
		if appName != "" {
			cfg.appName = appName
		}
	}
}

// WithEnvPrefix enables environment overrides and ${PREFIX}_CONFIG_PATH path resolution.
// Empty prefix is ignored.
func WithEnvPrefix(prefix string) Option {
	prefix = strings.TrimSpace(prefix)
	return func(cfg *settings) {
		if prefix != "" {
			cfg.envPrefix = prefix
		}
	}
}

// WithValidationRules registers custom model validation rules for Controller.
func WithValidationRules(rules ...model.Rule) Option {
	rules = append([]model.Rule(nil), rules...)

	return func(cfg *settings) {
		cfg.rules = append(cfg.rules, rules...)
	}
}

// WithLogger wires a custom logger to the config loader. If nil, a silent logger is used.
func WithLogger(logger log.Logger) Option {
	return func(cfg *settings) {
		if logger != nil {
			cfg.logger = logger
		}
	}
}
