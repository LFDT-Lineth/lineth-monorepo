SHELL := /bin/bash

.PHONY: ci ci-checks help fmt fmt-zig fmt-codegen fmt-testdata-generator fmt-profile build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected profile-zkc profile-zkc-all clean

R5_BIN := zig-out/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ELF_TO_JSON := ../arithmetization/src/test/examples/scripts/elf_to_json_gen/main.go
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc

# Input is currently embedded into the binary at build time (we don't have input
# serialization yet), selected via `-Dembedded-input=<valid|invalid>` and a fixture
# index `-Dembedded-spec=<N>`. The passing.bin/failing.bin fixtures and the
# file/mmap input-parsing path below are kept for later reintroduction once
# serialization lands, but are no longer used by the run/zkc targets.
EMBEDDED_INPUT ?= valid
EMBEDDED_SPEC ?= 0
PROFILE_CASES ?= 0
PROFILE_INPUT ?= valid
PROFILE_MODE ?= profiled
PROFILE_OUT ?= bench/verifier-profile.md
PROFILE_TOP_INSTRUCTIONS ?= 10
NATIVE_BIN := zig-out/bin/verifier-ray

# Legacy file-based input plumbing (unused for now, kept for reintroduction).
PASSING_INPUT_FILE := testdata/inputs/passing.bin
FAILING_INPUT_FILE := testdata/inputs/failing.bin
INPUT_FILE ?= $(PASSING_INPUT_FILE)
IN_ORIGIN ?= 0x08800000
NATIVE_INPUT_FILE := zig-out/input.bin
IN_BYTES_FILE ?= zig-out/input.hex

ci: generate-testdata ci-checks

ci-checks: fmt build-release run-passing run-failing-expected test zkc-verify-passing zkc-verify-failing-expected

help:
	@printf '%s\n' \
		'Usage: make <target>' \
		'' \
		'Common targets:' \
		'  help                  Show this help message' \
		'  ci                    Run fmt, release build, tests, and native/zkc smoke checks' \
		'  fmt                   Run Zig and Go formatting checks' \
		'  build                 Build verifier-ray natively' \
		'  build-release         Build verifier-ray natively with ReleaseSmall and stripped symbols' \
		'  build-r5              Build verifier-ray for the Linea R5 zkVM target' \
		'  run                   Run verifier-ray natively with embedded EMBEDDED_INPUT' \
		'  run-passing           Run verifier-ray natively with embedded valid input' \
		'  run-failing           Run verifier-ray natively with embedded invalid input (exits non-zero)' \
		'  run-failing-expected  Run native invalid input and require it to fail' \
		'  test                  Run Zig tests and Go codegen tests' \
		'  test-zig              Generate testdata and run Zig tests' \
		'  test-codegen          Run Go codegen tests' \
		'  generate-testdata     Generate testdata for Zig tests' \
		'  verify-testdata       Verify generated testdata is up to date with git (for CI)' \
		'  zkc-build             Build the R5 ELF and generate the zkc JSON input' \
		'  zkc-verify            Run verifier-ray through the zkc RISC-V interpreter with embedded EMBEDDED_INPUT' \
		'  zkc-verify-passing    Run zkc with embedded valid input' \
		'  zkc-verify-failing    Run zkc with embedded invalid input (exits non-zero)' \
		'  zkc-verify-failing-expected  Require zkc invalid input to fail' \
		'  profile-zkc           Profile verifier-ray through zkc and write a compact markdown report' \
		'  profile-zkc-all       Profile every generated verifier case' \
		'  clean                 Remove Zig build artifacts' \
		'' \
		'input variables:' \
		'  EMBEDDED_INPUT        Embedded input fixture kind: valid or invalid, default valid' \
		'  EMBEDDED_SPEC         Embedded fixture index, default 0' \
		'  PROFILE_CASES         Profiling selector: all, N, A-B, or comma-separated selectors, default 0' \
		'  PROFILE_INPUT         Profiling input fixture kind: valid or invalid, default valid' \
		'  PROFILE_MODE          Profiling mode: raw, profiled, or both, default profiled' \
		'  PROFILE_OUT           Profiling markdown output path, default bench/verifier-profile.md' \
		'  PROFILE_TOP_INSTRUCTIONS  Number of top instructions in the report, default 10' \
		'' \
		'zkc variables:' \
		'  IN_ORIGIN            Input bytes address, default 0x08800000'

fmt: fmt-zig fmt-codegen fmt-testdata-generator fmt-profile

fmt-zig:
	zig fmt --check build.zig src test testdata/generated

fmt-codegen:
	@if [[ -n "$$(gofmt -l codegen)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l codegen; \
		exit 1; \
	fi

fmt-testdata-generator:
	@if [[ -n "$$(gofmt -l testdata/generate)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l testdata/generate; \
		exit 1; \
	fi

fmt-profile:
	@if [[ -n "$$(gofmt -l bench)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l bench; \
		exit 1; \
	fi

build: build-zig

build-zig:
	zig build

build-release:
	zig build --release=small -Dstrip=true -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC)

build-r5:
	zig build --release=small -Dstrip=true -Dr5=true -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC)

run:
	zig build run -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC)

run-passing:
	$(MAKE) run EMBEDDED_INPUT=valid

run-failing:
	$(MAKE) run EMBEDDED_INPUT=invalid

run-failing-expected:
	@if $(MAKE) run-failing; then \
		echo "expected run-failing to fail"; \
		exit 1; \
	else \
		echo "run-failing failed as expected"; \
	fi

test: test-zig test-codegen

test-zig: generate-testdata
	zig build test
	zig build -Dverifier-profiling=true test-profiling

test-generated-zig: test-zig

test-codegen:
	cd codegen && go test ./...

generate-testdata:
	cd testdata/generate && go run .

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/*.zig

# Legacy file-based input transport, unused while inputs are embedded; kept for
# reintroduction once input serialization lands.
prepare-zkc-input:
	@# The ELF-to-JSON helper treats 0x input as a big-endian integer and reverses it
	@# before writing guest RAM, so encode binary fixtures in reverse transport order.
	mkdir -p $(dir $(IN_BYTES_FILE))
	{ printf "0x"; xxd -p -c1 $(INPUT_FILE) | awk 'BEGIN{i=0} {lines[i++]=$$0} END{for(j=i-1;j>=0;j--) print lines[j]}' | tr -d '\n'; } > $(IN_BYTES_FILE)

# The input is embedded in the binary, so the guest never reads the IN region;
# elf_to_json still requires an input argument, so pass a one-byte placeholder.
zkc-build: build-r5
	go run $(ELF_TO_JSON) $(R5_BIN) "0x00" $(IN_ORIGIN) > $(R5_JSON)

zkc-verify: zkc-build
	zkc exec -q $(R5_JSON) $(ZKC_MAIN)

zkc-verify-passing:
	$(MAKE) zkc-verify EMBEDDED_INPUT=valid

zkc-verify-failing:
	$(MAKE) zkc-verify EMBEDDED_INPUT=invalid

zkc-verify-failing-expected:
	@if $(MAKE) zkc-verify-failing; then \
		echo "expected zkc-verify-failing to fail"; \
		exit 1; \
	else \
		echo "zkc-verify-failing failed as expected"; \
	fi

profile-zkc:
	go run bench/verifier_profile/main.go \
		-cases="$(PROFILE_CASES)" \
		-input="$(PROFILE_INPUT)" \
		-mode="$(PROFILE_MODE)" \
		-out="$(PROFILE_OUT)" \
		-top-instructions="$(PROFILE_TOP_INSTRUCTIONS)"

profile-zkc-all:
	$(MAKE) profile-zkc PROFILE_CASES=all

clean:
	rm -rf .zig-cache zig-out
