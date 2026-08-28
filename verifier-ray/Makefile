SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected profile-zkc-case profile-zkc clean

# `make` with no target runs first goal. Set explicitly here to avoid include overriding the default goal.
.DEFAULT_GOAL := ci

# shared guest build tools (elf-to-json, linker script, IN_ORIGIN)
include ../riscv-guests/build_common/Makefile

R5_BIN := zig-out/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc
PROOF_IMAGE := testdata/riscv_proof_image.bin

# Optional legacy embedded-fixture mode, still used by the profiling targets.
# The default smoke-test run/zkc path loads the committed real proof image.
EMBEDDED_INPUT ?= valid
EMBEDDED_SPEC ?= 0
NATIVE_BIN := zig-out/bin/verifier-ray

# elf-to-json accepts @path for raw bytes placed directly at `_in_start`.
IN_BYTES := @$(PROOF_IMAGE)

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
		'  run                   Run verifier-ray natively against testdata/riscv_proof_image.bin' \
		'  run-passing           Alias for run' \
		'  run-failing           Run verifier-ray natively with an invalid embedded fixture (exits non-zero)' \
		'  run-failing-expected  Run native invalid input and require it to fail' \
		'  test                  Run Zig tests and Go codegen tests' \
		'  test-zig              Generate testdata and run Zig tests' \
		'  test-codegen          Run Go codegen tests' \
		'  generate-testdata     Generate testdata for Zig tests' \
		'  verify-testdata       Verify generated testdata is up to date with git (for CI)' \
		'  zkc-build             Build the R5 ELF and generate the zkc JSON input' \
		'  zkc-verify            Run verifier-ray through the zkc RISC-V interpreter against testdata/riscv_proof_image.bin' \
		'  zkc-verify-passing    Alias for zkc-verify' \
		'  zkc-verify-failing    Run zkc with an invalid embedded fixture (exits non-zero)' \
		'  zkc-verify-failing-expected  Require zkc invalid input to fail' \
		'  profile-zkc           Profile every generated verifier case and write a compact CSV report' \
		'  clean                 Remove Zig build artifacts' \
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
	zig build --release=small -Dstrip=true

build-r5:
	zig build --release=small -Dstrip=true -Dr5=true -Ddisable-accelerators=$(DISABLE_ACCELERATORS)

run:
	zig build run

run-passing:
	$(MAKE) run

run-failing:
	zig build run -Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC)

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
	cd codegen/generate-riscv-system && go run .

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/*.zig testdata/riscv_proof_image.bin

zkc-build: build-r5
	$(call ELF_TO_JSON,$(R5_BIN),$(R5_JSON))

zkc-verify: zkc-build
	zkc exec $(R5_JSON) $(ZKC_MAIN)

zkc-verify-passing:
	$(MAKE) zkc-verify

zkc-verify-failing:
	zig build --release=small -Dstrip=true -Dr5=true -Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC) -Ddisable-accelerators=$(DISABLE_ACCELERATORS)
	$(call ELF_TO_JSON,$(R5_BIN),$(R5_JSON))
	zkc exec $(R5_JSON) $(ZKC_MAIN)

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
