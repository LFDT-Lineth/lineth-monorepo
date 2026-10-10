# The zkc module name
ZKC_MOD := github.com/LFDT-Lineth/zkc
# The directory where to find the zkc testdata. Deferred (=, not :=): on a cold
# module cache it is empty until download-zkc-mod has run.
ZKC_MOD_DIR = $(shell go list -m -f '{{.Dir}}' $(ZKC_MOD))
ZKC_TESTDATA_SYNC_SRC_DIR = $(ZKC_MOD_DIR)/testdata/zkc
ZKC_TESTDATA_SYNC_DST_DIR := zkcdriver/testdata/synced

# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	test \
	ci-lint \
	download-zkc-mod \
	download-zkc-testdata \
	docker-build \
	docker-smoke

# One prover-ray image for all modes; the mode is set per pipeline in the config.
# DOCKER overridable for rootful/rootless or podman, e.g. DOCKER='sudo docker'.
DOCKER     ?= docker
IMAGE_NAME ?= consensys/lineth-prover-ray
IMAGE_TAG  ?= dev
REPO_ROOT  := $(shell git rev-parse --show-toplevel)

##
## Run all the unit-tests
##
test: ci-lint 
	go test -tags debug ./...

##
## Run the CI linting
##
ci-lint:
	go fmt ./...	
	golangci-lint run --timeout 5m

##
## Download the zkc testdata
##
# Download the module in its own target: make expands a recipe before running
# it, so ZKC_MOD_DIR must only be expanded once the download has completed.
download-zkc-mod:
	go mod download $(ZKC_MOD)

download-zkc-testdata: download-zkc-mod
	@test -n "$(ZKC_MOD_DIR)" || (echo "error: could not resolve directory of module $(ZKC_MOD)" && exit 1)

	# cleanup the destination directory, and create it if it doesn't exist
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/*
	mkdir -p $(ZKC_TESTDATA_SYNC_DST_DIR)

	cp -rf $(ZKC_TESTDATA_SYNC_SRC_DIR)/* $(ZKC_TESTDATA_SYNC_DST_DIR)
	chmod -R u+w $(ZKC_TESTDATA_SYNC_DST_DIR)

	# we don't care about invalid test-cases, it only matters that the valid ones are provable
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/invalid
	find $(ZKC_TESTDATA_SYNC_DST_DIR) -type f -name "*.rejects" -delete

	# we don't compile binaries. They are compiled as-needed during integration tests (to be up-to-date)

##
## Build the one prover-ray image (Go prover + native runner + guest ELF).
## The native runner is built from riscv-guests, passed in as a build context.
##
docker-build:
	$(DOCKER) build -f Dockerfile \
		--build-context riscv-guests=$(REPO_ROOT)/riscv-guests \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		.

##
## Smoke-test the built image: dev-mock request->response + dev-zkvm artifacts.
##
docker-smoke:
	DOCKER="$(DOCKER)" ./scripts/docker-smoke.sh $(IMAGE_NAME):$(IMAGE_TAG)