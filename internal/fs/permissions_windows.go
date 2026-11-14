package fs

import (
	"os"
	"unsafe"

	"github.com/ygrebnov/errorc"
	"golang.org/x/sys/windows"

	"github.com/ygrebnov/config/pkg/errors"
)

func openReadFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY, 0)
}

type aclHeader struct {
	Revision byte
	Sbz1     byte
	Size     uint16
	Count    uint16
	Sbz2     uint16
}

// validatePrivatePermissions validates the DACL of an already opened file.
func validatePrivatePermissions(file *os.File) error {
	return validatePrivateACL(windows.Handle(file.Fd()))
}

// validatePrivateDirectoryPermissions validates the DACL of the
// application directory represented by root.
func validatePrivateDirectoryPermissions(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()

	info, err := dir.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errorc.With(
			errors.ErrInsecurePermissions,
			errorc.String("validation", "application root is not a directory"),
		)
	}

	return validatePrivateACL(windows.Handle(dir.Fd()))
}

func validatePrivateACL(handle windows.Handle) error {
	sd, err := windows.GetSecurityInfo(
		handle,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	if sd == nil {
		return errorc.With(
			errors.ErrInsecurePermissions,
			errorc.String("validation", "missing security descriptor"),
		)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil {
		return errorc.With(
			errors.ErrInsecurePermissions,
			errorc.String("validation", "NULL DACL is not permitted"),
		)
	}

	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}

	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}

	adminSID, err := windows.CreateWellKnownSid(
		windows.WinBuiltinAdministratorsSid,
	)
	if err != nil {
		return err
	}

	allowed := []*windows.SID{
		user.User.Sid,
		systemSID,
		adminSID,
	}

	header := (*aclHeader)(unsafe.Pointer(dacl))

	for i := uint32(0); i < uint32(header.Count); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE

		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}

		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			// Deny entries cannot grant additional access.
			continue

		case windows.ACCESS_ALLOWED_ACE_TYPE:
			if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
				continue
			}

			sid := (*windows.SID)(
				unsafe.Pointer(&ace.SidStart),
			)

			if !sid.IsValid() {
				return errorc.With(
					errors.ErrInsecurePermissions,
					errorc.String("validation", "invalid SID in ACE"),
				)
			}

			authorized := false

			for _, permitted := range allowed {
				if sid.Equals(permitted) {
					authorized = true
					break
				}
			}

			if !authorized {
				return errorc.With(
					errors.ErrInsecurePermissions,
					errorc.String("validation", "ACL grants access to unauthorized SID"),
					errorc.String("sid", sid.String()),
				)
			}

		default:
			// Fail closed on ACE types we do not understand.
			return errorc.With(
				errors.ErrInsecurePermissions,
				errorc.String("validation", "unsupported ACE type"),
				errorc.Int("ace_type", int(ace.Header.AceType)),
			)
		}
	}

	return nil
}
