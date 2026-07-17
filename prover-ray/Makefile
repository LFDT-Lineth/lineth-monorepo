# The version of zkc as presented in the go.mod
ZKC_VERSION := $(shell grep 'zkc' go.mod | awk '/zkc/{print $$2}')
# The zkc module name
ZKC_MOD := github.com/LFDT-Lineth/zkc
# The zkc installed package name
ZKC_PKG := "github.com/!l!f!d!t-!lineth/zkc"
# The go path, usually $HOME/go
GOPATH := $(shell go env GOPATH)
# The directory where to find the zkc testdata
ZKC_TESTDATA_SYNC_SRC_DIR := $(GOPATH)/pkg/mod/$(ZKC_PKG)@$(ZKC_VERSION)/testdata/zkc
ZKC_TESTDATA_SYNC_DST_DIR := zkcdriver/testdata/synced
ZKC_BIN := $(GOPATH)/bin/zkc
#
ZKC_IGNORES := $(ZKC_TESTDATA_SYNC_DST_DIR)/unit/fixed_array_10.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/unit/fixed_array_11.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/unit/fixed_array_12.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/keccakf_with_padding.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/test/felt/poseidon_zeros_big_01.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/test/felt/poseidon_range_01.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/test/u32/poseidon_zeros_big_01.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/test/u32/poseidon_range_01.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/felt/poseidon.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/felt/memory.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/u32/poseidon.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/bench/poseidon/u32/memory.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/util/fill_bytes.zkc \
	$(ZKC_TESTDATA_SYNC_DST_DIR)/unit/basic_14.zkc

# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	test \
	ci-lint \
	download-zkc-testdata \
	go-install-zkc-compiler \
	check-zkc-testdata

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
## Install the zkc compiler
##
go-install-zkc-compiler:
	go install $(ZKC_MOD)/cmd/zkc@$(ZKC_VERSION)

##
## Check that the committed zkc testdata is up to date (for CI)
##
check-zkc-testdata: download-zkc-testdata
	git diff --exit-code $(ZKC_TESTDATA_SYNC_DST_DIR)/

##
## Download the zkc testdata
##
download-zkc-testdata: go-install-zkc-compiler
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/*
	mkdir -p $(ZKC_TESTDATA_SYNC_DST_DIR)
	go get $(ZKC_MOD)@$(ZKC_VERSION)
	cp -rf $(ZKC_TESTDATA_SYNC_SRC_DIR)/* $(ZKC_TESTDATA_SYNC_DST_DIR)
	chmod -R u+w $(ZKC_TESTDATA_SYNC_DST_DIR)
	rm -rf $(ZKC_TESTDATA_SYNC_DST_DIR)/invalid
	rm -rf $(ZKC_IGNORES)
	find $(ZKC_TESTDATA_SYNC_DST_DIR) -name "*.zkc" | while read f; do \
		$(ZKC_BIN) compile -o $${f%.zkc}.bin $$f > /dev/null 2>&1 || echo "error compiling $$f"; \
	done
