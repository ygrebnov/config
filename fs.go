// Package config: filesystem path and persistence helpers.
//
// This file contains path validation and directory creation logic (EnsurePath)
// and associated error sentinels used by the Provider when persisting config
// files to disk.
package config

import (
	"errors"
	"os"
	"path/filepath"
)

// Filesystem & path utilities (extracted from util.go).

var (
	ErrInaccessiblePath        = errors.New("inaccessible path")
	ErrCannotCreateDirectories = errors.New("cannot create directories")
)

// EnsurePath ensures the directories for a file path exist and the path
// does not already exist as a directory.
func EnsurePath(p string) error {
	info, err := os.Stat(p)
	switch {
	case err == nil:
		if info.IsDir() {
			return ErrInaccessiblePath
		}
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return ErrInaccessiblePath
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ErrCannotCreateDirectories
	}
	return nil
}
