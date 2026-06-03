// Package config: encoding (YAML/JSON) helpers.
//
// This file defines loadFromFile and writeToFile which handle reading and
// writing configuration structures in YAML or JSON formats, using error
// sentinels declared in config.go for classification.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
	"github.com/ygrebnov/errorc"
	"gopkg.in/yaml.v3"
)

// Encoding / decoding logic extracted from util.go for clarity.
// Uses exported error sentinels declared in config.go: ErrUnsupportedConfigFileType,
// ErrParse, ErrFormat, ErrWrite.

type classifiedCauseError struct {
	kind  error
	cause error
}

func (e *classifiedCauseError) Error() string {
	if e == nil || e.kind == nil {
		return ""
	}
	return e.kind.Error()
}

func (e *classifiedCauseError) Unwrap() []error {
	if e == nil {
		return nil
	}

	errs := make([]error, 0, 2)
	if e.kind != nil {
		errs = append(errs, e.kind)
	}
	if e.cause != nil {
		errs = append(errs, e.cause)
	}
	return errs
}

func wrapFileIOError(kind error, path string, cause error) error {
	return errorc.With(
		&classifiedCauseError{kind: kind, cause: cause},
		errorc.String(configkeys.Path, path),
		errorc.Error(configkeys.Cause, cause),
	)
}

func loadFromFile(path string, cfg interface{}) error {
	if path == "" {
		return nil
	}

	ext := filepath.Ext(path)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return errorc.With(
			configerrors.ErrUnsupportedConfigFileType,
			errorc.String(configkeys.Path, path),
			errorc.String(configkeys.FileFormat, ext),
		)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return wrapFileIOError(configerrors.ErrReadFile, path, err)
	}

	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)
	default:
		err = yaml.Unmarshal(data, cfg)
	}
	if err != nil {
		return errorc.With(
			configerrors.ErrParse,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	return nil
}

func writeToFile(path string, cfg interface{}) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = errorc.With(
				configerrors.ErrFormat,
				errorc.String(configkeys.Path, path),
				errorc.String(configkeys.Cause, fmt.Sprint(r)),
			)
		}
	}()

	ext := filepath.Ext(path)
	if ext != "" && ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return errorc.With(
			configerrors.ErrUnsupportedConfigFileType,
			errorc.String(configkeys.Path, path),
			errorc.String(configkeys.FileFormat, ext),
		)
	}

	var (
		data []byte
		err  error
	)
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(cfg, "", "  ")
	default:
		data, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return errorc.With(
			configerrors.ErrFormat,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}

	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "temp-config-*"+ext)
	if err != nil {
		return wrapFileIOError(configerrors.ErrCreateTempFile, path, err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return errorc.With(
			configerrors.ErrWrite,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	if err := tmpFile.Close(); err != nil {
		return wrapFileIOError(configerrors.ErrCloseTempFile, path, err)
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return errorc.With(
			configerrors.ErrWrite,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	return nil
}
