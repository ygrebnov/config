package errors

import "github.com/ygrebnov/errorc"

var namespace = errorc.Namespace("config")

var (
	// ErrNilTarget is returned when Load or Controller.Load receives a nil target pointer.
	ErrNilTarget = namespace.NewError("nil target")
	// ErrNotStructPointer is returned when the target is not a non-nil pointer to a struct.
	ErrNotStructPointer = namespace.NewError("target must be a non-nil pointer to struct")
	// ErrEmptyPath is returned when WithPath receives an empty path.
	ErrEmptyPath = namespace.NewError("path cannot be empty")
	// ErrEmptyAppName is returned when WithAppName receives an empty application name.
	ErrEmptyAppName = namespace.NewError("app name cannot be empty")
	// ErrEmptyEnvPrefix is returned when WithEnvPrefix receives an empty environment prefix.
	ErrEmptyEnvPrefix = namespace.NewError("env prefix cannot be empty")
	// ErrCannotResolveConfigPath is returned when config path resolution fails.
	ErrCannotResolveConfigPath = namespace.NewError("cannot resolve config path")
	// ErrPathNotConfigured is returned when an operation requires a config path but none was configured.
	ErrPathNotConfigured = namespace.NewError("config path is not configured")
	// ErrControllerNotLoaded is returned when controller operations require a prior successful Load call.
	ErrControllerNotLoaded = namespace.NewError("controller is not loaded")
	// ErrOptionNotFound is returned when a dotted option path does not map to a known config field.
	ErrOptionNotFound = namespace.NewError("config option not found")
	// ErrOptionNotSettable is returned when a config option path resolves to a value that cannot be updated.
	ErrOptionNotSettable = namespace.NewError("config option is not settable")
	// ErrInvalidOptionValue is returned when a provided option value cannot be assigned to the target field.
	ErrInvalidOptionValue = namespace.NewError("invalid config option value")
	// ErrInaccessiblePath is returned when a filesystem path cannot be accessed.
	ErrInaccessiblePath = namespace.NewError("inaccessible path")
	// ErrCannotCreateDirectories is returned when parent directories for a config file cannot be created.
	ErrCannotCreateDirectories = namespace.NewError("cannot create directories")
	// ErrUnsupportedConfigFileType is returned when the config file extension is not YAML or JSON.
	ErrUnsupportedConfigFileType = namespace.NewError("unsupported config file type")
	// ErrReadFile is returned when reading a config file from disk fails.
	ErrReadFile = namespace.NewError("read config file")
	// ErrCreateTempFile is returned when a temporary file for atomic config writes cannot be created.
	ErrCreateTempFile = namespace.NewError("create temp config file")
	// ErrCloseTempFile is returned when the temporary file used for atomic config writes cannot be closed.
	ErrCloseTempFile = namespace.NewError("close temp config file")
	// ErrParse is returned when YAML or JSON config content cannot be decoded.
	ErrParse = namespace.NewError("parse config file")
	// ErrFormat is returned when config data cannot be encoded for persistence.
	ErrFormat = namespace.NewError("format config")
	// ErrWrite is returned when encoded config data cannot be written or renamed into place.
	ErrWrite = namespace.NewError("write config")
)
