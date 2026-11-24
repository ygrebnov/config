package errors

import (
	errorsPkg "errors"

	"github.com/ygrebnov/errorc"
)

func newError(message string) error { return errorc.New("config: " + message) }

var (
	// Deprecated: missing configuration files are allowed when their parent exists.
	ErrConfigurationFileNotFound = errorc.New("configuration file not found")
	// ErrConfigurationDirectoryNotFound is returned for a custom path whose parent directory is absent.
	ErrConfigurationDirectoryNotFound            = newError("configuration directory not found")
	ErrCannotResolveUserConfigDir                = newError("cannot resolve user config dir")
	ErrInvalidAppName                            = newError("invalid application name")
	ErrInvalidConfigFilePath                     = newError("invalid path")
	ErrInsecurePermissions                       = newError("insecure permissions")
	ErrCannotLoadConfigurationIntoProvidedObject = errorc.New("cannot load configuration into provided object")
	ErrCannotInitializeConfigurationObject       = errorc.New("cannot initialize configuration object")

	// ErrNilTarget is returned when Load or Controller.Load receives a nil target pointer.
	ErrNilTarget = newError("nil target")
	// ErrPathNotConfigured is returned when an operation requires a config path but none was configured.
	ErrPathNotConfigured = newError("config path is not configured")
	// ErrConfigurationOptionNotFound is returned when a model path does not map to a stored config field.
	ErrConfigurationOptionNotFound = newError("config option not found")
	// ErrInaccessiblePath is returned when a filesystem path cannot be accessed.
	ErrInaccessiblePath = newError("inaccessible path")
	// ErrCannotCreateDirectories is returned when an app-discovered config directory cannot be created.
	ErrCannotCreateDirectories = newError("cannot create directories")
	// ErrUnsupportedConfigFileType is returned when the config file extension is not YAML or JSON.
	ErrUnsupportedConfigFileType = newError("unsupported config file type")
	// ErrCreateTempFile is returned when a temporary file for atomic config writes cannot be created.
	ErrCreateTempFile = newError("create temp config file")
	// ErrCloseTempFile is returned when the temporary file used for atomic config writes cannot be closed.
	ErrCloseTempFile = newError("close temp config file")
	// ErrParse is returned when YAML or JSON config content cannot be decoded.
	ErrParse = newError("parse config file")
	// ErrWrite is returned when encoded config data cannot be written or renamed into place.
	ErrWrite = newError("write config")
)

var (
	ErrNilContext = errorc.New("nil context")
)

// Is wraps native errors package Is.
func Is(err, target error) bool {
	return errorsPkg.Is(err, target)
}
