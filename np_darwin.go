package main

import (
	"io"
	"os"
	"syscall"
)

func expandFile(fd int, len int64) error {
	if err := syscall.Ftruncate(fd, len); err != nil {
		return err
	}

	_, err := syscall.Pwrite(fd, []byte(""), len)
	return err
}

type readAt struct {
	*os.File
	offset int64
}

// Read implements io.Reader interface using ReadAt() without modifying file descriptor position
// by using pread() internally
func (r *readAt) Read(p []byte) (n int, err error) {
	nb, err := r.ReadAt(p, r.offset)
	if err == nil {
		// Update internal offset in case a retry happens due to error
		r.offset += int64(nb)
	}

	return nb, err
}

func copyFile(dst *os.File, src *os.File, offset int64, len int64) (int64, error) {
	return io.CopyN(dst, &readAt{src, offset}, len)
}
