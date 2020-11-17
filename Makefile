.DEFAULT_GOAL := help

.PHONY : check lint lint-extra install-linters dep test
.PHONY : build clean install format  bin
.PHONY : host-apps bin
.PHONY : run stop config
.PHONY : docker-image docker-clean docker-network
.PHONY : docker-apps docker-bin docker-volume
.PHONY : docker-run docker-stop

VERSION := $(shell git describe)
#VERSION := v0.1.0 # for debugging updater

RFC_3339 := "+%Y-%m-%dT%H:%M:%SZ"
DATE := $(shell date -u $(RFC_3339))
COMMIT := $(shell git rev-list -1 HEAD)

PROJECT_BASE := github.com/skycoin/skywire
DMSG_BASE := github.com/skycoin/dmsg
OPTS?=GO111MODULE=on
MANAGER_UI_DIR = static/skywire-manager-src
DOCKER_IMAGE?=skywire-runner # docker image to use for running skywire-visor.`golang`, `buildpack-deps:stretch-scm`  is OK too
DOCKER_NETWORK?=SKYNET
DOCKER_NODE?=SKY01
DOCKER_OPTS?=GO111MODULE=on GOOS=linux # go options for compiling for docker container

TEST_OPTS_BASE:=-cover -timeout=5m -mod=vendor

RACE_FLAG:=-race
GOARCH:=$(shell go env GOARCH)

ifneq (,$(findstring 64,$(GOARCH)))
    TEST_OPTS_BASE:=$(TEST_OPTS_BASE) $(RACE_FLAG)
endif

TEST_OPTS_NOCI:=-$(TEST_OPTS_BASE) -v
TEST_OPTS:=$(TEST_OPTS_BASE) -tags no_ci

BUILDINFO_PATH := $(DMSG_BASE)/buildinfo

BUILDINFO_VERSION := -X $(BUILDINFO_PATH).version=$(VERSION)
BUILDINFO_DATE := -X $(BUILDINFO_PATH).date=$(DATE)
BUILDINFO_COMMIT := -X $(BUILDINFO_PATH).commit=$(COMMIT)

BUILDINFO?=$(BUILDINFO_VERSION) $(BUILDINFO_DATE) $(BUILDINFO_COMMIT)

BUILD_OPTS?="-ldflags=$(BUILDINFO)"
BUILD_OPTS_DEPLOY?="-ldflags=$(BUILDINFO) -w -s"

check: lint test ## Run linters and tests

build: dep host-apps bin ## Install dependencies, build apps and binaries. `go build` with ${OPTS}

run: stop build	config  ## Run skywire-visor on host
	./skywire-visor skywire.json

stop: ## Stop running skywire-visor on host
	-bash -c "kill $$(ps aux |grep '[s]kywire-visor' |awk '{print $$2}')"

config: ## Generate skywire.json
	-./skywire-cli visor gen-config -o  ./skywire.json -r

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

rerun: stop
	${OPTS} go build -race -o ./skywire-visor ./cmd/skywire-visor
	-./skywire-cli visor gen-config -o  ./skywire.json -r
	perl -pi -e 's/localhost//g' ./skywire.json
	./skywire-visor skywire.json

lint: ## Run linters. Use make install-linters first
	${OPTS} golangci-lint run -c .golangci.yml ./...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately

lint-extra: ## Run linters with extra checks.
	${OPTS} golangci-lint run --no-config --enable-all ./...
	# The govet version in golangci-lint is out of date and has spurious warnings, run it separately
	${OPTS} go vet -all ./...

vendorcheck:  ## Run vendorcheck
	GO111MODULE=off vendorcheck ./internal/...
	GO111MODULE=off vendorcheck ./pkg/...
	GO111MODULE=off vendorcheck ./cmd/apps/...
	GO111MODULE=off vendorcheck ./cmd/setup-node/...
	GO111MODULE=off vendorcheck ./cmd/skywire-cli/...
	GO111MODULE=off vendorcheck ./cmd/skywire-visor/...

test: ## Run tests
	-go clean -testcache &>/dev/null
	${OPTS} go test ${TEST_OPTS} ./internal/...
	${OPTS} go test ${TEST_OPTS} ./pkg/...

test-no-ci: ## Run no_ci tests
	-go clean -testcache
	${OPTS} go test ${TEST_OPTS_NOCI} ./pkg/transport/... -run "TCP|PubKeyTable"

install-linters: ## Install linters
	- VERSION=latest ./ci_scripts/install-golangci-lint.sh
	# GO111MODULE=off go get -u github.com/FiloSottile/vendorcheck
	# For some reason this install method is not recommended, see https://github.com/golangci/golangci-lint#install
	# However, they suggest `curl ... | bash` which we should not do
	# ${OPTS} go get -u github.com/golangci/golangci-lint/cmd/golangci-lint
	${OPTS} go get -u golang.org/x/tools/cmd/goimports
	${OPTS} go get -u github.com/incu6us/goimports-reviser

tidy: ## Tidies and vendors dependencies.
	${OPTS} go mod tidy -v
	${OPTS} go mod vendor -v

format: tidy ## Formats the code. Must have goimports and goimports-reviser installed (use make install-linters).
	${OPTS} goimports -w -local ${PROJECT_BASE} ./pkg
	${OPTS} goimports -w -local ${PROJECT_BASE} ./cmd
	${OPTS} goimports -w -local ${PROJECT_BASE} ./internal
	find . -type f -name '*.go' -not -path "./vendor/*"  -exec goimports-reviser -project-name ${PROJECT_BASE} -file-path {} \;

dep: ## Sorts dependencies
	${OPTS} go mod vendor -v

# Apps
host-apps: ## Build app
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skychat ./cmd/apps/skychat
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks ./cmd/apps/skysocks
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks-client  ./cmd/apps/skysocks-client
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-server ./cmd/apps/vpn-server
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-client ./cmd/apps/vpn-client

# Bin
bin: ## Build `skywire-visor`, `skywire-cli`
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-cli  ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS} -o ./setup-node ./cmd/setup-node

release: ## Build `skywire-visor`, `skywire-cli` and apps without -race flag
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS} -o ./skywire-cli  ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS} -o ./setup-node ./cmd/setup-node
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skychat ./cmd/apps/skychat
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks ./cmd/apps/skysocks
	${OPTS} go build ${BUILD_OPTS} -o ./apps/skysocks-client  ./cmd/apps/skysocks-client
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-server ./cmd/apps/vpn-server
	${OPTS} go build ${BUILD_OPTS} -o ./apps/vpn-client ./cmd/apps/vpn-client

build-deploy: ## Build for deployment Docker images
	${OPTS} go build -tags netgo ${BUILD_OPTS_DEPLOY} -o /release/skywire-visor ./cmd/skywire-visor
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/skywire-cli ./cmd/skywire-cli
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skychat ./cmd/apps/skychat
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skysocks ./cmd/apps/skysocks
	${OPTS} go build ${BUILD_OPTS_DEPLOY} -o /release/apps/skysocks-client ./cmd/apps/skysocks-client

package-amd64: ## Build the debian package.
	scripts/dPKGBUILD.sh amd64

package-arm64: ## Build the debian package.
	scripts/dPKGBUILD.sh arm64

package-armhf: ## Build the debian package.
	scripts/dPKGBUILD.sh armhf

all-packages: ## Build the debian package.
	scripts/dPKGBUILD.sh amd64
	scripts/dPKGBUILD.sh arm64
	scripts/dPKGBUILD.sh armhf

github-release: ## Create a GitHub release
	goreleaser --rm-dist

# Manager UI
install-deps-ui:  ## Install the UI dependencies
	cd $(MANAGER_UI_DIR) && npm ci

lint-ui:  ## Lint the UI code
	cd $(MANAGER_UI_DIR) && npm run lint

build-ui: install-deps-ui  ## Builds the UI
	cd $(MANAGER_UI_DIR) && npm run build
	mkdir -p ${PWD}/bin
	${OPTS} GOBIN=${PWD}/bin go get github.com/rakyll/statik
	${PWD}/bin/statik -src=$(MANAGER_UI_DIR)/dist -dest ./cmd/skywire-visor -f

# Dockerized skywire-visor
docker-image: ## Build docker image `skywire-runner`
	docker image build --tag=skywire-runner --rm  - < skywire-runner.Dockerfile

docker-clean: ## Clean docker system: remove container ${DOCKER_NODE} and network ${DOCKER_NETWORK}
	-docker network rm ${DOCKER_NETWORK}
	-docker container rm --force ${DOCKER_NODE}

docker-network: ## Create docker network ${DOCKER_NETWORK}
	-docker network create ${DOCKER_NETWORK}

docker-apps: ## Build apps binaries for dockerized skywire-visor. `go build` with  ${DOCKER_OPTS}
	-${DOCKER_OPTS} go build -race -o ./visor/apps/skychat ./cmd/apps/skychat
	-${DOCKER_OPTS} go build -race -o ./visor/apps/skysocks ./cmd/apps/skysocks
	-${DOCKER_OPTS} go build -race -o ./visor/apps/skysocks-client  ./cmd/apps/skysocks-client

docker-bin: ## Build `skywire-visor`, `skywire-cli`. `go build` with  ${DOCKER_OPTS}
	${DOCKER_OPTS} go build -race -o ./visor/skywire-visor ./cmd/skywire-visor

docker-volume: dep docker-apps docker-bin bin  ## Prepare docker volume for dockerized skywire-visor
	-${DOCKER_OPTS} go build  -o ./docker/skywire-services/setup-node ./cmd/setup-node
	-./skywire-cli visor gen-config -o  ./skywire-visor/skywire.json -r
	perl -pi -e 's/localhost//g' ./visor/skywire.json # To make visor accessible from outside with skywire-cli

docker-run: docker-clean docker-image docker-network docker-volume ## Run dockerized skywire-visor ${DOCKER_NODE} in image ${DOCKER_IMAGE} with network ${DOCKER_NETWORK}
	docker run -it -v $(shell pwd)/visor:/sky --network=${DOCKER_NETWORK} \
		--name=${DOCKER_NODE} ${DOCKER_IMAGE} bash -c "cd /sky && ./skywire-visor skywire.json"

docker-setup-node:	## Runs setup-node in detached state in ${DOCKER_NETWORK}
	-docker container rm setup-node -f
	docker run -d --network=${DOCKER_NETWORK}  	\
	 				--name=setup-node	\
	 				--hostname=setup-node	skywire-services \
					  bash -c "./setup-node setup-node.json"

docker-stop: ## Stop running dockerized skywire-visor ${DOCKER_NODE}
	-docker container stop ${DOCKER_NODE}

docker-rerun: docker-stop
	-./skywire-cli gen-config -o ./visor/skywire.json -r
	perl -pi -e 's/localhost//g' ./visor/skywire.json # To make visor accessible from outside with skywire-cli
	${DOCKER_OPTS} go build -race -o ./visor/skywire-visor ./cmd/skywire-visor
	docker container start -i ${DOCKER_NODE}

run-syslog: ## Run syslog-ng in docker. Logs are mounted under /tmp/syslog
	-rm -rf /tmp/syslog
	-mkdir -p /tmp/syslog
	-docker container rm syslog-ng -f
	docker run -d -p 514:514/udp  -v /tmp/syslog:/var/log  --name syslog-ng balabit/syslog-ng:latest

mod-comm: ## Comments the 'replace' rule in go.mod
	./ci_scripts/go_mod_replace.sh comment go.mod

mod-uncomm: ## Uncomments the 'replace' rule in go.mod
	./ci_scripts/go_mod_replace.sh uncomment go.mod

build-android:
	cd $$HOME && PATH=$$PATH:$$HOME/go/bin ANDROID_NDK_HOME=$$HOME/Downloads/android-ndk-r21d ANDROID_HOME=$$HOME/Library/Android/sdk gomobile bind -o ./go/src/github.com/skycoin/skywire/cmd/skywirevisormobile/android/app/skywire.aar -target=android ./go/src/github.com/skycoin/skywire/pkg/skywiremob/ && cd $$OLDPWD
	cd ./cmd/skywirevisormobile/android/ && ./gradlew assembleDebug && cd $$OLDPWD
	cd ./cmd/skywirevisormobile/android/app/build/outputs/apk/debug/ && PATH=$$PATH:$$HOME/Downloads/platform-tools adb install ./app-debug.apk && cd $$OLDPWD

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
