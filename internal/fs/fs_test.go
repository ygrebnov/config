package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/log"
)

type testLogger struct {
	out string
}

func (l *testLogger) Log(_ log.Level, message string, fields ...log.Field) {
	l.out += message + ","
	for _, f := range fields {
		l.out += fmt.Sprintf(" %s=%v", f.Key(), f.String())
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestFromSourcePrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))

	explicit := filepath.Join(home, "explicit.yaml")
	env := filepath.Join(home, "env.json")
	app := filepath.Join(home, "xdg", "myapp", "config.yml")
	writeTestFile(t, explicit, "name: explicit\n")
	writeTestFile(t, env, "{\"name\":\"env\"}\n")
	writeTestFile(t, app, "name: app\n")
	t.Setenv("PFX_CONFIG_PATH", env)

	for _, tt := range []struct {
		name string
		cfg  Config
		path string
		data string
	}{
		{"explicit", Config{Path: explicit, EnvPrefix: "PFX", AppName: "myapp"}, explicit, "name: explicit\n"},
		{"environment", Config{EnvPrefix: "PFX", AppName: "myapp"}, env, "{\"name\":\"env\"}\n"},
		{"application", Config{AppName: "myapp"}, app, "name: app\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := &testLogger{}
			fs := New(&tt.cfg, logger)
			path, data, err := fs.From(context.Background())
			if err != nil || path != tt.path || string(data) != tt.data {
				t.Fatalf("From() = (%q, %q, %v), want (%q, %q)", path, data, err, tt.path, tt.data)
			}
			if !strings.Contains(logger.out, "loaded configuration") {
				t.Fatalf("missing load log: %q", logger.out)
			}
			data[0] = 'X'
			_, again, err := fs.From(context.Background())
			if err != nil || string(again) != tt.data {
				t.Fatalf("cached data changed: %q, %v", again, err)
			}
		})
	}
}

func TestFromMissingAndInvalidPaths(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "missing.yaml")
	path, data, err := New(&Config{Path: explicit}, &testLogger{}).From(context.Background())
	if err != nil || path != explicit || data != nil {
		t.Fatalf("explicit missing file = (%q, %q, %v)", path, data, err)
	}

	t.Setenv("PFX_CONFIG_PATH", explicit)
	path, data, err = New(&Config{EnvPrefix: "PFX"}, &testLogger{}).From(context.Background())
	if err != nil || path != explicit || data != nil {
		t.Fatalf("environment missing file = (%q, %q, %v)", path, data, err)
	}

	missingParent := filepath.Join(dir, "missing-parent", "config.yaml")
	_, _, err = New(&Config{Path: missingParent}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrConfigurationDirectoryNotFound) {
		t.Fatalf("explicit missing directory: %v", err)
	}
	t.Setenv("PFX_CONFIG_PATH", missingParent)
	_, _, err = New(&Config{EnvPrefix: "PFX"}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrConfigurationDirectoryNotFound) {
		t.Fatalf("environment missing directory: %v", err)
	}

	_, _, err = New(&Config{Path: dir}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrInvalidConfigFilePath) {
		t.Fatalf("directory source: %v", err)
	}

	textPath := filepath.Join(dir, "config.txt")
	writeTestFile(t, textPath, "invalid\n")
	_, _, err = New(&Config{Path: textPath}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrUnsupportedConfigFileType) {
		t.Fatalf("unsupported extension: %v", err)
	}
}

func TestToAtomicSaveAndValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.yaml")
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	fs := New(&Config{}, &testLogger{})
	for _, value := range []string{"name: first\n", "name: second\n"} {
		if err := fs.To(context.Background(), path, []byte(value)); err != nil {
			t.Fatalf("To() error: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != value {
			t.Fatalf("saved content = %q, %v", data, err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("config file is not private: %v", info.Mode())
	}

	for _, tt := range []struct {
		path string
		want error
	}{
		{filepath.Join(dir, "new.txt"), configerrors.ErrUnsupportedConfigFileType},
		{dir, configerrors.ErrInvalidConfigFilePath},
	} {
		err := fs.To(context.Background(), tt.path, []byte("data"))
		if !errors.Is(err, tt.want) {
			t.Fatalf("To(%q) error = %v, want %v", tt.path, err, tt.want)
		}
	}

	missingParent := filepath.Join(dir, "absent", "config.yaml")
	if err := fs.To(context.Background(), missingParent, []byte("data")); !errors.Is(err, configerrors.ErrConfigurationDirectoryNotFound) {
		t.Fatalf("To() missing directory error = %v", err)
	}
	if _, err := os.Stat(filepath.Dir(missingParent)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("To() created custom parent directory: %v", err)
	}
}

func TestSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	safe := filepath.Join(dir, "safe")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(safe, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, outside, "secret\n")
	link := filepath.Join(safe, "config.yaml")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, _, err := New(&Config{Path: link}, &testLogger{}).From(context.Background())
	if err == nil {
		t.Fatal("read followed symlink outside its root")
	}
	if err := New(&Config{}, &testLogger{}).To(context.Background(), link, []byte("replacement")); err == nil {
		t.Fatal("save accepted symlink target")
	}
	data, err := os.ReadFile(outside)
	if err != nil || string(data) != "secret\n" {
		t.Fatalf("outside file changed: %q, %v", data, err)
	}
}

func TestAppPathSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	outside := t.TempDir()
	if err := os.MkdirAll(xdg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(xdg, "myapp")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, _, err := New(&Config{AppName: "myapp"}, &testLogger{}).From(context.Background())
	if err == nil {
		t.Fatal("app path escaped home through symlink")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fs := New(&Config{Path: filepath.Join(t.TempDir(), "config.yaml")}, &testLogger{})
	if _, _, err := fs.From(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("From canceled: %v", err)
	}
	if err := fs.To(ctx, filepath.Join(t.TempDir(), "config.yaml"), nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("To canceled: %v", err)
	}
}
