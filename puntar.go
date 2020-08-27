package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

type jobInfo struct {
	offset int64
	hdr    *tar.Header
}

func main() {
	tarFile := flag.String("file", "", "tar file to extract")
	isVerbose := flag.Bool("verbose", false, "enable verbose logging")
	numWorkers := flag.Uint("workers", 4, "number of concurrent workers")
	flag.Parse()

	if len(*tarFile) == 0 {
		log.Fatal("error: missing archive file to extract")
	}

	tarFh, err := os.Open(*tarFile)
	if err != nil {
		log.Fatalf("error: failed to open archive file \"%s\" to extract with error: %v", *tarFile, err)
	}
	defer func() { _ = tarFh.Close() }()

	tarReader := tar.NewReader(tarFh)
	wg := sync.WaitGroup{}
	files := make(chan jobInfo, *numWorkers)
	worker := func() {
		srcFh, err := os.Open(*tarFile)
		if err != nil {
			log.Fatalf("error %v re-opening tar hdr \"%s\"", err, *tarFile)
		}

		defer func() {
			_ = srcFh.Close()
		}()

		for job := range files {
			if offset, err := srcFh.Seek(job.offset, io.SeekStart); err != nil || offset != job.offset {
				log.Fatal("failed to seek tar hdr for async hdr extraction")
			}

			dstFh, err := os.OpenFile(job.hdr.Name, os.O_CREATE|os.O_RDWR, os.FileMode(job.hdr.Mode))
			if err != nil {
				log.Fatal(err)
			}

			if nb, err := io.CopyN(dstFh, srcFh, job.hdr.Size); err != nil || nb != job.hdr.Size {
				log.Fatalf("error extracting hdr \"%s\" with error: %v", job.hdr.Name, err)
			}

			wg.Done()
			_ = dstFh.Close()

			if *isVerbose {
				fmt.Printf("%s\n", job.hdr.Name)
			}
		}
	}

	// Concurrent workers
	for i := uint(0); i < (*numWorkers); i++ {
		go worker()
	}

	// Process the tar file
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
			if fserr := os.Mkdir(hdr.Name, hdr.FileInfo().Mode()); fserr != nil {
				log.Fatalf("failed to create directory \"%s\" with error %v", hdr.Name, fserr)
			}

			if *isVerbose {
				fmt.Printf("%s\n", hdr.Name)
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
				fmt.Printf("skipping type=%d, name=%s\n", hdr.Typeflag, hdr.Name)
			}
		}
	}

	wg.Wait()
}
