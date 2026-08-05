package config

import (
	"strings"

	"github.com/ygrebnov/model"

	"github.com/ygrebnov/config/pkg/log"
)

// Option configures the config loading process.
type Option func(*settings)

type settings struct {
	path      string
	appName   string
	envPrefix string
	logger    log.Logger
	rules     []model.Rule
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
	return cfg, nil
}

// WithPath sets an explicit config file path. It takes precedence over env and app-name resolution.
// Empty path is ignored.
func WithPath(path string) Option {
	path = strings.TrimSpace(path)
	return func(cfg *settings) {
		if path != "" {
			cfg.path = path
		}
	}
}

// WithAppName enables loading from a well-known user config location: <user-config-dir>/<app>/config.yml.
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
