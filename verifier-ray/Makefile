SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-codegen generate-testdata generate-devmode-system verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected zkc-verify-devmode zkc-trace-devmode profile-zkc-case clean

RISCV_SYSTEM ?= testdata/generated/riscv_system.zig
ZIG_OUT ?= zig-out
ZIG_BUILD = zig build \
	--prefix "$(abspath $(ZIG_OUT))" \
	-Driscv-system="$(abspath $(RISCV_SYSTEM))"

R5_BIN := $(ZIG_OUT)/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc
PROOF_IMAGE ?= testdata/riscv_proof_image.bin

DEVMODE_SYSTEM := testdata/generated/devmode_system.zig
DEVMODE_PROOF_IMAGE := testdata/devmode_proof_image.bin
# Optional legacy embedded-fixture mode, still used by the profiling targets.
# The default smoke-test run/zkc path loads the generated real proof image.
EMBEDDED_INPUT ?= valid
EMBEDDED_SPEC ?= 0

# elf-to-json accepts @path for raw bytes placed directly at `_in_start`.
IN_BYTES = @$(abspath $(PROOF_IMAGE))

DISABLE_ACCELERATORS ?= true

# Run zkc and elf_to_json from ../arithmetization so their versions match ZKC_MAIN.
ZKC = go -C ../arithmetization tool zkc
ELF_TO_JSON = go -C ../arithmetization tool elf_to_json

ci: generate-testdata fmt build-release run-passing run-failing-expected test zkc-verify-passing zkc-verify-failing-expected zkc-verify-devmode

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
		'  clean                 Remove Zig build artifacts' \
		'zkc variables:' \
		'  PROOF_IMAGE          Proof paired with RISCV_SYSTEM, default testdata/riscv_proof_image.bin' \
		'' \
		'Zig build variables:' \
		'  RISCV_SYSTEM         Generated verifier system source, default testdata/generated/riscv_system.zig' \
		'  ZIG_OUT              Per-build output prefix, default zig-out' \
		'  DISABLE_ACCELERATORS  Disable Lineth zkVM accelerator wrappers (for compatibility testing), default true' \

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

build-zig: $(RISCV_SYSTEM) $(DEVMODE_SYSTEM)
	$(ZIG_BUILD)

build-release: $(RISCV_SYSTEM)
	$(ZIG_BUILD) --release=small -Dstrip=true

build-r5: $(RISCV_SYSTEM)
	$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true -Ddisable-accelerators=$(DISABLE_ACCELERATORS)

# The RISC-V system and its proof image are generated together and not
# committed (see .gitignore); every target that reads them depends on them, so
# they are produced when missing. The generator writes these default paths
# only, so an overridden RISCV_SYSTEM / PROOF_IMAGE must already exist.
testdata/generated/riscv_system.zig testdata/riscv_proof_image.bin &:
	cd codegen/generate-riscv-system && go run .

run: $(PROOF_IMAGE)
	$(ZIG_BUILD) run -Dinput-path="$(PROOF_IMAGE)"

run-passing:
	$(MAKE) run

run-failing: $(PROOF_IMAGE)
	$(ZIG_BUILD) run -Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC)

# Generate the input outside the `if`, so a generation failure fails this
# target instead of counting as the expected rejection.
run-failing-expected: $(PROOF_IMAGE)
	@if $(MAKE) run-failing; then \
		echo "expected run-failing to fail"; \
		exit 1; \
	else \
		echo "run-failing failed as expected"; \
	fi

test: test-zig test-codegen

# Depends on the generated files, not on generate-testdata, so CI's
# verify-testdata followed by test proves each system once. Run
# generate-testdata to refresh them against prover-ray HEAD.
test-zig: testdata/generated/riscv_system.zig $(DEVMODE_SYSTEM)
	$(ZIG_BUILD) test
	$(ZIG_BUILD) -Dverifier-profiling=true test-profiling

test-codegen:
	cd codegen && go test ./...

generate-testdata:
	cd testdata/generate && go run .
	cd codegen/generate-riscv-system && go run .
	cd codegen/generate-devmode-system && go run .

generate-devmode-system:
	cd codegen/generate-devmode-system && go run .

$(DEVMODE_SYSTEM) $(DEVMODE_PROOF_IMAGE) &: codegen/devmode_bootstrap.go codegen/generate-devmode-system/main.go
	cd codegen/generate-devmode-system && go run .

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/verify.zig

zkc-build: $(PROOF_IMAGE) build-r5
	$(ELF_TO_JSON) "$(abspath $(R5_BIN))" $(IN_BYTES) > $(R5_JSON)

zkc-verify: zkc-build
	$(ZKC) exec "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

zkc-verify-passing:
	$(MAKE) zkc-verify

zkc-verify-failing: $(PROOF_IMAGE)
	$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true \
		-Dembedded-input=invalid -Dembedded-spec=$(EMBEDDED_SPEC) \
		-Ddisable-accelerators=$(DISABLE_ACCELERATORS)
	$(ELF_TO_JSON) "$(abspath $(R5_BIN))" $(IN_BYTES) > $(R5_JSON)
	$(ZKC) exec "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

zkc-verify-failing-expected: $(PROOF_IMAGE)
	@if $(MAKE) zkc-verify-failing; then \
		echo "expected zkc-verify-failing to fail"; \
		exit 1; \
	else \
		echo "zkc-verify-failing failed as expected"; \
	fi

zkc-verify-devmode: $(DEVMODE_PROOF_IMAGE)
	$(MAKE) zkc-verify RISCV_SYSTEM=$(DEVMODE_SYSTEM) PROOF_IMAGE=$(DEVMODE_PROOF_IMAGE)

zkc-trace-devmode: $(DEVMODE_PROOF_IMAGE)
	$(MAKE) zkc-build RISCV_SYSTEM=$(DEVMODE_SYSTEM) PROOF_IMAGE=$(DEVMODE_PROOF_IMAGE)
	$(ZKC) trace --stats -r -v "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"
profile-zkc-case: $(PROOF_IMAGE)
	@$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true \
		-Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC) \
		-Dverifier-profiling=true -Dr5-marks=true
	$(ELF_TO_JSON) "$(abspath $(R5_BIN))" $(IN_BYTES) > $(R5_JSON)
	@$(ZKC) exec "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

clean:
	rm -rf .zig-cache zig-out
