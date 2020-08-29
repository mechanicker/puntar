package main

import "syscall"

func expandFile(fd int, len int64) error {
	if err := syscall.Ftruncate(fd, len); err != nil {
		return err
	}

	_, err := syscall.Pwrite(fd, []byte(""), len)
	return err
}
