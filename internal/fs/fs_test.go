package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kitstreams "github.com/pumpingbytes/go-kit/streams"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

type stubTempFile struct {
	name     string
	writeErr error
	closeErr error
}

func (f *stubTempFile) Name() string { return f.name }

func (f *stubTempFile) Write(b []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}

	return len(b), nil
}

func (f *stubTempFile) Close() error {
	return f.closeErr
}

type fromExpect struct {
	path        string
	data        string
	errIs       error
	errContains []string
	out         string
}

func TestFS_From(t *testing.T) {
	tests := []struct {
		name  string
		ctx   func() context.Context
		setup func(t *testing.T, dir string, cfg *Config) fromExpect
	}{
		{
			name: "returns context error before any work",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()
				return fromExpect{errIs: context.Canceled}
			},
		},
		{
			name: "returns zero values when no source is configured",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()
				return fromExpect{}
			},
		},
		{
			name: "uses explicit path with highest precedence",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()

				explicitPath := filepath.Join(dir, "explicit.yaml")
				envPath := filepath.Join(dir, "env.yaml")
				appPath := filepath.Join(dir, "xdg", "myapp", "config.yml")
				writeTestFile(t, explicitPath, "name: explicit\n")
				writeTestFile(t, envPath, "name: env\n")
				writeTestFile(t, appPath, "name: app\n")

				t.Setenv("PFX_CONFIG_PATH", envPath)
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
				cfg.Path = explicitPath
				cfg.EnvPrefix = "PFX"
				cfg.AppName = "myapp"

				return fromExpect{
					path: explicitPath,
					data: "name: explicit\n",
					out:  "Loaded configuration from " + explicitPath + "\n",
				}
			},
		},
		{
			name: "uses env path before app name",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()

				envPath := filepath.Join(dir, "env.json")
				appPath := filepath.Join(dir, "xdg", "myapp", "config.yml")
				writeTestFile(t, envPath, "{}\n")
				writeTestFile(t, appPath, "name: app\n")

				t.Setenv("PFX_CONFIG_PATH", envPath)
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
				cfg.EnvPrefix = "PFX"
				cfg.AppName = "myapp"

				return fromExpect{
					path: envPath,
					data: "{}\n",
					out:  "Loaded configuration from " + envPath + "\n",
				}
			},
		},
		{
			name: "uses app name when explicit and env paths are absent",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()

				t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
				cfg.AppName = "myapp"
				appPath := filepath.Join(dir, "xdg", "myapp", "config.yml")
				writeTestFile(t, appPath, "name: app\n")

				return fromExpect{
					path: appPath,
					data: "name: app\n",
					out:  "Loaded configuration from " + appPath + "\n",
				}
			},
		},
		{
			name: "returns config-dir resolution error for app name",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()

				t.Setenv("HOME", "")
				t.Setenv("XDG_CONFIG_HOME", "")
				cfg.AppName = "myapp"

				return fromExpect{
					errIs: configerrors.ErrCannotResolveUserConfigDir,
				}
			},
		},
		{
			name: "returns not-found error for explicit missing file",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()

				path := filepath.Join(dir, "missing.yaml")
				cfg.Path = path

				return fromExpect{
					path:  path,
					errIs: configerrors.ErrConfigurationFileNotFound,
				}
			},
		},
		{
			name: "returns invalid path error for a directory",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config) fromExpect {
				t.Helper()
				cfg.Path = dir

				return fromExpect{
					path:  dir,
					errIs: configerrors.ErrInvalidConfigFilePath,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &Config{}
			streams := kitstreams.NewBuffers()
			want := tt.setup(t, dir, cfg)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			gotPath, gotData, err := New(cfg, streams).From(ctx)
			if gotPath != want.path {
				t.Fatalf("path = %q, want %q", gotPath, want.path)
			}
			if string(gotData) != want.data {
				t.Fatalf("data = %q, want %q", gotData, want.data)
			}
			assertError(t, err, want.errIs, want.errContains)

			out, errOut := streams.Strings()
			if out != want.out {
				t.Fatalf("stdout = %q, want %q", out, want.out)
			}
			if errOut != "" {
				t.Fatalf("stderr = %q, want empty", errOut)
			}
		})
	}
}

func TestFS_FromCachesAndCopiesBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeTestFile(t, path, "name: original\n")

	fs := New(&Config{Path: path}, kitstreams.NewBuffers())
	_, first, err := fs.From(context.Background())
	if err != nil {
		t.Fatalf("first From() error: %v", err)
	}
	first[0] = 'X'

	_, second, err := fs.From(context.Background())
	if err != nil {
		t.Fatalf("second From() error: %v", err)
	}
	if string(second) != "name: original\n" {
		t.Fatalf("cached data = %q, want %q", second, "name: original\n")
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string) string
		errIs error
	}{
		{
			name: "returns not exist for missing path",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return filepath.Join(dir, "missing.yaml")
			},
			errIs: os.ErrNotExist,
		},
		{
			name: "returns invalid config file path for directory",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return dir
			},
			errIs: configerrors.ErrInvalidConfigFilePath,
		},
		{
			name: "returns unsupported config file type",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "config.txt")
				writeTestFile(t, path, "name: text\n")
				return path
			},
			errIs: configerrors.ErrUnsupportedConfigFileType,
		},
		{
			name: "accepts supported file types",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				path := filepath.Join(dir, "config.json")
				writeTestFile(t, path, "{}\n")
				return path
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.setup(t, t.TempDir()))
			assertError(t, err, tt.errIs, nil)
		})
	}
}

type toExpect struct {
	errIs           error
	errContains     []string
	fileContent     string
	fileShouldExist bool
	skipPathCheck   bool
}

func TestFS_To(t *testing.T) {
	mkdirErr := errors.New("mkdir failure")
	createTempErr := errors.New("create temp failure")
	writeErr := errors.New("write failure")
	closeErr := errors.New("close failure")
	renameErr := errors.New("rename failure")

	tests := []struct {
		name    string
		ctx     func() context.Context
		setup   func(t *testing.T, dir string) (string, toExpect)
		payload string
	}{
		{
			name: "returns context error before any work",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				return filepath.Join(dir, "config.yaml"), toExpect{errIs: context.Canceled}
			},
		},
		{
			name: "returns validate path error for directory target",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				return dir, toExpect{
					errIs:         configerrors.ErrInvalidConfigFilePath,
					skipPathCheck: true,
				}
			},
		},
		{
			name: "returns validate path error for unsupported extension",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				path := filepath.Join(dir, "config.txt")
				writeTestFile(t, path, "text\n")
				return path, toExpect{
					errIs:           configerrors.ErrUnsupportedConfigFileType,
					fileContent:     "text\n",
					fileShouldExist: true,
				}
			},
		},
		{
			name: "wraps mkdir errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				mkdirAll = func(string, os.FileMode) error { return mkdirErr }
				return filepath.Join(dir, "nested", "config.yaml"), toExpect{
					errIs:       configerrors.ErrCannotCreateDirectories,
					errContains: []string{mkdirErr.Error()},
				}
			},
		},
		{
			name: "wraps create temp errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				createTempFile = func(string, string) (tempFile, error) {
					return nil, createTempErr
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrCreateTempFile,
					errContains: []string{createTempErr.Error()},
				}
			},
		},
		{
			name: "wraps temp file write errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				createTempFile = func(string, string) (tempFile, error) {
					return &stubTempFile{
						name:     filepath.Join(dir, "temp-write.yaml"),
						writeErr: writeErr,
					}, nil
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrWrite,
					errContains: []string{writeErr.Error()},
				}
			},
		},
		{
			name: "wraps temp file close errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				createTempFile = func(string, string) (tempFile, error) {
					return &stubTempFile{
						name:     filepath.Join(dir, "temp-close.yaml"),
						closeErr: closeErr,
					}, nil
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrCloseTempFile,
					errContains: []string{closeErr.Error()},
				}
			},
		},
		{
			name: "wraps rename errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				renameFile = func(string, string) error { return renameErr }
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrWrite,
					errContains: []string{renameErr.Error()},
				}
			},
		},
		{
			name:    "writes raw JSON bytes",
			ctx:     context.Background,
			payload: "{\"name\":\"json\"}\n",
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				return filepath.Join(dir, "nested", "config.json"), toExpect{
					fileContent:     "{\"name\":\"json\"}\n",
					fileShouldExist: true,
				}
			},
		},
		{
			name:    "writes raw YAML bytes",
			ctx:     context.Background,
			payload: "name: yaml\n",
			setup: func(t *testing.T, dir string) (string, toExpect) {
				t.Helper()
				return filepath.Join(dir, "nested", "config.yaml"), toExpect{
					fileContent:     "name: yaml\n",
					fileShouldExist: true,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origMkdirAll := mkdirAll
			origCreateTempFile := createTempFile
			origRenameFile := renameFile
			t.Cleanup(func() {
				mkdirAll = origMkdirAll
				createTempFile = origCreateTempFile
				renameFile = origRenameFile
			})

			path, want := tt.setup(t, t.TempDir())
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			err := New(&Config{}, kitstreams.NewBuffers()).To(
				ctx,
				path,
				[]byte(tt.payload),
			)
			assertError(t, err, want.errIs, want.errContains)

			if want.skipPathCheck {
				return
			}

			got, readErr := os.ReadFile(path)
			switch {
			case want.fileShouldExist && readErr != nil:
				t.Fatalf("reading written file: %v", readErr)
			case want.fileShouldExist && string(got) != want.fileContent:
				t.Fatalf("file content = %q, want %q", string(got), want.fileContent)
			case !want.fileShouldExist && readErr == nil:
				t.Fatalf("expected file %q to be absent", path)
			case !want.fileShouldExist && !errors.Is(readErr, os.ErrNotExist):
				t.Fatalf("expected not-exist for %q, got %v", path, readErr)
			}
		})
	}
}

func assertError(
	t *testing.T,
	err error,
	want error,
	contains []string,
) {
	t.Helper()

	if want == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}

	if !errors.Is(err, want) {
		t.Fatalf("expected error matching %v, got %v", want, err)
	}
	for _, fragment := range contains {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error %q to contain %q", err.Error(), fragment)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
