SHELL := /bin/bash

.PHONY: ci help fmt fmt-zig fmt-go build build-zig build-release build-r5 run run-passing run-failing run-failing-expected test test-zig test-go generate-testdata generate-devmode-system verify-testdata zkc-build zkc-verify zkc-verify-passing zkc-verify-failing zkc-verify-failing-expected zkc-trace profile-zkc-case clean

ZIG_OUT ?= zig-out
ZIG_BUILD = zig build --prefix "$(abspath $(ZIG_OUT))"

R5_BIN := $(ZIG_OUT)/bin/verifier-ray
R5_JSON := $(R5_BIN).json
ZKC_MAIN := ../arithmetization/src/main/riscv/main.zkc

# The system the smoke runner verifies, and its honest proof image. Both are
# generated from prover-ray HEAD and not committed (see .gitignore).
DEVMODE_SYSTEM := testdata/generated/devmode_system.zig
DEVMODE_PROOF_IMAGE := testdata/devmode_proof_image.bin
PROOF_IMAGE ?= $(DEVMODE_PROOF_IMAGE)
# The proof image of one verify.zig case, read by test/proof_image_test.zig.
FIXTURE_PROOF_IMAGE := testdata/fixture_proof_image.bin
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

# The generated systems and proof images come from prover-ray HEAD
# (`prover verifier-assets`). go -C runs in ../prover-ray, so pass absolute paths.
VERIFIER_ASSETS = go -C ../prover-ray run ./cmd/prover verifier-assets
# The prover-ray sources the devmode assets are generated from: every non-test Go file (the compilers
# under wiop decide the compiled system too) and the embedded Zig templates of the codegen.
DEVMODE_SYSTEM_INPUTS := $(shell find ../prover-ray -type f \( -name '*.go' -o -name '*.zig.tmpl' \) \
	-not -name '*_test.go' -not -path '*/testdata/*') ../prover-ray/go.mod ../prover-ray/go.sum

ci: generate-testdata fmt build-release run-passing run-failing-expected test zkc-verify-passing zkc-verify-failing-expected

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
		'  run                   Run verifier-ray natively against the devmode proof image' \
		'  run-passing           Alias for run' \
		'  run-failing           Run verifier-ray natively with an invalid embedded fixture (exits non-zero)' \
		'  run-failing-expected  Run native invalid input and require it to fail' \
		'  test                  Run Zig tests and Go tests' \
		'  test-zig              Generate testdata and run Zig tests' \
		'  test-go               Run the Go tests (proof ABI agreement with the prover-ray encoder)' \
		'  generate-testdata     Generate testdata for Zig tests from prover-ray HEAD' \
		'  verify-testdata       Verify generated testdata is up to date with git (for CI)' \
		'  zkc-build             Build the R5 ELF and generate the zkc JSON input' \
		'  zkc-verify            Run verifier-ray through the zkc RISC-V interpreter against the devmode proof image' \
		'  zkc-verify-passing    Alias for zkc-verify' \
		'  zkc-verify-failing    Run zkc with an invalid embedded fixture (exits non-zero)' \
		'  zkc-verify-failing-expected  Require zkc invalid input to fail' \
		'  zkc-trace             Trace the zkc run of zkc-verify and print its statistics' \
		'  clean                 Remove Zig build artifacts' \
		'zkc variables:' \
		'  PROOF_IMAGE          Proof of the devmode system, default testdata/devmode_proof_image.bin' \
		'' \
		'Zig build variables:' \
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

# Every build compiles the devmode system, so every build target depends on it.
build-zig: $(DEVMODE_SYSTEM)
	$(ZIG_BUILD)

build-release: $(DEVMODE_SYSTEM)
	$(ZIG_BUILD) --release=small -Dstrip=true

build-r5: $(DEVMODE_SYSTEM)
	$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true -Ddisable-accelerators=$(DISABLE_ACCELERATORS)

run: $(DEVMODE_SYSTEM) $(PROOF_IMAGE)
	$(ZIG_BUILD) run -Dinput-path="$(PROOF_IMAGE)"

run-passing:
	$(MAKE) run

run-failing: $(DEVMODE_SYSTEM) $(PROOF_IMAGE)
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

test: test-zig test-go

# Depends on the generated files, not on generate-testdata, so CI's
# verify-testdata followed by test proves each system once. Run
# generate-testdata to refresh them against prover-ray HEAD.
test-zig: $(DEVMODE_SYSTEM) $(FIXTURE_PROOF_IMAGE)
	$(ZIG_BUILD) test
	$(ZIG_BUILD) -Dverifier-profiling=true test-profiling

# The codegen tests run with prover-ray's; these are verifier-ray's own.
test-go:
	go test ./...

# cmd/generate-testdata writes verify.zig and the fixture proof image.
generate-testdata:
	go run ./cmd/generate-testdata
	$(MAKE) generate-devmode-system

# The image belongs to the committed verify.zig (image_case_index and that case's system), so it is
# rebuilt whenever verify.zig or its generator changes. `-image-only` writes only the image, and fails
# unless the committed verify.zig is exactly what the generator produces now.
$(FIXTURE_PROOF_IMAGE): testdata/generated/verify.zig $(wildcard cmd/generate-testdata/*.go)
	go run ./cmd/generate-testdata -image-only

generate-devmode-system:
	$(VERIFIER_ASSETS) --system devmode \
		--zig "$(abspath $(DEVMODE_SYSTEM))" --image "$(abspath $(DEVMODE_PROOF_IMAGE))"

$(DEVMODE_SYSTEM) $(DEVMODE_PROOF_IMAGE) &: $(DEVMODE_SYSTEM_INPUTS)
	$(MAKE) generate-devmode-system

verify-testdata: generate-testdata
	git diff --exit-code -- testdata/generated/verify.zig

zkc-build: $(PROOF_IMAGE) build-r5
	$(ELF_TO_JSON) "$(abspath $(R5_BIN))" $(IN_BYTES) > $(R5_JSON)

zkc-verify: zkc-build
	$(ZKC) exec "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

zkc-verify-passing:
	$(MAKE) zkc-verify

zkc-verify-failing: $(DEVMODE_SYSTEM) $(PROOF_IMAGE)
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

zkc-trace: zkc-build
	$(ZKC) trace --stats -r -v "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

profile-zkc-case: $(DEVMODE_SYSTEM) $(PROOF_IMAGE)
	@$(ZIG_BUILD) --release=small -Dstrip=true -Dr5=true \
		-Dembedded-input=$(EMBEDDED_INPUT) -Dembedded-spec=$(EMBEDDED_SPEC) \
		-Dverifier-profiling=true -Dr5-marks=true
	$(ELF_TO_JSON) "$(abspath $(R5_BIN))" $(IN_BYTES) > $(R5_JSON)
	@$(ZKC) exec "$(abspath $(R5_JSON))" "$(abspath $(ZKC_MAIN))"

clean:
	rm -rf .zig-cache zig-out
