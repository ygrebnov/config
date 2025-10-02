package config

import "errors"

// Exported error categories returned by this package. These are used with wrapping
// so callers can detect error classes using errors.Is/As.
//   - ErrEnsureConfigDir: failure to create parent directories for a config file.
//   - ErrUnsupportedConfigFileType: file extension is neither .yaml/.yml nor .json.
//   - ErrParse: failure to parse an existing config file.
//   - ErrFormat: failure to marshal a config to bytes (e.g., unsupported type).
//   - ErrWrite: failure to write the config file to disk.
var (
	ErrEnsureConfigDir           = errors.New("ensure config dir")
	ErrUnsupportedConfigFileType = errors.New("unsupported config file type")
	ErrParse                     = errors.New("parse config file")
	ErrFormat                    = errors.New("format config")
	ErrWrite                     = errors.New("write to config file")
)
