package apiauth

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func readPrivate(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: %w", path, errUnsafe)
	}
	return os.ReadFile(path)
}

// writePrivate creates path (never over an existing file) with its own
// ACL: SYSTEM, Administrators and the current user. Mode bits mean
// nothing on Windows, and ProgramData lets every user read what it
// inherits.
func writePrivate(path, _ string, data []byte) error {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + u.User.Sid.String() + ")")
	if err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return &os.PathError{Op: "create", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	_, err = f.Write(data)
	if err = errors.Join(err, f.Close()); err != nil {
		os.Remove(path)
	}
	return err
}
