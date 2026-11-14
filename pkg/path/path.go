package path

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ygrebnov/errorc"

	"github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/keys"
)

const (
	DefaultConfigFilename = "config.yml"
	parentDir             = ".."
)

// GetConfigPath resolves an application config path beneath the user's home.
// Explicit file paths should be used when a location outside home is intended.
func GetConfigPath(appName string) (string, error) {
	if err := validateAppName(appName); err != nil {
		return "", err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", errorc.With(errors.ErrCannotResolveUserConfigDir, errorc.Error(keys.Cause, err))
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base, err = os.UserConfigDir()
		if err != nil {
			return "", errorc.With(
				errors.ErrCannotResolveUserConfigDir,
				errorc.Error(keys.Cause, err),
			)
		}
	}

	if !filepath.IsAbs(base) {
		return "", errorc.With(
			errors.ErrInvalidConfigFilePath,
			errorc.String(keys.Path, base),
		)
	}

	rel, err := filepath.Rel(home, base)
	if err != nil ||
		rel == parentDir ||
		filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, parentDir+string(filepath.Separator)) {
		return "", errorc.With(
			errors.ErrInvalidConfigFilePath,
			errorc.String(keys.Path, base),
		)
	}

	return filepath.Join(base, appName, DefaultConfigFilename), nil
}

func validateAppName(name string) error {
	if name == "" ||
		name == "." ||
		name == parentDir ||
		strings.TrimSpace(name) != name ||
		strings.ContainsAny(name, `/\`) ||
		filepath.IsAbs(name) ||
		strings.ContainsAny(name, `<>:"|?*`) ||
		strings.HasSuffix(name, ".") {
		return errors.ErrInvalidAppName
	}

	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.ErrInvalidAppName
		}
	}

	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return errors.ErrInvalidAppName
	}
	if len(base) == 4 &&
		(strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) {
		if base[3] >= '1' && base[3] <= '9' {
			return errors.ErrInvalidAppName
		}
	}

	return nil
}
