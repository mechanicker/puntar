package main

import "syscall"

func expandFile(fd int, len int64) error {
	// falloc.h:#define FALLOC_FL_KEEP_SIZE	0x01 /* default is extend size */
	return syscall.Fallocate(fd, 0x01, 0, len)
}
