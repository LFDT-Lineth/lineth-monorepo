# required for `source`
SHELL := /usr/bin/env bash
CORSET_ROOT := $(shell pwd)/../go-corset

VERSION := $(shell git rev-parse --short HEAD)

LINUX_AMD64_FLAGS := CGO_ENABLED=1 CC="x86_64-linux-musl-gcc" CXX="x86_64-linux-musl-g++" GOOS="linux" GOARCH="amd64"
DARWIN_ARM64_FLAGS := CGO_ENABLED=1 GOOS="linux" GOARCH="arm64"

# Ensure that we are using go-corset from the monorepo
GO_CORSET := PATH="${CORSET_ROOT}/bin":$$PATH

# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	copy-prover-assets \
	download-srs \
	bin/proofless \
	bin/compression-aggregation-sample \
	bin/state-manager-inspector \
	zkevm/arithmetization/zkevm.bin \
	lib/compressor \
	lib/shnarf-calculator \
	lib/compressor-and-shnarf-calculator \
	lib/compressor-and-shnarf-calculator-local \
	docker \
	bin/prover \
	bin/checker \
	go-corset \
	testdata \


##
## This rule produces the go-corset binary
##
go-corset:
	cd ${CORSET_ROOT} && make build

##
## Build and bundle the go-corset trace-expander dependency
##
zkevm/arithmetization/zkevm.bin: go-corset
	cd ../constraints && $(GO_CORSET) make zkevm.bin && mv zkevm.bin ../prover/zkevm/arithmetization

##
## zkevm.bin without 2B block gas limit constraint to test old traces
##
zkevm/arithmetization/zkevm_for_old_trace.bin: go-corset
	cd ../constraints && $(GO_CORSET) make zkevm_for_old_replay_tests.bin &&\
	mv zkevm_for_old_replay_tests.bin ../prover/zkevm/arithmetization/zkevm_for_old_trace.bin

##
## Generate the setup for the execution prover (to be run with S3 access)
##
setup: bin/prover
	bin/prover setup --config ./config/config-sepolia-full.toml --dict ./lib/compressor/compressor_dict.bin --assets-dir ./prover-assets

##
## Copy the prover assets to the S3 bucket (zkuat)
##
copy-prover-assets:
	aws s3 sync --exclude "*prover/dev*" --exclude "*05b9ef1*" --exclude "*05b9ef1*" --exclude "*96e3a19*" --exclude "*a9e4681*" prover-assets s3://zk-uat-prover/prover-assets/ --profile=zk-uat-s3-access

download-srs:
	aws s3 sync s3://gnark-ignition/with_lines/ ./prover-assets/kzgsrs --exclude "*" --include "*.memdump"

###
### Controller
###
bin/controller:
	mkdir -p bin/
	rm -f $@
	go build -o $@ ./cmd/controller

##
##	Prover (setup and prove)
##
bin/prover:
	mkdir -p bin
	rm -f $@
	go build -tags debug -o $@ ./cmd/prover

##
##	Compiles the state-manager inspector
##
bin/state-manager-inspector:
	mkdir -p bin
	rm -f $@
	go build -o ./$@ ./cmd/dev-tools/state-manager-inspector

##
## Generate the sample generator for the compression and the aggregation
##
bin/compression-aggregation-sample:
	mkdir -p bin
	rm -f $@
	go build -o ./$@ -tags nocorset ./cmd/dev-tools/testcase-gen/compression-aggregation

##
##	corset-checker
##
bin/checker:
	mkdir -p bin
	rm -f $@
	go build -o $@ ./cmd/dev-tools/corset-checker

##
##	Build the prover docker image
##
docker:
	echo "Building docker image for the prover version ${VERSION}"
	cd .. && docker buildx build -f prover/Dockerfile --build-context prover=prover/ --build-context corset=corset/ --build-context constraints=constraints/ -t consensys/linea-prover .

##
## Build compressor library
##
lib/compressor:
	CGO_ENABLED=1 go build -tags=nocorset -buildmode=c-shared -o lib/compressor/build/libblob_compressor_native_jna ./lib/compressor/blob_compressor.go

##
## Build the shnarf calculator library
##
lib/shnarf-calculator:
	CGO_ENABLED=1 go build -tags=nocorset -buildmode=c-shared -o lib/shnarf_calculator/build/libshnarf_calculator_native_jna ./lib/shnarf_calculator/shnarf_calculator.go

##
## Build the compressor and the shnarf-calculator for local system
##
lib/compressor-and-shnarf-calculator-local: lib/compressor lib/shnarf_calculator

##
## Run all the unit-tests
##
test:
	go test ./...

##
## Run the CI linting
##
ci-lint:
	golangci-lint run --timeout 5m

##
## Echo, the CGO flags. Usefull for testing manually
##
cgo-flags:
	@echo $(CORSET_FLAGS)

#
# Update the prover's testdata in ../testdata/prover
#
make update-testdata-for-coordinator:
	go test -timeout 30m -tags amd64,nocorset -run TestSamples github.com/consensys/linea-monorepo/prover/backend/testing -v
	cp -f backend/testing/**/*-*-zkProof.json ../testdata/prover/output
