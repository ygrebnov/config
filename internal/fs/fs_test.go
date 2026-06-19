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

type stubStore struct {
	fromBytesErr   error
	fromBytesCalls int
	loaded         [][]byte

	yaml      []byte
	yamlErr   error
	yamlCalls int

	json      []byte
	jsonErr   error
	jsonCalls int
}

func (s *stubStore) FromBytes(b []byte) error {
	s.fromBytesCalls++
	s.loaded = append(s.loaded, append([]byte(nil), b...))
	return s.fromBytesErr
}

func (s *stubStore) GetYAML() ([]byte, error) {
	s.yamlCalls++
	return append([]byte(nil), s.yaml...), s.yamlErr
}

func (s *stubStore) GetJSON() ([]byte, error) {
	s.jsonCalls++
	return append([]byte(nil), s.json...), s.jsonErr
}

type stubTempFile struct {
	name     string
	writeErr error
	closeErr error
	writes   [][]byte
}

func (f *stubTempFile) Name() string { return f.name }

func (f *stubTempFile) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), b...))
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
	errIs       error
	errContains []string
	storeCalls  int
	loaded      []string
	out         string
}

func TestFS_From(t *testing.T) {
	storeParseErr := errors.New("store parse error")

	tests := []struct {
		name  string
		ctx   func() context.Context
		setup func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect
	}{
		{
			name: "returns context error before any work",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()
				return fromExpect{
					errIs:      context.Canceled,
					storeCalls: 0,
				}
			},
		},
		{
			name: "returns zero values when no source is configured",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()
				return fromExpect{
					storeCalls: 0,
				}
			},
		},
		{
			name: "uses explicit path with highest precedence",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
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
					path:       explicitPath,
					storeCalls: 1,
					loaded:     []string{"name: explicit\n"},
					out:        "Loaded configuration from " + explicitPath + "\n",
				}
			},
		},
		{
			name: "uses env path before app name",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
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
					path:       envPath,
					storeCalls: 1,
					loaded:     []string{"{}\n"},
					out:        "Loaded configuration from " + envPath + "\n",
				}
			},
		},
		{
			name: "uses app name when explicit and env paths are absent",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()

				t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
				cfg.AppName = "myapp"

				appPath := filepath.Join(dir, "xdg", "myapp", "config.yml")
				writeTestFile(t, appPath, "name: app\n")

				return fromExpect{
					path:       appPath,
					storeCalls: 1,
					loaded:     []string{"name: app\n"},
					out:        "Loaded configuration from " + appPath + "\n",
				}
			},
		},
		{
			name: "returns config-dir resolution error for app name",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()

				t.Setenv("HOME", "")
				t.Setenv("XDG_CONFIG_HOME", "")
				cfg.AppName = "myapp"

				return fromExpect{
					errIs:       configerrors.ErrCannotResolveUserConfigDir,
					errContains: []string{"$HOME is not defined"},
					storeCalls:  0,
				}
			},
		},
		{
			name: "ignores missing file",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()

				path := filepath.Join(dir, "missing.yaml")
				cfg.Path = path

				return fromExpect{
					path:       path,
					storeCalls: 0,
				}
			},
		},
		{
			name: "returns non-not-exist file errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()

				cfg.Path = dir

				return fromExpect{
					path:       dir,
					errIs:      configerrors.ErrInvalidConfigFilePath,
					storeCalls: 0,
				}
			},
		},
		{
			name: "wraps store parse errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, cfg *Config, store *stubStore) fromExpect {
				t.Helper()

				path := filepath.Join(dir, "config.yaml")
				writeTestFile(t, path, "name: broken\n")

				cfg.Path = path
				store.fromBytesErr = storeParseErr

				return fromExpect{
					path:        path,
					errIs:       configerrors.ErrParse,
					errContains: []string{path, storeParseErr.Error()},
					storeCalls:  1,
					loaded:      []string{"name: broken\n"},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := &Config{}
			store := &stubStore{}
			streams := kitstreams.NewBuffers()

			want := tt.setup(t, dir, cfg, store)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			fs := New(cfg, store, streams)
			gotPath, err := fs.From(ctx)

			if gotPath != want.path {
				t.Fatalf("path = %q, want %q", gotPath, want.path)
			}

			if want.errIs == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if !errors.Is(err, want.errIs) {
					t.Fatalf("expected error matching %v, got %v", want.errIs, err)
				}
				for _, fragment := range want.errContains {
					if !strings.Contains(err.Error(), fragment) {
						t.Fatalf("expected error %q to contain %q", err.Error(), fragment)
					}
				}
			}

			if store.fromBytesCalls != want.storeCalls {
				t.Fatalf("FromBytes calls = %d, want %d", store.fromBytesCalls, want.storeCalls)
			}

			if len(store.loaded) != len(want.loaded) {
				t.Fatalf("loaded payloads = %d, want %d", len(store.loaded), len(want.loaded))
			}
			for i := range want.loaded {
				if string(store.loaded[i]) != want.loaded[i] {
					t.Fatalf("loaded[%d] = %q, want %q", i, string(store.loaded[i]), want.loaded[i])
				}
			}

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

type validatePathExpect struct {
	errIs       error
	errContains []string
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string) (string, validatePathExpect)
	}{
		{
			name: "returns not exist for missing path",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				path := filepath.Join(dir, "missing.yaml")
				return path, validatePathExpect{errIs: os.ErrNotExist}
			},
		},
		{
			name: "returns inaccessible path when stat fails",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				blockedDir := filepath.Join(dir, "blocked")
				if err := os.Mkdir(blockedDir, 0o700); err != nil {
					t.Fatalf("mkdir blocked dir: %v", err)
				}
				if err := os.Chmod(blockedDir, 0); err != nil {
					t.Fatalf("chmod blocked dir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chmod(blockedDir, 0o700)
				})

				path := filepath.Join(blockedDir, "config.yaml")
				return path, validatePathExpect{
					errIs:       configerrors.ErrInaccessiblePath,
					errContains: []string{path},
				}
			},
		},
		{
			name: "returns invalid config file path for directory",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()
				return dir, validatePathExpect{errIs: configerrors.ErrInvalidConfigFilePath}
			},
		},
		{
			name: "returns unsupported config file type",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				path := filepath.Join(dir, "config.txt")
				writeTestFile(t, path, "name: text\n")
				return path, validatePathExpect{
					errIs:       configerrors.ErrUnsupportedConfigFileType,
					errContains: []string{path, ".txt"},
				}
			},
		},
		{
			name: "accepts yaml file",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				path := filepath.Join(dir, "config.yaml")
				writeTestFile(t, path, "name: yaml\n")
				return path, validatePathExpect{}
			},
		},
		{
			name: "accepts yml file",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				path := filepath.Join(dir, "config.yml")
				writeTestFile(t, path, "name: yml\n")
				return path, validatePathExpect{}
			},
		},
		{
			name: "accepts json file",
			setup: func(t *testing.T, dir string) (string, validatePathExpect) {
				t.Helper()

				path := filepath.Join(dir, "config.json")
				writeTestFile(t, path, "{}\n")
				return path, validatePathExpect{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path, want := tt.setup(t, dir)

			err := validatePath(path)

			if want.errIs == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if !errors.Is(err, want.errIs) {
				t.Fatalf("expected error matching %v, got %v", want.errIs, err)
			}
			for _, fragment := range want.errContains {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("expected error %q to contain %q", err.Error(), fragment)
				}
			}
		})
	}
}

type toExpect struct {
	errIs           error
	errContains     []string
	yamlCalls       int
	jsonCalls       int
	fileContent     string
	fileShouldExist bool
	skipPathCheck   bool
}

func TestFS_To(t *testing.T) {
	jsonStoreErr := errors.New("json store error")
	yamlStoreErr := errors.New("yaml store error")
	mkdirErr := errors.New("mkdir failure")
	createTempErr := errors.New("create temp failure")
	writeErr := errors.New("write failure")
	closeErr := errors.New("close failure")
	renameErr := errors.New("rename failure")

	tests := []struct {
		name  string
		ctx   func() context.Context
		setup func(t *testing.T, dir string, store *stubStore) (string, toExpect)
	}{
		{
			name: "returns context error before any work",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()
				return filepath.Join(dir, "config.yaml"), toExpect{errIs: context.Canceled}
			},
		},
		{
			name: "returns validate path error for directory target",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
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
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
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
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				mkdirAll = func(string, os.FileMode) error { return mkdirErr }
				return filepath.Join(dir, "nested", "config.yaml"), toExpect{
					errIs:       configerrors.ErrCannotCreateDirectories,
					errContains: []string{mkdirErr.Error()},
				}
			},
		},
		{
			name: "returns json store errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.jsonErr = jsonStoreErr
				return filepath.Join(dir, "config.json"), toExpect{
					errIs:     jsonStoreErr,
					jsonCalls: 1,
				}
			},
		},
		{
			name: "returns yaml store errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yamlErr = yamlStoreErr
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:     yamlStoreErr,
					yamlCalls: 1,
				}
			},
		},
		{
			name: "wraps create temp errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yaml = []byte("name: create\n")
				createTempFile = func(string, string) (tempFile, error) {
					return nil, createTempErr
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrCreateTempFile,
					errContains: []string{createTempErr.Error()},
					yamlCalls:   1,
				}
			},
		},
		{
			name: "wraps temp file write errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yaml = []byte("name: write\n")
				createTempFile = func(string, string) (tempFile, error) {
					return &stubTempFile{
						name:     filepath.Join(dir, "temp-write.yaml"),
						writeErr: writeErr,
					}, nil
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrWrite,
					errContains: []string{writeErr.Error()},
					yamlCalls:   1,
				}
			},
		},
		{
			name: "wraps temp file close errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yaml = []byte("name: close\n")
				createTempFile = func(string, string) (tempFile, error) {
					return &stubTempFile{
						name:     filepath.Join(dir, "temp-close.yaml"),
						closeErr: closeErr,
					}, nil
				}
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrCloseTempFile,
					errContains: []string{closeErr.Error()},
					yamlCalls:   1,
				}
			},
		},
		{
			name: "wraps rename errors",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yaml = []byte("name: rename\n")
				renameFile = func(string, string) error { return renameErr }
				return filepath.Join(dir, "config.yaml"), toExpect{
					errIs:       configerrors.ErrWrite,
					errContains: []string{renameErr.Error()},
					yamlCalls:   1,
				}
			},
		},
		{
			name: "writes json files using GetJSON",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.json = []byte("{\"name\":\"json\"}\n")
				path := filepath.Join(dir, "nested", "config.json")
				return path, toExpect{
					jsonCalls:       1,
					fileContent:     "{\"name\":\"json\"}\n",
					fileShouldExist: true,
				}
			},
		},
		{
			name: "writes yaml files using GetYAML",
			ctx:  context.Background,
			setup: func(t *testing.T, dir string, store *stubStore) (string, toExpect) {
				t.Helper()

				store.yaml = []byte("name: yaml\n")
				path := filepath.Join(dir, "nested", "config.yaml")
				return path, toExpect{
					yamlCalls:       1,
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

			dir := t.TempDir()
			store := &stubStore{}
			path, want := tt.setup(t, dir, store)

			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			fs := New(&Config{}, store, kitstreams.NewBuffers())
			err := fs.To(ctx, path)

			if want.errIs == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if !errors.Is(err, want.errIs) {
					t.Fatalf("expected error matching %v, got %v", want.errIs, err)
				}
				for _, fragment := range want.errContains {
					if !strings.Contains(err.Error(), fragment) {
						t.Fatalf("expected error %q to contain %q", err.Error(), fragment)
					}
				}
			}

			if store.yamlCalls != want.yamlCalls {
				t.Fatalf("GetYAML calls = %d, want %d", store.yamlCalls, want.yamlCalls)
			}
			if store.jsonCalls != want.jsonCalls {
				t.Fatalf("GetJSON calls = %d, want %d", store.jsonCalls, want.jsonCalls)
			}

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

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
