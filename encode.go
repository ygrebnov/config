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

	"gopkg.in/yaml.v3"
)

// Encoding / decoding logic extracted from util.go for clarity.
// Uses exported error sentinels declared in config.go: ErrUnsupportedConfigFileType,
// ErrParse, ErrFormat, ErrWrite.

func loadFromFile(path string, cfg interface{}) error {
	if path == "" {
		return nil
	}
	ext := filepath.Ext(path)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return fmt.Errorf("%w: %s", ErrUnsupportedConfigFileType, ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	switch ext {
	case ".json":
		err = json.Unmarshal(data, cfg)
	default:
		err = yaml.Unmarshal(data, cfg)
	}
	if err != nil {
		return fmt.Errorf("%w %s: %w", ErrParse, path, err)
	}
	return nil
}

func writeToFile(path string, cfg interface{}) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("%w as %s: %v", ErrFormat, filepath.Ext(path), r)
		}
	}()
	ext := filepath.Ext(path)
	if ext != "" && ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return fmt.Errorf("%w: %s", ErrUnsupportedConfigFileType, ext)
	}
	var data []byte
	var err error
	switch ext {
	case ".json":
		data, err = json.MarshalIndent(cfg, "", "  ")
	default:
		data, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return fmt.Errorf("%w as %s: %w", ErrFormat, ext, err)
	}
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "temp-config-*"+ext)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("%w %s: %w", ErrWrite, path, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		return fmt.Errorf("rename temp file to %s: %w", path, err)
	}
	return nil
}
