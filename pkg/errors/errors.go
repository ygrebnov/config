package errors

import (
	errorsPkg "errors"

	"github.com/ygrebnov/errorc"
)

var (
	// ErrConfigurationDirectoryNotFound is returned for a custom path whose parent directory is absent.
	ErrConfigurationDirectoryNotFound            = errorc.New("configuration directory not found")
	ErrCannotResolveUserConfigDir                = errorc.New("cannot resolve user config dir")
	ErrInvalidAppName                            = errorc.New("invalid application name")
	ErrInvalidConfigFilePath                     = errorc.New("invalid path")
	ErrInsecurePermissions                       = errorc.New("insecure permissions")
	ErrCannotLoadConfigurationIntoProvidedObject = errorc.New("cannot load configuration into provided object")
	ErrCannotInitializeConfigurationObject       = errorc.New("cannot initialize configuration object")

	// ErrNilTarget is returned when Load or Controller.Load receives a nil target pointer.
	ErrNilTarget = errorc.New("nil target")
	// ErrPathNotConfigured is returned when an operation requires a config path but none was configured.
	ErrPathNotConfigured = errorc.New("config path is not configured")
	// ErrConfigurationOptionNotFound is returned when a model path does not map to a stored config field.
	ErrConfigurationOptionNotFound = errorc.New("config option not found")
	// ErrInaccessiblePath is returned when a filesystem path cannot be accessed.
	ErrInaccessiblePath = errorc.New("inaccessible path")
	// ErrCannotCreateDirectories is returned when an app-discovered config directory cannot be created.
	ErrCannotCreateDirectories = errorc.New("cannot create directories")
	// ErrUnsupportedConfigFileType is returned when the config file extension is not YAML or JSON.
	ErrUnsupportedConfigFileType = errorc.New("unsupported config file type")
	// ErrCreateTempFile is returned when a temporary file for atomic config writes cannot be created.
	ErrCreateTempFile = errorc.New("create temp config file")
	// ErrCloseTempFile is returned when the temporary file used for atomic config writes cannot be closed.
	ErrCloseTempFile = errorc.New("close temp config file")
	// ErrParse is returned when YAML or JSON config content cannot be decoded.
	ErrParse = errorc.New("parse config file")
	// ErrWrite is returned when encoded config data cannot be written or renamed into place.
	ErrWrite = errorc.New("write config")
)

var (
	ErrNilContext = errorc.New("nil context")
)

// Is wraps native errors package Is.
func Is(err, target error) bool {
	return errorsPkg.Is(err, target)
}
