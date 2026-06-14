SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-codegen fmt-testdata-generate fmt-bench build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected bench-vanishing-build bench-vanishing-json bench-vanishing-zkc bench-vanishing-doc bench-vanishing-compare-doc bench-vanishing-all-doc clean

R5_BIN := zig-out/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ELF_TO_JSON := ../arithmetization/src/test/examples/scripts/elf_to_json_gen/main.go
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc
VANISHING_BENCH_ZKC_MAIN := bench/riscv_main_bench.zkc
VANISHING_BENCH_BIN := zig-out/bin/verifier-ray-vanishing-bench
VANISHING_BENCH_JSON := $(VANISHING_BENCH_BIN).json
VANISHING_BENCH_LOG := zig-out/vanishing-bench.log
VANISHING_BENCH_LOG_DIR := zig-out/vanishing-bench
VANISHING_BENCH_DOC := bench/vanishing-benchmark.md
VANISHING_BENCH_COMPARE_DOC := bench/vanishing-benchmark-comparison.md
VANISHING_BENCH_CASE ?= 0
VANISHING_BENCH_CASES ?= 0-5

PASSING_INPUT_FILE := testdata/inputs/passing.bin
FAILING_INPUT_FILE := testdata/inputs/failing.bin
INPUT_FILE ?= $(PASSING_INPUT_FILE)
IN_BYTES_OFFSET ?= 0x08800000
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
		'  run                   Run verifier-ray natively with INPUT_FILE' \
		'  run-passing           Run verifier-ray natively with the passing fixture' \
		'  run-failing           Run verifier-ray natively with the failing fixture (exits non-zero)' \
		'  run-failing-expected  Run native failing fixture and require it to fail' \
		'  test                  Run Zig tests and Go codegen tests' \
		'  test-zig              Generate testdata and run Zig tests' \
		'  test-codegen          Run Go codegen tests' \
		'  generate-testdata     Generate testdata for Zig tests' \
		'  verify-testdata       Verify generated testdata is up to date with git (for CI)' \
		'  zkc-build             Build the R5 ELF and generate the zkc JSON input' \
		'  zkc-verify            Run verifier-ray through the zkc RISC-V interpreter with INPUT_FILE' \
		'  zkc-verify-passing    Run zkc with the passing fixture' \
		'  zkc-verify-failing    Run zkc with the failing fixture (exits non-zero)' \
		'  zkc-verify-failing-expected  Require zkc failing fixture to fail' \
		'  bench-vanishing-doc   Run the R5 vanishing benchmark and write bench/vanishing-benchmark.md' \
		'  bench-vanishing-compare-doc  Run selected vanishing benchmarks and write bench/vanishing-benchmark-comparison.md' \
		'  bench-vanishing-all-doc  Run all generated vanishing benchmarks and write the comparison report' \
		'  clean                 Remove Zig build artifacts' \
		'' \
		'input variables:' \
		'  INPUT_FILE            Binary input fixture for native and zkc runs, default testdata/inputs/passing.bin' \
		'  NATIVE_INPUT_FILE     Fixed native input path; must match main.zig' \
		'  IN_BYTES_FILE         Big-endian hex transport file for zkc' \
		'' \
		'zkc variables:' \
		'  IN_BYTES_OFFSET       Input bytes address, default 0x08800000' \
		'' \
		'benchmark variables:' \
		'  VANISHING_BENCH_CASE     Generated vanishing case index, default 0' \
		'  VANISHING_BENCH_CASES    Case selector for comparison, default 0-5; examples: all, 0-10, 0,2,5-8'

fmt: generate-testdata fmt-zig fmt-codegen fmt-testdata-generate fmt-bench

fmt-zig:
	zig fmt --check build.zig src test testdata/generated bench/vanishing_main.zig

fmt-codegen:
	@if [[ -n "$$(gofmt -l codegen)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l codegen; \
		exit 1; \
	fi

fmt-testdata-generate:
	@if [[ -n "$$(gofmt -l testdata/generate)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l testdata/generate; \
		exit 1; \
	fi

fmt-bench:
	@if [[ -n "$$(gofmt -l bench/render_vanishing_benchmark)" ]]; then \
		echo "please run gofmt"; \
		gofmt -l bench/render_vanishing_benchmark; \
		exit 1; \
	fi
	zkc format --check bench/riscv_main_bench.zkc

build: build-zig

build-zig:
	zig build

build-release:
	zig build --release=small -Dstrip=true

build-r5:
	zig build --release=small -Dstrip=true -Dr5=true

run:
	mkdir -p $(dir $(NATIVE_INPUT_FILE))
	cp $(INPUT_FILE) $(NATIVE_INPUT_FILE)
	zig build run

run-passing:
	$(MAKE) run INPUT_FILE=$(PASSING_INPUT_FILE)

run-failing:
	$(MAKE) run INPUT_FILE=$(FAILING_INPUT_FILE)

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

test-generated-zig: test-zig

test-codegen:
	cd codegen && go test ./...

generate-testdata:
	cd testdata/generate && go run .

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/*.zig bench/generated/*.zig

prepare-zkc-input:
	@# The ELF-to-JSON helper treats 0x input as a big-endian integer and reverses it
	@# before writing guest RAM, so encode binary fixtures in reverse transport order.
	mkdir -p $(dir $(IN_BYTES_FILE))
	{ printf "0x"; xxd -p -c1 $(INPUT_FILE) | awk 'BEGIN{i=0} {lines[i++]=$$0} END{for(j=i-1;j>=0;j--) print lines[j]}' | tr -d '\n'; } > $(IN_BYTES_FILE)

zkc-build: build-r5 prepare-zkc-input
	go run $(ELF_TO_JSON) $(R5_BIN) "@$(IN_BYTES_FILE)" $(IN_BYTES_OFFSET) > $(R5_JSON)

zkc-verify: zkc-build
	zkc exec -q $(R5_JSON) $(ZKC_MAIN)

zkc-verify-passing:
	$(MAKE) zkc-verify INPUT_FILE=$(PASSING_INPUT_FILE)

zkc-verify-failing:
	$(MAKE) zkc-verify INPUT_FILE=$(FAILING_INPUT_FILE)

zkc-verify-failing-expected:
	@if $(MAKE) zkc-verify-failing; then \
		echo "expected zkc-verify-failing to fail"; \
		exit 1; \
	else \
		echo "zkc-verify-failing failed as expected"; \
	fi

bench-vanishing-build: generate-testdata
	zig build --release=small -Dstrip=true -Dr5=true -Dvanishing-bench=true -Dvanishing-case=$(VANISHING_BENCH_CASE)

bench-vanishing-json: bench-vanishing-build
	go run $(ELF_TO_JSON) $(VANISHING_BENCH_BIN) "" $(IN_BYTES_OFFSET) > $(VANISHING_BENCH_JSON)

bench-vanishing-zkc: bench-vanishing-json
	mkdir -p $(dir $(VANISHING_BENCH_LOG))
	set -o pipefail; zkc exec -q $(VANISHING_BENCH_JSON) $(VANISHING_BENCH_ZKC_MAIN) | tee $(VANISHING_BENCH_LOG)

bench-vanishing-doc: bench-vanishing-zkc
	go run bench/render_vanishing_benchmark/main.go \
		-log $(VANISHING_BENCH_LOG) \
		-out $(VANISHING_BENCH_DOC) \
		-metadata bench/generated/vanishing.zig \
		-bin $(VANISHING_BENCH_BIN) \
		-json $(VANISHING_BENCH_JSON) \
		-case-index $(VANISHING_BENCH_CASE) \
		-zkc-main $(VANISHING_BENCH_ZKC_MAIN)

bench-vanishing-compare-doc: generate-testdata
	mkdir -p $(VANISHING_BENCH_LOG_DIR)
	rm -f $(VANISHING_BENCH_LOG_DIR)/case-*.log
	set -e; set -o pipefail; \
	cases="$$(go run bench/render_vanishing_benchmark/main.go \
		-metadata bench/generated/vanishing.zig \
		-case-spec "$(VANISHING_BENCH_CASES)" \
		-print-cases)"; \
	echo "vanishing benchmark cases: $$cases"; \
	for case in $$cases; do \
		echo "benchmarking vanishing case $$case"; \
		zig build --release=small -Dstrip=true -Dr5=true -Dvanishing-bench=true -Dvanishing-case=$$case; \
		go run $(ELF_TO_JSON) $(VANISHING_BENCH_BIN) "" $(IN_BYTES_OFFSET) > $(VANISHING_BENCH_JSON); \
		zkc exec -q $(VANISHING_BENCH_JSON) $(VANISHING_BENCH_ZKC_MAIN) | tee $(VANISHING_BENCH_LOG_DIR)/case-$$case.log; \
	done; \
	go run bench/render_vanishing_benchmark/main.go \
		-log-dir $(VANISHING_BENCH_LOG_DIR) \
		-out $(VANISHING_BENCH_COMPARE_DOC) \
		-metadata bench/generated/vanishing.zig \
		-case-spec "$(VANISHING_BENCH_CASES)" \
		-zkc-main $(VANISHING_BENCH_ZKC_MAIN)

bench-vanishing-all-doc:
	$(MAKE) bench-vanishing-compare-doc VANISHING_BENCH_CASES=all

clean:
	rm -rf .zig-cache zig-out
