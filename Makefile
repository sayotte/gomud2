SHELL=/bin/bash
ROOT = $(shell pwd)
export GOPATH := ${ROOT}
export PATH := ${ROOT}/bin:${PATH}
PROJ_ROOT = github.com/sayotte/gomud2
LINT_COMMON = --deadline=60s \
		--dupl-threshold=75 \
		--cyclo-over=45
LINT_EXCLUSIONS = --exclude='Errors unhandled.,LOW,HIGH' \
		--exclude='Expect directory permissions to be 0700 or less,MEDIUM,HIGH' \
		--exclude='should have comment or be unexported' \
		--exclude='should be'
TEST_FLAGS ?= -cover

VERSION := $(shell git --no-pager describe --tags --always)

.PHONY: *

all:	lint test install

clean: clean_estates_files
	rm -rf bin/
	rm -rf pkg/
	go clean ./...

test:
	go test ${TEST_FLAGS} -timeout=15m ${PROJ_ROOT}/...

# install gometalinter.  because the binary expects linters to be in $GOPATH/src/github.com/alecthomas/, we must
# symlink the vendor package to that directory.
# a step of `make clean` will remove the symlinked directory.
gometalinter:
	go install github.com/alecthomas/gometalinter
	#mkdir -p src/github.com/alecthomas
	#ln -sf ../../vendor/github.com/alecthomas/gometalinter src/github.com/alecthomas
	gometalinter --install

lint: gometalinter
	cd src && gometalinter ${LINT_COMMON} ${PROJ_ROOT}/... ${LINT_EXCLUSIONS}

install:
	go fmt ${PROJ_ROOT}/... # go fmt is run here for sanity reason
	go install -ldflags "-X main.version=${VERSION}" ${PROJ_ROOT}/...

# start up a shell, used to easily set GOPATH
shell:
	bash

