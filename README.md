# Parallel tar extraction utility

Extracting archive files is IO bound. Either use ASYNC IO or threads. Go gives us both!

## Details

The main goroutine opens and reads the tar file. On finding a directory, it creates a directory
inline. On finding a file for extraction, it stores the current offset in the tar file containing
the file data along with the header and sends it via a channel.

Concurrent workers are waiting on a buffered chan for file extraction jobs. The workers pick the file
for extraction and use a copy mechanism that does not alter the position of file descriptor of open tar
file. Reusing open tar file allows us to have as many workers as `ulimit -n` permits.

On Linux, `sendfile` is used and on Darwin, `io.CopyN` with custom `io.Reader` to not update file position
is used.

### Build
The build is driven by a simple Makefile.

To build for current platform
```
$ make all
```

To build for a specific platform
```
$ make GOOS=linux all
```

### Features

```
$ puntar
Usage of puntar:
  -dir string
    	destination directory (default ".")
  -file string
    	tar file to extract
  -update
    	update existing target
  -verbose
    	enable verbose logging
  -workers uint
    	number of concurrent workers (default 4)
```

* Supports updating (additions & modifications only, no deletions) destination folder. This allows continuing from where we left off OR failed.
* Specific number of concurrent workers

## Results

Extraction of a fairly large tar file over NFS yielded **5x** improvement over `GNU tar`

```
$ ls -sh staging-repo.tar
248M staging-repo.tar

$ time tar -xf staging-repo.tar

real	0m15.184s
user	0m0.088s
sys	0m1.080s

$ time puntar -file staging-repo.tar -workers 1000 -dir test

real	0m2.981s
user	0m0.320s
sys	0m0.960s

$ diff -r r-81855 test/r-81855
app-174:~/tmp
$ echo $?
0
```
