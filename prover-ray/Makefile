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
	download-instrumentation-testdata

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

##
## Populate instrumentation/testdata/shards/ from the already-synced zkcdriver
## testdata.  Each .zkc file is a fixture; its paired .accepts file holds the
## inputs (one JSON object per line, copied as-is from the zkcdriver testdata).
##
INSTRUMENTATION_TESTDATA_DST := instrumentation/testdata/shards
INSTRUMENTATION_TESTDATA_SRC := $(ZKC_TESTDATA_SYNC_DST_DIR)/unit
download-instrumentation-testdata: download-zkc-testdata
	rm -rf $(INSTRUMENTATION_TESTDATA_DST)/*.zkc $(INSTRUMENTATION_TESTDATA_DST)/*.accepts
	mkdir -p $(INSTRUMENTATION_TESTDATA_DST)
	cp $(INSTRUMENTATION_TESTDATA_SRC)/*.zkc $(INSTRUMENTATION_TESTDATA_DST)/
	cp $(INSTRUMENTATION_TESTDATA_SRC)/*.accepts $(INSTRUMENTATION_TESTDATA_DST)/