# See http://clarkgrubb.com/makefile-style-guide
MAKEFLAGS += --warn-undefined-variables
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
.DELETE_ON_ERROR:
.SUFFIXES:

APP?=go-app
PORT?=8081
PROJECT?=github.com/icicimov/$(APP)

GIT_COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%F_%T')

# Docker vars
DOCKER_REGISTRY := hub.docker.com
DOCKER_REPOSITORY ?= igoratencompass/$(APP)
DOCKER_TAG ?= ${GIT_COMMIT}
DOCKER_IMAGE ?= ${DOCKER_REGISTRY}/${DOCKER_REPOSITORY}:${DOCKER_TAG}

# Go build flags
GOOS := linux
GOARCH := amd64
GOLDFLAGS := -ldflags "-w -s -X ${PROJECT}/version.Release=${DOCKER_TAG} -X ${PROJECT}/version.Commit=${DOCKER_TAG} -X ${PROJECT}/version.BuildTime=${BUILD_TIME}"

.PHONY: all
all: fmt lint build

.PHONY: clean
clean:
	rm -f ${APP}

build: clean
	@echo "-> $@"
	go get -u github.com/prometheus/client_golang/prometheus
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-a $(GOLDFLAGS) -installsuffix cgo -o ${APP} ./src

test:
	go test -v -race ./...

.PHONY: fmt
fmt: 
	@echo "-> $@"
	@gofmt -s -l ./src | grep -v vendor | tee /dev/stderr

.PHONY: lint
lint:
	@echo "-> $@"
	@go get -u golang.org/x/lint/golint
	@golint ./... | grep -v vendor | tee /dev/stderr

container:
	docker build -t ${DOCKER_IMAGE} -e "PORT=${PORT}" -e "GOOS=$(GOOS)" -e "GOARCH=$(GOARCH)" -e "GOLDFLAGS=$(GOLDFLAGS)" ./src

run: container
	docker stop $(APP)-$(DOCKER_TAG) 2>/dev/null || true && docker rm --force $(APP)-${DOCKER_TAG} 2>/dev/null || true
	docker run --name $(APP)-$(DOCKER_TAG) -p ${PORT}:${PORT} --rm -e "PORT=${PORT}" ${DOCKER_IMAGE}

push: container
	docker push ${DOCKER_IMAGE}
