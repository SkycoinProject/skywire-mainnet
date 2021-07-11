.DEFAULT_GOAL := help

.PHONY : check lint lint-extra install-linters dep test
.PHONY : build clean install format  bin
.PHONY : host-apps bin
.PHONY : docker-image docker-clean docker-network
.PHONY : docker-apps docker-bin docker-volume
.PHONY : docker-run docker-stop
.PHONY : sysroot sysroot-clean

SHELL := /bin/bash
VERSION := $(shell git describe)
#VERSION := v0.1.0 # for debugging updater

RFC_3339 := "+%Y-%m-%dT%H:%M:%SZ"
DATE := $(shell date -u $(RFC_3339))
COMMIT := $(shell git rev-list -1 HEAD)
BRANCH := latest

PROJECT_BASE := github.com/skycoin/skywire
DMSG_BASE := github.com/skycoin/dmsg
OPTS?=GO111MODULE=on
STATIC_OPTS?= $(OPTS) CC=musl-gcc
MANAGER_UI_DIR = static/skywire-manager-src
GO_BUILDER_VERSION=v1.16.4
MANAGER_UI_BUILT_DIR=cmd/skywire-visor/static

TEST_OPTS:=-cover -timeout=5m -mod=vendor

GOARCH:=$(shell go env GOARCH)

ifneq (,$(findstring 64,$(GOARCH)))
    TEST_OPTS:=$(TEST_OPTS) -race
endif

BUILDINFO_PATH := $(DMSG_BASE)/buildinfo

BUILDINFO_VERSION := -X $(BUILDINFO_PATH).version=$(VERSION)
BUILDINFO_DATE := -X $(BUILDINFO_PATH).date=$(DATE)
BUILDINFO_COMMIT := -X $(BUILDINFO_PATH).commit=$(COMMIT)

BUILDINFO?=$(BUILDINFO_VERSION) $(BUILDINFO_DATE) $(BUILDINFO_COMMIT)

BUILD_OPTS?="-ldflags=$(BUILDINFO)" -mod=vendor $(RACE_FLAG)
BUILD_OPTS_DEPLOY?="-ldflags=$(BUILDINFO) -w -s"

check: lint test ## Run linters and tests

build: host-apps bin ## Install dependencies, build apps and binaries. `go build` with ${OPTS}

build-systray: host-apps bin-systray ## Install dependencies, build apps and binaries `go build` with ${OPTS}, with CGO and systray

build-static: host-apps-static bin-static ## Build apps and binaries. `go build` with ${OPTS}

install-generate: ## Installs required execs for go generate.
	${OPTS} go install github.com/mjibson/esc
	${OPTS} go install github.com/vektra/mockery/cmd/mockery
	# If the following does not work, you may need to run:
	# 	git config --global url.git@github.com:.insteadOf https://github.com/
	# Source: https://stackoverflow.com/questions/27500861/whats-the-proper-way-to-go-get-a-private-repository
	# We are using 'go get' instead of 'go install' here, because we don't have a git tag in which 'readmegen' is already implemented.
	${OPTS} go get -u github.com/SkycoinPro/skywire-services/cmd/readmegen

generate: ## Generate mocks and config README's
	go generate ./...

clean: ## Clean project: remove created binaries and apps
	-rm -rf ./apps
	-rm -f ./skywire-visor ./skywire-cli ./setup-node

install: ## Install `skywire-visor`, `skywire-cli`, `setup-node`
	${OPTS} go install ${BUILD_OPTS} ./cmd/skywire-visor ./cmd/skywire-cli ./cmd/setup-node

install-static: ## Install `skywire-visor`, `skywire-cli`, `setup-node`
	${STATIC_OPTS} go install -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' ./cmd/skywire-visor ./cmd/skywire-cli ./cmd/setup-node

lint: ## Run linters. Use make install-linters first
	${OPTS} golangci-lint run -c .golangci.yml ./...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately

lint-ci:
	${OPTS} golangci-lint run --build-tags=musl -c .golangci.yml ./...

lint-extra: ## Run linters with extra checks.
	${OPTS} golangci-lint run --no-config --enable-all ./...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately
	${OPTS} go vet -all ./...

test: ## Run tests
	-go clean -testcache &>/dev/null
	${OPTS} go test ${TEST_OPTS} ./internal/...
	${OPTS} go test ${TEST_OPTS} ./pkg/...

install-linters: ## Install linters
	- VERSION=latest ./ci_scripts/install-golangci-lint.sh
	${OPTS} go get -u golang.org/x/tools/cmd/goimports
	${OPTS} go get -u github.com/incu6us/goimports-reviser/v2

tidy: ## Tidies and vendors dependencies.
	${OPTS} go mod tidy -v

format: tidy ## Formats the code. Must have goimports and goimports-reviser installed (use make install-linters).
	${OPTS} goimports -w -local ${PROJECT_BASE} ./pkg
	${OPTS} goimports -w -local ${PROJECT_BASE} ./cmd
	${OPTS} goimports -w -local ${PROJECT_BASE} ./internal
	find . -type f -name '*.go' -not -path "./vendor/*"  -exec goimports-reviser -project-name ${PROJECT_BASE} -file-path {} \;

dep: tidy ## Sorts dependencies
	${OPTS} go mod vendor -v

snapshot-systray: sysroot ## create snapshot release
	docker run --rm --privileged \
		-v $(CURDIR):/go/src/github.com/skycoin/skywire \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(GOPATH)/src:/go/src \
		-v $(CURDIR)/sysroot:/sysroot \
		-w /go/src/github.com/skycoin/skywire \
		alexadhyatma/golang-cross:$(GO_BUILDER_VERSION) -f /go/src/github.com/skycoin/skywire/.goreleaser-systray.yml --snapshot --skip-publish --rm-dist

snapshot:
	goreleaser --snapshot --skip-publish --rm-dist

snapshot-clean: ## Cleans snapshot / release
	rm -rf ./dist

sysroot:
	mkdir -p ./sysroot
	@echo "getting sysroot for cross compilation"
	if [[ ! -f /tmp/snapshot-05-12-2021.tar.gz ]]; then \
  		curl -L -o /tmp/snapshot-05-12-2021.tar.gz "https://alexadhy-git.s3-ap-southeast-1.amazonaws.com/snapshot-05-12-2021.tar.gz"; \
	fi
	tar xf /tmp/snapshot-05-12-2021.tar.gz -C ./sysroot/

sysroot-clean:
	@rm -rf ./sysroot
	@rm -rf /tmp/sysroot-git

host-apps: ## Build app
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skychat ./cmd/apps/skychat
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks ./cmd/apps/skysocks
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks-client  ./cmd/apps/skysocks-client
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-server ./cmd/apps/vpn-server
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-client ./cmd/apps/vpn-client

# Static Apps
host-apps-static: ## Build app
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./apps/skychat ./cmd/apps/skychat
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./apps/skysocks ./cmd/apps/skysocks
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./apps/skysocks-client  ./cmd/apps/skysocks-client
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./apps/vpn-server ./cmd/apps/vpn-server
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./apps/vpn-client ./cmd/apps/vpn-client

# Bin
bin: ## Build `skywire-visor`, `skywire-cli`
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-cli  ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS} -o ./setup-node ./cmd/setup-node

bin-systray:
	${OPTS} go build ${BUILD_OPTS} -tags systray -o ./skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS} -tags systray -o ./skywire-cli ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS} -tags systray -o ./setup-node ./cmd/setup-node

# Static Bin
bin-static: ## Build `skywire-visor`, `skywire-cli`
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./skywire-visor ./cmd/skywire-visor
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./skywire-cli  ./cmd/skywire-cli
	${STATIC_OPTS} go build -trimpath --ldflags '-linkmode external -extldflags "-static" -buildid=' -o ./setup-node ./cmd/setup-node

build-deploy: ## Build for deployment Docker images
	${OPTS} go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/skywire-cli ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skychat ./cmd/apps/skychat
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skysocks ./cmd/apps/skysocks
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skysocks-client ./cmd/apps/skysocks-client

github-release-systray: sysroot ## Create a GitHub release
	docker run --rm --privileged \
		-v $(CURDIR):/go/src/github.com/skycoin/skywire \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(GOPATH)/src:/go/src \
		-v $(CURDIR)/sysroot:/sysroot \
		-w /go/src/github.com/skycoin/skywire \
		alexadhyatma/golang-cross:$(GO_BUILDER_VERSION) -f /go/src/github.com/skycoin/skywire/.goreleaser-systray.yml --rm-dist

github-release:
	goreleaser --rm-dist

build-docker: ## Build docker image
	./ci_scripts/docker-push.sh -t ${BRANCH} -b

# Manager UI
install-deps-ui:  ## Install the UI dependencies
	cd $(MANAGER_UI_DIR) && npm ci

run: ## Run skywire visor with skywire-config.json, and start a browser if running a hypervisor
	./skywire-visor -c ./skywire-config.json

lint-ui:  ## Lint the UI code
	cd $(MANAGER_UI_DIR) && npm run lint

build-ui: install-deps-ui  ## Builds the UI
	cd $(MANAGER_UI_DIR) && npm run build
	mkdir -p ${PWD}/bin
	rm -rf ${MANAGER_UI_BUILT_DIR}
	mkdir ${MANAGER_UI_BUILT_DIR}
	cp -r ${MANAGER_UI_DIR}/dist/. ${MANAGER_UI_BUILT_DIR}

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
