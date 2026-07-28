SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected profile-zkc-case profile-zkc clean

# `make` with no target runs first goal. Set explicitly here to avoid include overriding the default goal.
.DEFAULT_GOAL := ci

# shared guest build tools (elf-to-json, linker script, IN_ORIGIN)
include ../riscv-guests/build_common/Makefile

R5_BIN := zig-out/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc

# Input is currently embedded into the binary at build time (we don't have input
# serialization yet), selected via `-Dembedded-input=<valid|invalid>` and a fixture
# index `-Dembedded-spec=<N>`. The passing.bin/failing.bin fixtures and the
# file/mmap input-parsing path below are kept for later reintroduction once
# serialization lands, but are no longer used by the run/zkc targets.
EMBEDDED_INPUT ?= valid
EMBEDDED_SPEC ?= 0
NATIVE_BIN := zig-out/bin/verifier-ray

# elf-to-json expects a big-endian hex string or `@path/to/in_bytes` for the input bytes. We currently
# embed the input in the binary so we pass one byte of placeholder. But the guest right now never reads.
IN_BYTES := 0x00

# Legacy file-based input plumbing (unused for now, kept for reintroduction).
PASSING_INPUT_FILE := testdata/inputs/passing.bin
FAILING_INPUT_FILE := testdata/inputs/failing.bin
INPUT_FILE ?= $(PASSING_INPUT_FILE)
NATIVE_INPUT_FILE := zig-out/input.bin
IN_BYTES_FILE ?= zig-out/input.hex

DISABLE_ACCELERATORS ?= false

ci: fmt build-release run-passing run-failing-expected test zkc-verify-passing zkc-verify-failing-expected

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
		'  build-r5              Build verifier-ray for the Lineth R5 zkVM target' \
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
		'  profile-zkc           Profile every generated verifier case and write a compact CSV report' \
		'  clean                 Remove Zig build artifacts' \
		'' \
		'input variables:' \
		'  EMBEDDED_INPUT        Embedded input fixture kind: valid or invalid, default valid' \
		'  EMBEDDED_SPEC         Embedded fixture index, default 0' \
		'' \
		'zkc variables:' \
		'  IN_ORIGIN             Input bytes address, default 0x08800000' \
		'' \
		'R5 build variables:' \
		'  DISABLE_ACCELERATORS  Disable Lineth zkVM accelerator wrappers (for compatibility testing), default false' \

fmt: fmt-zig fmt-go

fmt-zig:
	zig fmt --check build.zig src test testdata/generated

fmt-go:
	@if [[ -n "$$(gofmt -l .)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l .; \
		exit 1; \
	fi

build: build-zig

build-zig:
	zig build

build-release:
	zig build --release=small -Dstrip=true -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC)

build-r5:
	zig build --release=small -Dstrip=true -Dr5=true -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC) -Ddisable-accelerators=$(DISABLE_ACCELERATORS)

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
	cd testdata/generate/fri && go run .
	cd ../prover-ray/crypto/koalabear/fri && go test -tags gen_vectors -run '^TestGenerateVectors$$' .

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/*.zig testdata/generated/*.json

# Legacy file-based input transport, unused while inputs are embedded; kept for
# reintroduction once input serialization lands.
prepare-zkc-input:
	@# The ELF-to-JSON helper treats 0x input as a big-endian integer and reverses it
	@# before writing guest RAM, so encode binary fixtures in reverse transport order.
	mkdir -p $(dir $(IN_BYTES_FILE))
	{ printf "0x"; xxd -p -c1 $(INPUT_FILE) | awk 'BEGIN{i=0} {lines[i++]=$$0} END{for(j=i-1;j>=0;j--) print lines[j]}' | tr -d '\n'; } > $(IN_BYTES_FILE)

zkc-build: build-r5
	$(call ELF_TO_JSON,$(R5_BIN),$(R5_JSON))

zkc-verify: zkc-build
	zkc exec -f -q $(R5_JSON) $(ZKC_MAIN)

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

profile-zkc-case:
	@zig build --release=small -Dstrip=true -Dr5=true -Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC) -Dverifier-profiling=true -Dr5-marks=true
	$(call ELF_TO_JSON,$(R5_BIN),$(R5_JSON))
	@zkc exec -f $(R5_JSON) $(ZKC_MAIN)

profile-zkc: generate-testdata
	go run bench/verifier_profile/main.go \
		-out=bench/verifier-profile.csv

clean:
	rm -rf .zig-cache zig-out
