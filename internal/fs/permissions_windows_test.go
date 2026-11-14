package fs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	"golang.org/x/sys/windows"
)

func TestWindowsPrivateAndPublicFileACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	setTestPathACL(t, path, privateACL(t))
	if err := validatePrivatePermissions(file); err != nil {
		t.Fatalf("private file rejected: %v", err)
	}

	setTestPathACL(t, path, privateACL(t)+"(A;;FR;;;WD)")
	if err := validatePrivatePermissions(file); !errors.Is(err, configerrors.ErrInsecurePermissions) {
		t.Fatalf("Everyone-readable file accepted: %v", err)
	}
}

func TestWindowsPrivateAndPublicDirectoryACL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "app")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	setTestPathACL(t, dir, privateACL(t))
	if err := validatePrivateDirectoryPermissions(root); err != nil {
		t.Fatalf("private directory rejected: %v", err)
	}
	setTestPathACL(t, dir, privateACL(t)+"(A;;FR;;;BU)")
	if err := validatePrivateDirectoryPermissions(root); !errors.Is(err, configerrors.ErrInsecurePermissions) {
		t.Fatalf("Users-readable directory accepted: %v", err)
	}
}

func privateACL(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return "D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + user.User.Sid.String() + ")"
}

func setTestPathACL(t *testing.T, path, sddl string) {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	); err != nil {
		t.Fatal(err)
	}
}
