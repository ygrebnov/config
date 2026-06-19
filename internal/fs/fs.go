package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	kitstreams "github.com/pumpingbytes/go-kit/streams"
	"github.com/ygrebnov/errorc"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
	configpath "github.com/ygrebnov/config/pkg/path"
)

// FS performs filesystem operations.
type FS struct {
	once    sync.Once
	cfg     *Config
	store   store
	streams kitstreams.IOStreams

	path string
	err  error
}

type store interface {
	FromBytes(b []byte) error

	GetYAML() ([]byte, error)
	GetJSON() ([]byte, error)
}

func New(cfg *Config, s store, streams kitstreams.IOStreams) *FS {
	return &FS{cfg: cfg, store: s, streams: streams}
}

type Config struct {
	Path      string
	EnvPrefix string
	AppName   string
}

type tempFile interface {
	Name() string
	Write([]byte) (int, error)
	Close() error
}

var (
	mkdirAll       = os.MkdirAll
	createTempFile = func(dir, pattern string) (tempFile, error) {
		return os.CreateTemp(dir, pattern)
	}
	renameFile = os.Rename
)

func (fs *FS) From(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	fs.once.Do(func() {
		var (
			b                   []byte
			path                string
			err                 error
			returnOnErrNotExist bool
		)

		switch {
		case fs.cfg.Path != "":
			path = fs.cfg.Path
			returnOnErrNotExist = true
		case fs.cfg.EnvPrefix != "" && os.Getenv(fs.cfg.EnvPrefix+"_CONFIG_PATH") != "":
			path = os.Getenv(fs.cfg.EnvPrefix + "_CONFIG_PATH")
		case fs.cfg.AppName != "":
			path, err = configpath.GetConfigPath(fs.cfg.AppName)
			if err != nil {
				fs.err = err
				return
			}
		default:
			return
		}
		fs.path = path

		b, err = fs.fromFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if returnOnErrNotExist {
					fs.err = errorc.With(
						configerrors.ErrConfigurationFileNotFound,
						errorc.String(configkeys.Path, path),
					)
				}
				return
			}
			fs.err = err
			return
		}

		if err = fs.store.FromBytes(b); err != nil {
			fs.err = errorc.With(
				configerrors.ErrParse,
				errorc.String(configkeys.Path, path),
				errorc.Error(configkeys.Cause, err),
			)
			return
		}

		_, _ = fmt.Fprintf(fs.streams.Out(), "Loaded configuration from %s\n", path)
	})

	return fs.path, fs.err
}

func (fs *FS) fromFile(path string) ([]byte, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	return os.ReadFile(path)
}

func validatePath(path string) error {
	info, err := os.Stat(path)
	switch {
	case err != nil && os.IsNotExist(err):
		return err
	case err != nil:
		return errorc.With(
			configerrors.ErrInaccessiblePath,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}

	if info.IsDir() {
		return errorc.With(
			configerrors.ErrInvalidConfigFilePath,
			errorc.String(configkeys.Path, path),
		)
	}

	ext := filepath.Ext(path)
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return errorc.With(
			configerrors.ErrUnsupportedConfigFileType,
			errorc.String(configkeys.Path, path),
			errorc.String(configkeys.FileFormat, ext),
		)
	}

	return nil
}

func (fs *FS) To(ctx context.Context, path string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := validatePath(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dir := filepath.Dir(path)
	if err := mkdirAll(dir, 0o700); err != nil {
		return errorc.With(
			configerrors.ErrCannotCreateDirectories,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}

	ext := filepath.Ext(path)
	var (
		data []byte
		e    error
	)
	switch ext {
	case ".json":
		data, e = fs.store.GetJSON()
	default:
		data, e = fs.store.GetYAML()
	}
	if e != nil {
		return e
	}

	tmpFile, err := createTempFile(dir, "temp-config-*"+ext)
	if err != nil {
		return errorc.With(
			configerrors.ErrCreateTempFile,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
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
		return errorc.With(
			configerrors.ErrCloseTempFile,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	if err := renameFile(tmpFile.Name(), path); err != nil {
		return errorc.With(
			configerrors.ErrWrite,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}

	return nil
}
