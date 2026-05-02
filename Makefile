# See http://clarkgrubb.com/makefile-style-guide
MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
.DELETE_ON_ERROR:
.SUFFIXES:

APP?=go-app
PORT?=8081
PROJECT?=gitlab.encompasshost.com/encompass/$(APP)

GIT_COMMIT := $(shell git rev-parse --short HEAD)
GIT_TAG    := $(shell git describe --tags --always 2>/dev/null || echo "v0.0.0")
BUILD_TIME := $(shell date -u '+%F_%T')

# Docker vars
DOCKER_REGISTRY := registry.encompasshost.com
DOCKER_REPOSITORY ?= encompass/go-app
DOCKER_TAG ?= ${GIT_COMMIT}
DOCKER_IMAGE ?= ${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}:${DOCKER_TAG}

# Go build flags
GOOS := linux
GOARCH := amd64
GOLDFLAGS := '-w -s -X main.Release="${GIT_TAG}" -X main.GitCommit="${GIT_COMMIT}" -X main.BuildTime="${BUILD_TIME}"'

# Help target
help:
	@echo ''
	@echo 'Usage: make [TARGET]'
	@echo 'Targets:'
	@echo '  help     	display this message'
	@echo '  build    	build golang binary'
	@echo '  fmt      	gofmt vendor'
	@echo '  test     	run go test'
	@echo '  lint     	run go linter'
	@echo '  all     	run go fmt lint build (default make)'
	@echo '  clean    	remove the sources'
	@echo '  container	build docker container'
	@echo '  run      	run the docker container'
	@echo '  push     	push to docker repository'
	@echo ''

.PHONY: all
all: fmt lint build

.PHONY: clean
clean:
	rm -f ${APP}

.PHONY: build
build: clean
	@echo "-> $@"
	cd src && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags $(GOLDFLAGS) -o ../${APP} .

.PHONY: test
test:
	cd src && go test -v -race ./...

.PHONY: fmt
fmt:
	@echo "-> $@"
	@if [ -n "$$(gofmt -s -l ./src | grep -v vendor)" ]; then \
		gofmt -s -l ./src | grep -v vendor; \
		exit 1; \
	fi

.PHONY: lint
lint:
	@echo "-> $@"
	@echo go get -u golang.org/x/lint/golint
	@golint ./src/... | tee /dev/stderr

.PHONY: container
container:
	docker build -t ${DOCKER_IMAGE} \
		--build-arg "GIT_TAG=$(GIT_TAG)" \
		--build-arg "GIT_COMMIT=$(GIT_COMMIT)" \
		--build-arg "BUILD_TIME=$(BUILD_TIME)" \
		./src

.PHONY: run
run: container
	docker stop $(APP)-$(DOCKER_TAG) 2>/dev/null || true && docker rm --force $(APP)-${DOCKER_TAG} 2>/dev/null || true
	docker run --name $(APP)-$(DOCKER_TAG) -p ${PORT}:${PORT} --rm -e "PORT=${PORT}" ${DOCKER_IMAGE}

.PHONY: push
push: container
	docker push ${DOCKER_IMAGE}

