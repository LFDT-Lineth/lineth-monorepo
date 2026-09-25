SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-generated-zig test-codegen generate-testdata generate-large-pcs-benchmark profile-large-pcs verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected profile-zkc-case profile-zkc clean

RISCV_SYSTEM ?= testdata/generated/riscv_system.zig
ZIG_OUT ?= zig-out
ZIG_BUILD = zig build \
	--prefix "$(abspath $(ZIG_OUT))" \
	-Driscv-system="$(abspath $(RISCV_SYSTEM))"

R5_BIN := $(ZIG_OUT)/bin/verifier-ray
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

# Use the zkc tool version pinned in go.mod
ZKC = go tool zkc
# Use pinned elf_to_json in go.mod
ELF_TO_JSON = go tool elf_to_json

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
		'  generate-large-pcs-benchmark  Generate the ignored 2^19 PCS benchmark fixture' \
		'  profile-large-pcs      Run the standalone PCS verifier under zkc and report RISC-V cycles' \
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
		'  PROOF_IMAGE          Proof paired with RISCV_SYSTEM, default testdata/riscv_proof_image.bin' \
		'' \
		'Zig build variables:' \
		'  RISCV_SYSTEM         Generated verifier system source, default testdata/generated/riscv_system.zig' \
		'  ZIG_OUT              Per-build output prefix, default zig-out' \
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
	$(ZIG_BUILD)

build-release:
	$(ZIG_BUILD) --release=small -Dstrip=true

build-r5:
	$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true -Ddisable-accelerators=$(DISABLE_ACCELERATORS)

run:
	$(ZIG_BUILD) run

run-passing:
	$(MAKE) run

run-failing:
	$(ZIG_BUILD) run -Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC)

run-failing-expected:
	@if $(MAKE) run-failing; then \
		echo "expected run-failing to fail"; \
		exit 1; \
	else \
		echo "run-failing failed as expected"; \
	fi

test: test-zig test-codegen

test-zig: generate-testdata
	$(ZIG_BUILD) test
	$(ZIG_BUILD) -Dverifier-profiling=true test-profiling

test-generated-zig: test-zig

test-codegen:
	cd codegen && go test ./...

generate-testdata:
	cd testdata/generate && go run .
	cd testdata/generate/fri && go run .
	cd codegen/generate-riscv-system && go run .

# Opt-in only: the output is under zig-out/ and is not part of normal testdata
# generation or CI. The generator self-checks the proof with prover-ray before
# emitting the verifier-facing Zig literal.
generate-large-pcs-benchmark:
	cd testdata/generate/fri && go run . -large-pcs

# Opt-in only: this builds a PCS-only R5 guest and runs it through zkc with the
# Poseidon2 accelerator enabled. The generated fixture and guest artifacts are
# ignored under zig-out/ and are not part of CI.
profile-large-pcs: generate-large-pcs-benchmark
	cd bench/bench_pcs && go run run.go

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/*.zig testdata/riscv_proof_image.bin

zkc-build: build-r5
	$(ELF_TO_JSON) $(R5_BIN) $(IN_BYTES) > $(R5_JSON)

zkc-verify: zkc-build
	$(ZKC) exec $(R5_JSON) $(ZKC_MAIN)

zkc-verify-passing:
	$(MAKE) zkc-verify

zkc-verify-failing:
	$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true \
		-Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC) \
		-Ddisable-accelerators=$(DISABLE_ACCELERATORS)
	$(ELF_TO_JSON) $(R5_BIN) $(IN_BYTES) > $(R5_JSON)
	$(ZKC) exec $(R5_JSON) $(ZKC_MAIN)

zkc-verify-failing-expected:
	@if $(MAKE) zkc-verify-failing; then \
		echo "expected zkc-verify-failing to fail"; \
		exit 1; \
	else \
		echo "zkc-verify-failing failed as expected"; \
	fi

profile-zkc-case:
	@$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true \
		-Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC) \
		-Dverifier-profiling=true -Dr5-marks=true
	$(ELF_TO_JSON) $(R5_BIN) $(IN_BYTES) > $(R5_JSON)
	@$(ZKC) exec $(R5_JSON) $(ZKC_MAIN)

profile-zkc: generate-testdata
	go run bench/verifier_profile/main.go \
		-out=bench/verifier-profile.csv

clean:
	rm -rf .zig-cache zig-out
