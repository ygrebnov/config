package config

import (
	"strings"

	kitstreams "github.com/pumpingbytes/go-kit/streams"
	"github.com/ygrebnov/model"
)

// Option configures the config loading process.
type Option func(*settings)

type settings struct {
	path      string
	appName   string
	envPrefix string
	streams   kitstreams.IOStreams
	rules     []model.Rule
}

func applyOptions(opts ...Option) settings {
	cfg := settings{streams: kitstreams.NewSilent()}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
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

// WithStreams wires user-facing message streams.
// nil streams are ignored.
func WithStreams(streams kitstreams.IOStreams) Option {
	return func(cfg *settings) {
		if streams != nil {
			cfg.streams = streams
		}
	}
}
