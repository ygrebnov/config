package config

import (
	"errors"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestApplyOptions_DefaultStrategies(t *testing.T) {
	cfg := applyOptions[testCfg]()
	if cfg.envSetStrategy != SetOverride {
		t.Fatalf("unexpected env strategy: %v", cfg.envSetStrategy)
	}
	if cfg.validationStrategy != ValidateAllErrors {
		t.Fatalf("unexpected validation strategy: %v", cfg.validationStrategy)
	}
}

func TestValidateSettings(t *testing.T) {
	tests := []struct {
		name string
		cfg  settings[testCfg]
		err  error
	}{
		{name: "empty path", cfg: applyOptions[testCfg](WithPath[testCfg]("")), err: configerrors.ErrEmptyPath},
		{name: "empty app name", cfg: applyOptions[testCfg](WithAppName[testCfg]("")), err: configerrors.ErrEmptyAppName},
		{name: "empty env prefix", cfg: applyOptions[testCfg](WithEnvPrefix[testCfg]("")), err: configerrors.ErrEmptyEnvPrefix},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSettings(tt.cfg); !errors.Is(err, tt.err) {
				t.Fatalf("expected %v, got %v", tt.err, err)
			}
		})
	}
}

