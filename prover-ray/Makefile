# The go-corset module name
ZKC_MOD := github.com/LFDT-Lineth/zkc
# The version of go-corset as presented in the go.mod
ZKC_VERSION := $(shell go list -m -f '{{.Version}}' $(ZKC_MOD))
# The go path, usually $HOME/go
GOPATH := $(shell go env GOPATH)
# The directory where to find the zkc testdata
ZKC_TESTDATA_SYNC_SRC_DIR := $(shell go list -m -f '{{.Dir}}' $(ZKC_MOD))/testdata/zkc
ZKC_TESTDATA_SYNC_DST_DIR := zkcdriver/testdata/synced

# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	test \
	ci-lint \
	download-zkc-testdata

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
	# cleanup the destination directory, and create it if it doesn't exist
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/*
	mkdir -p $(ZKC_TESTDATA_SYNC_DST_DIR)

	# go get zkc, then we can copy the testdata from the module's directory
	go get $(ZKC_MOD)@$(ZKC_VERSION)
	cp -rf $(ZKC_TESTDATA_SYNC_SRC_DIR)/* $(ZKC_TESTDATA_SYNC_DST_DIR)
	chmod -R u+w $(ZKC_TESTDATA_SYNC_DST_DIR)

	# we don't care about invalid test-cases, it only matters that the valid ones are provable
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/invalid
	find $(ZKC_TESTDATA_SYNC_DST_DIR) -type f -name "*.rejects" -delete

	# we don't compile binaries. They are compiled as-needed during integration tests (to be up-to-date)