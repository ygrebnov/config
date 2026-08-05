package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ygrebnov/errorc"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
	"github.com/ygrebnov/config/pkg/log"
	configpath "github.com/ygrebnov/config/pkg/path"
)

// FS performs filesystem operations.
type FS struct {
	once   sync.Once
	cfg    *Config
	logger log.Logger

	path string
	data []byte
	err  error
}

func New(cfg *Config, logger log.Logger) *FS {
	return &FS{cfg: cfg, logger: logger}
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

func (fs *FS) From(ctx context.Context) (p string, b []byte, e error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}

	fs.once.Do(func() {
		var (
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

		fs.data, err = fs.fromFile(path)
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

		fs.logger.Log(
			log.LevelDebug,
			"loaded configuration",
			log.String(configkeys.Path, path),
		)
	})

	return fs.path, cloneBytes(fs.data), fs.err
}

func cloneBytes(data []byte) []byte {
	if data == nil {
		return nil
	}

	return append([]byte{}, data...)
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

func (fs *FS) To(ctx context.Context, path string, data []byte) error {
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
