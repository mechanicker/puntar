package main

import (
	"archive/tar"
	"flag"
	"io"
	"log"
	"os"
	"sync"
	"syscall"
)

type jobInfo struct {
	offset int64
	hdr    *tar.Header
}

func main() {
	tarFile := flag.String("file", "", "tar file to extract")
	numWorkers := flag.Uint("workers", 4, "number of concurrent workers")
	isUpdate := flag.Bool("update", false, "update existing target")
	isVerbose := flag.Bool("verbose", false, "enable verbose logging")
	flag.Parse()

	if len(*tarFile) == 0 {
		flag.Usage()
		log.Fatalf("%v: missing archive file to extract", os.ErrInvalid)
	}

	// Validate workers count and try to ensure it does not hit open file limits
	var openFileLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &openFileLimit); err == nil {
		// Each worker opens the destination file
		// 10: stdin, stdout, stderr, event loop, 2 pipes, 1 (main tar file)  + 3 extras
		const approxOpenFds = 10
		if openFileLimit.Cur < uint64(*numWorkers)+approxOpenFds {
			flag.Usage()
			maxWorkers := openFileLimit.Cur - approxOpenFds
			log.Fatalf("error: too many workers, should be less than: %d", maxWorkers)
		}
	}

	blockSize := int64(1024)
	var stat syscall.Statfs_t
	if err := syscall.Statfs(*tarFile, &stat); err == nil {
		blockSize = int64(stat.Bsize)
	}

	// Note: tarFh is shared across all workers
	tarFh, err := os.Open(*tarFile)
	if err != nil {
		log.Fatalf("failed to open tar file \"%s\" with error: %v", *tarFile, err)
	}
	defer func() { _ = tarFh.Close() }()

	tarReader := tar.NewReader(tarFh)
	wg := sync.WaitGroup{}
	files := make(chan jobInfo, *numWorkers)
	worker := func() {
		for job := range files {
			if *isUpdate {
				if fileInfo, fsErr := os.Stat(job.hdr.Name); fsErr == nil && fileInfo.Size() == job.hdr.Size && fileInfo.Mode() == os.FileMode(job.hdr.Mode) {
					if *isVerbose {
						log.Printf("exists: skipping file: %s\n", job.hdr.Name)
					}

					wg.Done()
					continue
				}
			}

			dstFh, err := os.OpenFile(job.hdr.Name, os.O_CREATE|os.O_TRUNC|os.O_RDWR, os.FileMode(job.hdr.Mode))
			if err != nil {
				log.Fatalf("failed to open file \"%s\" for write with error: %v", job.hdr.Name, err)
			}

			// Best effort pre-allocate file size to 1 block less that actual file size
			// On a failed copy, we will have a file that is smaller in size than original
			_ = expandFile(int(dstFh.Fd()), job.hdr.Size-blockSize)

			// copyFile does not depend on position of file descriptor. Hence, allows concurrent reuse
			if nb, err := copyFile(dstFh, tarFh, job.offset, job.hdr.Size); err != nil || nb != job.hdr.Size {
				// Best effort cleanup of failed extraction
				_ = os.Remove(dstFh.Name())

				log.Fatalf("failed copying file \"%s\" with error: %v", job.hdr.Name, err)
			}

			if *isVerbose {
				log.Printf("%s\n", job.hdr.Name)
			}

			wg.Done()
			_ = dstFh.Close()
		}
	}

	// Consumers: Concurrent workers for extraction
	for i := uint(0); i < *numWorkers; i++ {
		go worker()
	}

	// Producer: Read tar file in main goroutine
	for {
		hdr, err := tarReader.Next()
		if err != nil {
			close(files)
			break
		}

		// Create directories in main thread to ensure they are available when
		// extracting files in concurrent goroutines
		switch hdr.Typeflag {
		case tar.TypeDir:
			if fsErr := os.Mkdir(hdr.Name, hdr.FileInfo().Mode()); fsErr != nil {
				if pathErr, ok := fsErr.(*os.PathError); !(ok && *isUpdate && pathErr.Err == syscall.EEXIST) {
					log.Fatalf("failed to create directory \"%s\" with error %v", hdr.Name, fsErr)
				}
			}

			if *isVerbose {
				log.Printf("%s\n", hdr.Name)
			}
		case tar.TypeReg:
			if offset, err := tarFh.Seek(0, io.SeekCurrent); err == nil {
				wg.Add(1)
				files <- jobInfo{offset, hdr}
			} else {
				log.Fatalf("failed to dup tar file descriptor: %v", err)
			}
		default:
			if *isVerbose {
				log.Printf("unsupported: skipping type=%d, name=%s\n", hdr.Typeflag, hdr.Name)
			}
		}
	}

	wg.Wait()
}
