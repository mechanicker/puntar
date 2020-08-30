package main

import (
	"os"
	"syscall"
)

func expandFile(fd int, len int64) error {
	// falloc.h:#define FALLOC_FL_KEEP_SIZE	0x01 /* default is extend size */
	return syscall.Fallocate(fd, 0x01, 0, len)
}

func copyFile(dst *os.File, src *os.File, offset int64, len int64) (int64, error) {
	for written := int64(0); written < len; {
		newOffset := offset + written
		nb, err := syscall.Sendfile(int(dst.Fd()), int(src.Fd()), &newOffset, int(len-written))

		switch err {
		case nil:
			written += int64(nb)
			break
		case syscall.EINTR:
		case syscall.EAGAIN:
			break
		default:
			return written, err
		}
	}

	// Reaches here only on success
	return len, nil
}
