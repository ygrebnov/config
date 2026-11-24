package path

import (
	nativeerrors "errors"
	"path/filepath"

	"github.com/ygrebnov/errorc"
	fspath "github.com/ygrebnov/fs/path"

	"github.com/ygrebnov/config/pkg/errors"
)

const DefaultConfigFilename = "config.yml"

// GetConfigPath resolves an application config path beneath the user's home.
// Explicit file paths should be used when a location outside home is intended.
func GetConfigPath(appName string) (string, error) {
	dir, err := fspath.ConfigDir(appName)
	if err != nil {
		switch {
		case nativeerrors.Is(err, fspath.ErrInvalidAppName):
			return "", errors.ErrInvalidAppName
		case nativeerrors.Is(err, fspath.ErrInvalidBasePath):
			return "", errorc.WithCause(errors.ErrInvalidConfigFilePath, err)
		default:
			return "", errorc.WithCause(errors.ErrCannotResolveUserConfigDir, err)
		}
	}
	if err := fspath.ValidateWithinHome(dir); err != nil {
		if nativeerrors.Is(err, fspath.ErrCannotResolveHome) {
			return "", errorc.WithCause(errors.ErrCannotResolveUserConfigDir, err)
		}
		return "", errorc.WithCause(errors.ErrInvalidConfigFilePath, err)
	}
	return filepath.Join(dir, DefaultConfigFilename), nil
}
