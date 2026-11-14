package fs

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestAppPathRejectsJunctionEscape(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	appDir := filepath.Join(home, "xdg", "myapp")
	if err := os.MkdirAll(filepath.Dir(appDir), 0o700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cmd", "/c", "mklink", "/J", appDir, outside)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot create junction: %v: %s", err, output)
	}
	if _, _, err := New(&Config{AppName: "myapp"}, &testLogger{}).From(context.Background()); err == nil {
		t.Fatal("app path escaped home through junction")
	}
}
