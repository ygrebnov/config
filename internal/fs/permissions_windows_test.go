package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sharedfs "github.com/ygrebnov/fs"
	"golang.org/x/sys/windows"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

func TestWindowsPrivateAndPublicFileACL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	setTestPathACL(t, path, privateACL(t))
	if err := sharedfs.ValidatePrivateFile(file); err != nil {
		t.Fatalf("private file rejected: %v", err)
	}

	setTestPathACL(t, path, privateACL(t)+"(A;;FR;;;WD)")
	if err := sharedfs.ValidatePrivateFile(file); !errors.Is(err, sharedfs.ErrInsecurePermissions) {
		t.Fatalf("Everyone-readable file accepted: %v", err)
	}
	_, _, err = New(&Config{Path: path}, &testLogger{}).From(context.Background())
	if !errors.Is(err, configerrors.ErrInsecurePermissions) || !errors.Is(err, sharedfs.ErrInsecurePermissions) {
		t.Fatalf("config file permission error = %v", err)
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
	if err := validateAppRootPermissions(root); err != nil {
		t.Fatalf("private directory rejected: %v", err)
	}
	setTestPathACL(t, dir, privateACL(t)+"(A;;FR;;;BU)")
	if err := validateAppRootPermissions(root); !errors.Is(err, configerrors.ErrInsecurePermissions) || !errors.Is(err, sharedfs.ErrInsecurePermissions) {
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
