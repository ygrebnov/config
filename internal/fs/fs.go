package fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	path    string
	appPath bool
	data    []byte
	err     error
}

func New(cfg *Config, logger log.Logger) *FS {
	return &FS{cfg: cfg, logger: logger}
}

type Config struct {
	Path      string
	EnvPrefix string
	AppName   string
}

func (fs *FS) From(ctx context.Context) (p string, b []byte, e error) {
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}

	fs.once.Do(func() {
		var (
			path string
			err  error
		)

		switch {
		case fs.cfg.Path != "":
			path = fs.cfg.Path
		case fs.cfg.EnvPrefix != "" && os.Getenv(fs.cfg.EnvPrefix+"_CONFIG_PATH") != "":
			path = os.Getenv(fs.cfg.EnvPrefix + "_CONFIG_PATH")
		case fs.cfg.AppName != "":
			path, err = configpath.GetConfigPath(fs.cfg.AppName)
			if err != nil {
				fs.err = err
				return
			}
			fs.appPath = true
		default:
			return
		}
		fs.path = path

		fs.data, err = fs.fromFile(path, fs.appPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
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

func validateExtension(path string) error {
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

func (fs *FS) fromFile(path string, appPath bool) ([]byte, error) {
	root, name, err := openParent(path, appPath, false)
	if err != nil {
		return nil, pathError(path, err)
	}
	defer root.Close()

	info, err := root.Stat(name)
	if err != nil {
		return nil, pathError(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, errorc.With(configerrors.ErrInvalidConfigFilePath, errorc.String(configkeys.Path, path))
	}
	if errValidateExtension := validateExtension(path); errValidateExtension != nil {
		return nil, errValidateExtension
	}

	// A nonblocking open prevents a replacement FIFO from hanging the read.
	file, err := openReadFile(root, name)
	if err != nil {
		return nil, pathError(path, err)
	}
	defer file.Close()

	info, err = file.Stat()
	if err != nil {
		return nil, pathError(path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, errorc.With(configerrors.ErrInvalidConfigFilePath, errorc.String(configkeys.Path, path))
	}
	if errValidatePrivatePermissions := validatePrivatePermissions(file); errValidatePrivatePermissions != nil {
		return nil, pathError(path, errValidatePrivatePermissions)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, pathError(path, err)
	}
	return data, nil
}

// openParent establishes a stable confinement boundary for one config file.
// App-discovered paths are confined to the user's home. Explicit and
// environment paths use their parent directory as the boundary.
func openParent(path string, appPath, create bool) (*os.Root, string, error) {
	dir, name, err := parsePath(path)
	if err != nil {
		return nil, "", err
	}

	if !appPath {
		return openCustomPathParent(dir, name)
	}

	return openAppPathParent(dir, name, create)
}

func openAppPathParent(dir, name string, create bool) (*os.Root, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", errorc.With(
			configerrors.ErrCannotResolveUserConfigDir,
			errorc.Error(configkeys.Cause, err),
		)
	}
	rel, err := filepath.Rel(home, dir)
	if err != nil ||
		rel == ".." ||
		filepath.IsAbs(rel) ||
		strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, "", configerrors.ErrInvalidConfigFilePath
	}

	homeRoot, err := os.OpenRoot(home)
	if err != nil {
		return nil, "", err
	}
	defer homeRoot.Close()

	if create {
		if errMkdirAll := homeRoot.MkdirAll(rel, 0o700); errMkdirAll != nil {
			return nil, "", errorc.With(
				configerrors.ErrCannotCreateDirectories,
				errorc.Error(configkeys.Cause, errMkdirAll),
			)
		}
	}
	appRoot, err := homeRoot.OpenRoot(rel)
	if err != nil {
		return nil, "", err
	}
	if errValidatePrivateDirectoryPermissions :=
		validatePrivateDirectoryPermissions(appRoot); errValidatePrivateDirectoryPermissions != nil {
		_ = appRoot.Close()
		return nil, "", errValidatePrivateDirectoryPermissions
	}
	return appRoot, name, nil
}

func openCustomPathParent(dir, name string) (*os.Root, string, error) {
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", configerrors.ErrConfigurationDirectoryNotFound
	}
	return root, name, err
}

func parsePath(path string) (dir, name string, err error) {
	dir = filepath.Dir(path)
	name = filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		return "", "", configerrors.ErrInvalidConfigFilePath
	}

	return dir, name, nil
}

func pathError(path string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, known := range []error{
		configerrors.ErrConfigurationDirectoryNotFound,
		configerrors.ErrCannotCreateDirectories,
		configerrors.ErrCannotResolveUserConfigDir,
		configerrors.ErrInvalidConfigFilePath,
		configerrors.ErrInsecurePermissions,
	} {
		if errors.Is(err, known) {
			return errorc.With(err, errorc.String(configkeys.Path, path))
		}
	}
	return errorc.With(
		configerrors.ErrInaccessiblePath,
		errorc.String(configkeys.Path, path),
		errorc.Error(configkeys.Cause, err),
	)
}

func (fs *FS) To(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root, name, err := openParent(path, fs.appPath && path == fs.path, true)
	if err != nil {
		return pathError(path, err)
	}
	defer root.Close()

	if errValidateFile := validateFile(root, name, path); errValidateFile != nil {
		return errValidateFile
	}

	if errValidateExtension := validateExtension(path); errValidateExtension != nil {
		return errValidateExtension
	}

	tempName, file, err := createTemp(root, filepath.Ext(path))
	if err != nil {
		return errorc.With(
			configerrors.ErrCreateTempFile,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, err),
		)
	}
	defer func() { _ = root.Remove(tempName) }()

	if errValidatePrivatePermissions := validatePrivatePermissions(file); errValidatePrivatePermissions != nil {
		_ = file.Close()
		return pathError(path, errValidatePrivatePermissions)
	}

	if errWrite := writeToFile(file, data, path); errWrite != nil {
		return errWrite
	}

	if errClose := file.Close(); errClose != nil {
		return errorc.With(
			configerrors.ErrCloseTempFile,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, errClose),
		)
	}

	if errCtx := ctx.Err(); errCtx != nil {
		return errCtx
	}

	if errRename := root.Rename(tempName, name); errRename != nil {
		return errorc.With(
			configerrors.ErrWrite,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, errRename),
		)
	}

	return nil
}

func validateFile(root *os.Root, name, path string) error {
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return errorc.With(configerrors.ErrInvalidConfigFilePath, errorc.String(configkeys.Path, path))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return pathError(path, err)
	}

	return nil
}

func createTemp(root *os.Root, ext string) (string, *os.File, error) {
	for range 10 {
		random := make([]byte, 12)
		if _, err := rand.Read(random); err != nil {
			return "", nil, err
		}
		name := "temp-config-" + hex.EncodeToString(random) + ext
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return name, file, err
	}
	return "", nil, os.ErrExist
}

func writeToFile(file *os.File, data []byte, path string) error {
	n, writeErr := file.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}

	if writeErr != nil {
		_ = file.Close()
		return errorc.With(
			configerrors.ErrWrite,
			errorc.String(configkeys.Path, path),
			errorc.Error(configkeys.Cause, writeErr),
		)
	}

	return nil
}
