package config

import (
	"os"
	"path/filepath"
	"strings"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
	"github.com/ygrebnov/errorc"
)

func resolveConfigPath[T any](cfg settings[T]) (string, error) {
	if cfg.path != "" {
		return cfg.path, nil
	}

	if cfg.envPrefix != "" {
		if envPath := strings.TrimSpace(os.Getenv(cfg.envPrefix + "_CONFIG_PATH")); envPath != "" {
			return envPath, nil
		}
	}

	if cfg.appName == "" {
		return "", nil
	}

	userConfigDir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if userConfigDir == "" {
		var err error
		userConfigDir, err = os.UserConfigDir()
		if err != nil {
			return "", errorc.With(
				configerrors.ErrCannotResolveConfigPath,
				errorc.String(configkeys.AppName, cfg.appName),
				errorc.Error(configkeys.Cause, err),
			)
		}
	}

	return filepath.Join(userConfigDir, cfg.appName, configFileName), nil
}
