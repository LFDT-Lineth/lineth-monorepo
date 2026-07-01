SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata verify-testdata prepare-native-input prepare-zkc-input zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected profile-zkc-case profile-zkc clean

R5_BIN := zig-out/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ELF_TO_JSON := ../arithmetization/src/test/scripts/elf_to_json_gen/main.go
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc

EMBEDDED_SPEC ?= 0
NATIVE_BIN := zig-out/bin/verifier-ray

SERIALIZED_INPUT_FILE = testdata/inputs/verify_case_$(EMBEDDED_SPEC).bin
SERIALIZED_INVALID_INPUT_FILE = testdata/inputs/verify_case_$(EMBEDDED_SPEC)_invalid.bin
PASSING_INPUT_FILE = $(SERIALIZED_INPUT_FILE)
INPUT_FILE ?= $(PASSING_INPUT_FILE)
IN_ORIGIN ?= 0x18800000
NATIVE_INPUT_FILE := zig-out/input.bin
IN_BYTES_FILE ?= zig-out/input.hex

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
		'  build-r5              Build verifier-ray for the Linea R5 zkVM target' \
		'  run                   Run verifier-ray natively with runtime INPUT_FILE' \
		'  run-passing           Run verifier-ray natively with serialized valid input' \
		'  run-failing           Run verifier-ray natively with serialized invalid input (exits non-zero)' \
		'  run-failing-expected  Run native invalid input and require it to fail' \
		'  test                  Run Zig tests and Go codegen tests' \
		'  test-zig              Generate testdata and run Zig tests' \
		'  test-codegen          Run Go codegen tests' \
		'  generate-testdata     Generate testdata for Zig tests' \
		'  verify-testdata       Verify generated testdata is up to date with git (for CI)' \
		'  zkc-build             Build the R5 ELF and generate the zkc JSON input' \
		'  zkc-verify            Run verifier-ray through the zkc RISC-V interpreter' \
		'  zkc-verify-passing    Run zkc with serialized valid input' \
		'  zkc-verify-failing    Run zkc with serialized invalid input (exits non-zero)' \
		'  zkc-verify-failing-expected  Require zkc invalid input to fail' \
		'  profile-zkc           Profile every generated verifier case and write a compact CSV report' \
		'  clean                 Remove Zig build artifacts and serialized input files' \
		'' \
		'input variables:' \
		'  EMBEDDED_SPEC         Embedded fixture index, default 0' \
		'  INPUT_FILE            Serialized proof input path, default valid fixture for EMBEDDED_SPEC' \
		'' \
		'zkc variables:' \
		'  IN_ORIGIN            Input bytes address, default 0x18800000'

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
	zig build --release=small -Dstrip=true -Dembedded-spec=$(EMBEDDED_SPEC)

build-r5:
	zig build --release=small -Dstrip=true -Dr5=true -Dembedded-spec=$(EMBEDDED_SPEC)

run:
	@$(MAKE) --no-print-directory generate-testdata
	@$(MAKE) --no-print-directory prepare-native-input INPUT_FILE=$(INPUT_FILE)
	zig build run -Dembedded-spec=$(EMBEDDED_SPEC)

run-passing:
	$(MAKE) run INPUT_FILE=$(PASSING_INPUT_FILE)

run-failing:
	$(MAKE) run INPUT_FILE=$(SERIALIZED_INVALID_INPUT_FILE)

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

prepare-native-input:
	mkdir -p $(dir $(NATIVE_INPUT_FILE))
	cp $(INPUT_FILE) $(NATIVE_INPUT_FILE)

prepare-zkc-input:
	@# The ELF-to-JSON helper treats 0x input as a big-endian integer and reverses it
	@# before writing guest RAM, so encode binary fixtures in reverse transport order.
	mkdir -p $(dir $(IN_BYTES_FILE))
	{ printf "0x"; xxd -p -c1 $(INPUT_FILE) | awk 'BEGIN{i=0} {lines[i++]=$$0} END{for(j=i-1;j>=0;j--) print lines[j]}' | tr -d '\n'; } > $(IN_BYTES_FILE)

zkc-build: generate-testdata
	$(MAKE) --no-print-directory build-r5 EMBEDDED_SPEC=$(EMBEDDED_SPEC)
	$(MAKE) --no-print-directory prepare-zkc-input INPUT_FILE=$(INPUT_FILE) IN_BYTES_FILE=$(IN_BYTES_FILE)
	go run $(ELF_TO_JSON) $(R5_BIN) "@$(IN_BYTES_FILE)" $(IN_ORIGIN) > $(R5_JSON)

zkc-verify: zkc-build
	zkc exec -f -q $(R5_JSON) $(ZKC_MAIN)

zkc-verify-passing:
	$(MAKE) zkc-verify INPUT_FILE=$(PASSING_INPUT_FILE)

zkc-verify-failing:
	$(MAKE) zkc-verify INPUT_FILE=$(SERIALIZED_INVALID_INPUT_FILE)

zkc-verify-failing-expected:
	@if $(MAKE) zkc-verify-failing; then \
		echo "expected zkc-verify-failing to fail"; \
		exit 1; \
	else \
		echo "zkc-verify-failing failed as expected"; \
	fi

profile-zkc-case:
	@$(MAKE) --no-print-directory prepare-zkc-input INPUT_FILE=$(INPUT_FILE) IN_BYTES_FILE=$(IN_BYTES_FILE)
	@zig build --release=small -Dstrip=true -Dr5=true -Dembedded-spec=$(EMBEDDED_SPEC) -Dverifier-profiling=true -Dr5-marks=true
	@go run $(ELF_TO_JSON) $(R5_BIN) "@$(IN_BYTES_FILE)" $(IN_ORIGIN) > $(R5_JSON)
	@zkc exec -f $(R5_JSON) $(ZKC_MAIN)

profile-zkc: generate-testdata
	go run bench/verifier_profile/main.go \
		-out=bench/verifier-profile.csv

clean:
	rm -rf .zig-cache zig-out
	rm -f testdata/inputs/verify_case_*.bin
