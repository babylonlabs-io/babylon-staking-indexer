TOOLS_DIR := tools
PACKAGES_E2E=$(shell go list ./... | grep '/e2etest')
BUILDDIR ?= $(CURDIR)/build

ldflags := $(LDFLAGS)
build_tags := $(BUILD_TAGS)
build_args := $(BUILD_ARGS)

ifeq ($(VERBOSE),true)
	build_args += -v
endif

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static" -v
endif

BUILD_TARGETS := build install
BUILD_FLAGS := --tags "$(build_tags)" --ldflags '$(ldflags)'

all: build install

build: BUILD_ARGS := $(build_args) -o $(BUILDDIR)

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	go $@ -mod=readonly $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

.PHONY: build install tests

build-docker:
	$(MAKE) BBN_PRIV_DEPLOY_KEY=${BBN_PRIV_DEPLOY_KEY} -C contrib/images babylon-staking-indexer

start-babylon-staking-indexer: build-docker stop-service
	docker compose up -d

stop-service:
	docker compose down
	
run-local:
	./bin/local-startup.sh;
	go run cmd/babylon-staking-indexer/main.go --config config/config-local.yml

generate:
	go generate ./...

test:
	./bin/local-startup.sh;
	go test -v -cover ./...

test-integration:
	go test -tags integration -v ./internal/... # todo(Kirill) change from internal to root

test-e2e:
	./bin/local-startup.sh;
	go test -mod=readonly -timeout=25m -v $(PACKAGES_E2E) -count=1 --tags=e2e;
	docker compose down
