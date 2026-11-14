package path

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestGetConfigPathValidatesAppName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	for _, name := range []string{"", ".", "..", "../escape", "nested/app", "nested\\app", " app", "app.", "CON", "COM1.txt", "bad:name"} {
		if _, err := GetConfigPath(name); !errors.Is(err, configerrors.ErrInvalidAppName) {
			t.Errorf("GetConfigPath(%q) error = %v, want ErrInvalidAppName", name, err)
		}
	}

	got, err := GetConfigPath("myapp")
	want := filepath.Join(home, ".config", "myapp", DefaultConfigFilename)
	if err != nil || got != want {
		t.Fatalf("GetConfigPath(myapp) = %q, %v, want %q", got, err, want)
	}
}

func TestGetConfigPathConfinesConfigBase(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for _, base := range []string{"relative", filepath.Dir(home)} {
		t.Setenv("XDG_CONFIG_HOME", base)
		if _, err := GetConfigPath("myapp"); !errors.Is(err, configerrors.ErrInvalidConfigFilePath) {
			t.Errorf("XDG_CONFIG_HOME=%q: %v", base, err)
		}
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	if _, err := GetConfigPath("myapp"); err != nil {
		t.Fatal(err)
	}

	// The home must exist for rooted app-path operations.
	if _, err := os.Stat(home); err != nil {
		t.Fatal(err)
	}
}
