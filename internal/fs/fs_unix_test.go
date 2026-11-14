//go:build !windows

package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestFromRejectsNonRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("FIFO unavailable: %v", err)
	}
	_, _, err := New(&Config{Path: path}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrInvalidConfigFilePath) {
		t.Fatalf("FIFO source error = %v", err)
	}
}

func TestFromRejectsPublicFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeTestFile(t, path, "secret: value\n")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := New(&Config{Path: path}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrInsecurePermissions) {
		t.Fatalf("public file error = %v", err)
	}
}

func TestAppPathRejectsPublicDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	path := filepath.Join(home, "xdg", "myapp", "config.yml")
	writeTestFile(t, path, "secret: value\n")
	if err := os.Chmod(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	_, _, err := New(&Config{AppName: "myapp"}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrInsecurePermissions) {
		t.Fatalf("public app directory error = %v", err)
	}
}

func TestOpenedRootSurvivesParentReplacement(t *testing.T) {
	base := t.TempDir()
	trusted := filepath.Join(base, "trusted")
	outside := filepath.Join(base, "outside")
	link := filepath.Join(base, "current")
	for _, dir := range []string{trusted, outside} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(trusted, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	root, name, err := openParent(filepath.Join(link, "config.yaml"), false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	next := link + ".next"
	if err := os.Symlink(outside, next); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(next, link); err != nil {
		t.Fatal(err)
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(trusted, name)); err != nil {
		t.Fatalf("opened root lost original directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, name)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("outside directory modified: %v", err)
	}
}

func TestAppPathConcurrentSymlinkReplacement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	inside := filepath.Join(home, "inside")
	outside := t.TempDir()
	writeTestFile(t, filepath.Join(inside, "config.yml"), "inside\n")
	writeTestFile(t, filepath.Join(outside, "config.yml"), "outside\n")

	link := filepath.Join(home, "xdg", "myapp")
	if err := os.MkdirAll(filepath.Dir(link), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	const iterations = 250
	swapErrors := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range iterations {
			target := inside
			if i%2 == 0 {
				target = outside
			}
			next := link + ".next"
			if err := os.Symlink(target, next); err != nil {
				swapErrors <- err
				return
			}
			if err := os.Rename(next, link); err != nil {
				swapErrors <- err
				return
			}
			runtime.Gosched()
		}
	}()

	for range iterations {
		fs := New(&Config{AppName: "myapp"}, &testLogger{})
		_, data, err := fs.From(context.Background())
		if err == nil && string(data) == "outside\n" {
			t.Fatal("read escaped home during symlink replacement")
		}
		if err == nil {
			_ = fs.To(context.Background(), fs.path, []byte("inside\n"))
		}
		runtime.Gosched()
	}
	<-done
	select {
	case err := <-swapErrors:
		t.Fatal(err)
	default:
	}
	data, err := os.ReadFile(filepath.Join(outside, "config.yml"))
	if err != nil || string(data) != "outside\n" {
		t.Fatalf("outside file changed: %q, %v", data, err)
	}
}
