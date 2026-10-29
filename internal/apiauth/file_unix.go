//go:build unix

package apiauth

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// readPrivate reads path, refusing symlinks and files others can read.
func readPrivate(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, syscall.ELOOP) {
		return nil, fmt.Errorf("%s is a symlink: %w", path, errUnsafe)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("%s: %w", path, errUnsafe)
	}
	b := make([]byte, 256)
	n, err := f.Read(b)
	if err != nil {
		return nil, err
	}
	return b[:n], nil
}

// writePrivate creates path (never over an existing file or symlink),
// mode 0600. As root it hands the file to the owner of ownerOf.
func writePrivate(path, ownerOf string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	if os.Geteuid() == 0 && ownerOf != "" {
		if fi, err := os.Stat(ownerOf); err == nil {
			if st, ok := fi.Sys().(*syscall.Stat_t); ok {
				err = f.Chown(int(st.Uid), int(st.Gid))
			}
			if err != nil {
				f.Close()
				os.Remove(path)
				return err
			}
		}
	}
	_, err = f.Write(data)
	if err = errors.Join(err, f.Close()); err != nil {
		os.Remove(path)
	}
	return err
}
