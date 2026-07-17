package errors

import (
	errorsPkg "errors"

	"github.com/ygrebnov/errorc"
)

var namespace = errorc.Namespace("config")

var (
	ErrConfigurationFileNotFound                 = errorc.New("configuration file not found")
	ErrCannotResolveUserConfigDir                = namespace.NewError("cannot resolve user config dir")
	ErrInvalidConfigFilePath                     = namespace.NewError("invalid path")
	ErrCannotLoadConfigurationIntoProvidedObject = errorc.New("cannot load configuration into provided object")
	ErrCannotInitializeConfigurationObject       = errorc.New("cannot initialize configuration object")

	// ErrNilTarget is returned when Load or Controller.Load receives a nil target pointer.
	ErrNilTarget = namespace.NewError("nil target")
	// ErrPathNotConfigured is returned when an operation requires a config path but none was configured.
	ErrPathNotConfigured = namespace.NewError("config path is not configured")
	// ErrConfigurationOptionNotFound is returned when a model path does not map to a stored config field.
	ErrConfigurationOptionNotFound = namespace.NewError("config option not found")
	// ErrInaccessiblePath is returned when a filesystem path cannot be accessed.
	ErrInaccessiblePath = namespace.NewError("inaccessible path")
	// ErrCannotCreateDirectories is returned when parent directories for a config file cannot be created.
	ErrCannotCreateDirectories = namespace.NewError("cannot create directories")
	// ErrUnsupportedConfigFileType is returned when the config file extension is not YAML or JSON.
	ErrUnsupportedConfigFileType = namespace.NewError("unsupported config file type")
	// ErrCreateTempFile is returned when a temporary file for atomic config writes cannot be created.
	ErrCreateTempFile = namespace.NewError("create temp config file")
	// ErrCloseTempFile is returned when the temporary file used for atomic config writes cannot be closed.
	ErrCloseTempFile = namespace.NewError("close temp config file")
	// ErrParse is returned when YAML or JSON config content cannot be decoded.
	ErrParse = namespace.NewError("parse config file")
	// ErrWrite is returned when encoded config data cannot be written or renamed into place.
	ErrWrite = namespace.NewError("write config")
)

var (
	ErrNilContext = errorc.New("nil context")
)

// Is wraps native errors package Is.
func Is(err, target error) bool {
	return errorsPkg.Is(err, target)
}
