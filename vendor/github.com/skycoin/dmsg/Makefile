ifeq ($(OS),Windows_NT)
	SHELL := pwsh
else
	SHELL := /bin/bash
endif

.DEFAULT_GOAL := help
.PHONY : check lint install-linters dep test build

VERSION := $(shell git describe --always)

RFC_3339 := "+%Y-%m-%dT%H:%M:%SZ"
COMMIT := $(shell git rev-list -1 HEAD)

ifeq ($(OS),Windows_NT)
	BIN := .\bin
    BIN_DIR?=.\bin
    CMD_DIR := .\cmd
	DATE := $(shell powershell -Command date -u ${RFC_3339})
else
	BIN := ${PWD}/bin
	BIN_DIR?=./bin
	CMD_DIR := ./cmd
	DATE := $(shell date -u ${RFC_3339})
endif

OPTS?=GO111MODULE=on

TEST_OPTS:=-tags no_ci -cover -timeout=5m

RACE_FLAG:=-race
GOARCH:=$(shell go env GOARCH)

ifneq (,$(findstring 64,$(GOARCH)))
    TEST_OPTS:=$(TEST_OPTS) $(RACE_FLAG)
endif

DMSG_REPO := github.com/skycoin/dmsg
BUILDINFO_PATH := $(DMSG_REPO)/buildinfo

BUILDINFO_VERSION := -X $(BUILDINFO_PATH).version=$(VERSION)
BUILDINFO_DATE := -X $(BUILDINFO_PATH).date=$(DATE)
BUILDINFO_COMMIT := -X $(BUILDINFO_PATH).commit=$(COMMIT)

BUILDINFO?=$(BUILDINFO_VERSION) $(BUILDINFO_DATE) $(BUILDINFO_COMMIT)

BUILD_OPTS?="-ldflags=$(BUILDINFO)"
BUILD_OPTS_DEPLOY?="-ldflags=$(BUILDINFO) -w -s"

check: lint test ## Run linters and tests

lint: ## Run linters. Use make install-linters first	
	${OPTS} golangci-lint run -c .golangci.yml ./...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately
	${OPTS} go vet -all ./...

vendorcheck:  ## Run vendorcheck
	GO111MODULE=off vendorcheck ./...

test: ## Run tests
	-go clean -testcache &>/dev/null
	${OPTS} go test ${TEST_OPTS} ./...

install-linters: ## Install linters
	# GO111MODULE=off go get -u github.com/FiloSottile/vendorcheck
	# For some reason this install method is not recommended, see https://github.com/golangci/golangci-lint#install
	# However, they suggest `curl ... | bash` which we should not do
	${OPTS} go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	${OPTS} go get -u golang.org/x/tools/cmd/goimports
	${OPTS} go get -u github.com/incu6us/goimports-reviser/v2

format: ## Formats the code. Must have goimports and goimports-reviser installed (use make install-linters).
	${OPTS} goimports -w -local ${DMSG_REPO} .

dep: ## Sorts dependencies
	${OPTS} go mod download
	${OPTS} go mod tidy -v

install: ## Install `dmsg-discovery`, `dmsg-server`, `dmsgget`,`dmsgpty-cli`, `dmsgpty-host`, `dmsgpty-ui`
	${OPTS} go install ${BUILD_OPTS} ./cmd/*

build: ## Build binaries into ./bin
	mkdir -p ${BIN}; go build ${BUILD_OPTS} -o ${BIN} ${CMD_DIR}/*
	
build-windows:
	powershell -Command new-item ${BIN} -itemtype directory -force
	powershell 'Get-ChildItem ${CMD_DIR} | % { go build ${BUILD_OPTS} -o ${BIN} $$_.FullName }'

build-deploy: ## Build for deployment Docker images
	go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/dmsg-discovery ./cmd/dmsg-discovery
	go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/dmsg-server ./cmd/dmsg-server

build-docker:
	./docker/scripts/docker-push.sh -t "develop" -b

start-db: ## Init local database env.
	source ./integration/env.sh && init_redis

stop-db: ## Stop local database env.
	source ./integration/env.sh && stop_redis

attach-db: ## Attach local database env.
	source ./integration/env.sh && attach_redis

start-dmsg: build ## Init local dmsg env.
	source ./integration/env.sh && init_dmsg

stop-dmsg: ## Stop local dmsg env.
	source ./integration/env.sh && stop_dmsg

attach-dmsg: ## Attach local dmsg tmux session.
	source ./integration/env.sh && attach_dmsg

start-pty: build ## Init local dmsgpty env.
	source ./integration/env.sh && init_dmsgpty

stop-pty: ## Stop local dmsgpty env.
	source ./integration/env.sh && stop_dmsgpty

attach-pty: ## Attach local dmsgpty tmux session.
	source ./integration/env.sh && attach_dmsgpty

stop-all: stop-pty stop-dmsg stop-db ## Stop all local tmux sessions.

integration-windows-start:
	powershell -Command .\integration\integration.ps1 start

integration-windows-stop:
	powershell -Command .\integration\integration.ps1 stop

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
