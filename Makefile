export GOOS=$(shell uname -s|tr '[A-Z]' '[a-z]')
DEPS=$(shell go list -f '{{.GoFiles}}'|tr -d '[]')

all: puntar

puntar: Makefile $(DEPS)
	@echo "Building: $(DEPS)"
	go build -trimpath

clean:
	rm -f puntar

.PHONEY: clean
