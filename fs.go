// Package config: filesystem path helpers.
package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ygrebnov/errorc"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
)

// EnsurePath ensures the directories for a file path exist and the path does not already exist as a directory.
func EnsurePath(path string) error {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.IsDir() {
			return errorc.With(configerrors.ErrInaccessiblePath, errorc.String(configkeys.Path, path))
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return errorc.With(
			configerrors.ErrInaccessiblePath,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errorc.With(
			configerrors.ErrCannotCreateDirectories,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	return nil
}
