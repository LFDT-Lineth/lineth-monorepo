# The zkc module name
ZKC_MOD := github.com/LFDT-Lineth/zkc
# The directory where to find the zkc testdata
ZKC_MOD_DIR := $(shell go list -m -f '{{.Dir}}' $(ZKC_MOD))
ZKC_TESTDATA_SYNC_SRC_DIR := $(ZKC_MOD_DIR)/testdata/zkc
ZKC_TESTDATA_SYNC_DST_DIR := zkcdriver/testdata/synced

# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	test \
	ci-lint \
	download-zkc-testdata \
	docker-build \
	download-guest \
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
download-zkc-testdata:
	# download dependencies first, so we can resolve and validate the module's directory
	go mod download $(ZKC_MOD)
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

# Published guest artifact, independently versioned from the prover image.
L2_EXECUTION_GUEST_RELEASE ?= releases/riscv-l2-execution/v0.1.0-debug
L2_EXECUTION_GUEST_ASSET ?= l2-execution-v0.1.0-debug-ace669155c1f5492b26d4ae816b032338a8467323c668ea96ac288307115be1a.elf
L2_EXECUTION_GUEST_SHA256 ?= 62e8c9b1cd2d5c0fa4de2974792740f7ccd633175fa2e398756896795385a432
GUEST_ASSET_DIR := $(REPO_ROOT)/prover-ray/bin/guest

# Uses local gh authentication or GH_TOKEN in CI; credentials never enter Docker.
download-guest:
	@set -eu; \
	mkdir -p "$(GUEST_ASSET_DIR)"; \
	asset="$(GUEST_ASSET_DIR)/evm_execution_guest"; \
	if [ -f "$$asset" ] && printf '%s  %s\n' "$(L2_EXECUTION_GUEST_SHA256)" "$$asset" | shasum -a 256 -c - >/dev/null 2>&1; then \
		echo "Using verified guest ELF: $(L2_EXECUTION_GUEST_RELEASE)"; exit 0; \
	fi; \
	tmp=$$(mktemp "$(GUEST_ASSET_DIR)/download.XXXXXX"); \
	trap 'rm -f "$$tmp"' EXIT; \
	gh release download "$(L2_EXECUTION_GUEST_RELEASE)" --repo LFDT-Lineth/lineth-monorepo \
		--pattern "$(L2_EXECUTION_GUEST_ASSET)" --output "$$tmp" --clobber; \
	printf '%s  %s\n' "$(L2_EXECUTION_GUEST_SHA256)" "$$tmp" | shasum -a 256 -c -; \
	mv "$$tmp" "$$asset"

##
## Build the prover image with a released guest ELF and a locally built native runner.
##
docker-build: download-guest
	$(DOCKER) build -f Dockerfile \
		--build-context riscv-guests=$(REPO_ROOT)/riscv-guests \
		--build-context l2-execution-guest=$(GUEST_ASSET_DIR) \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		.

##
## Smoke-test the built image: dev-mock request->response + dev-zkvm artifacts.
##
docker-smoke:
	DOCKER="$(DOCKER)" ./scripts/docker-smoke.sh $(IMAGE_NAME):$(IMAGE_TAG)