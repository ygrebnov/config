package path

import (
	"os"
	"path/filepath"

	"github.com/ygrebnov/errorc"

	"github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/keys"
)

const DefaultConfigFilename = "config.yml"

func getConfigDir(appName string) (string, error) {
	userConfigDir := os.Getenv("XDG_CONFIG_HOME")
	if userConfigDir != "" {
		return filepath.Join(userConfigDir, appName), nil
	}

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", errorc.With(errors.ErrCannotResolveUserConfigDir, errorc.Error(keys.Cause, err))
	}
	return filepath.Join(userConfigDir, appName), nil
}

func GetConfigPath(appName string) (string, error) {
	configDir, err := getConfigDir(appName)
	if err != nil {
		return "", err
	}

	return filepath.Join(configDir, DefaultConfigFilename), nil
}
