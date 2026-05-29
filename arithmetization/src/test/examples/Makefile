# For the target we refer to https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md.
# Note that Zicclsm extension does not affect the generated ELF so it is omitted.
# Moreover, ABI being LP64 (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

# Declare all targets as phony to prevent conflicts with files of the same name
.PHONY: exec debug compile exec-elf elf-to-json install-zkc zkc-exec zkc-debug clean clean-all linker-script verify-elf blake-rust-build blake-rust-json blake-rust-exec keccak-rust-build keccak-rust-json keccak-rust-exec build-act4 run-act4 require-test require-src require-bin-ext

# Keep validation, build, and execution targets running in order
.NOTPARALLEL:

######################################################
# Variables and support functions
######################################################

# Directory of this Makefile (used for relative paths)
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ARITHMETIZATION_DIR := $(abspath $(MAKEFILE_DIR)../../..)

# Paths to go main (to turn BIN into JSON), zkc main (to execute/debug the json) and linker script
GO_MAIN       := $(MAKEFILE_DIR)main.go
ZKC_MAIN      := $(MAKEFILE_DIR)../../main/riscv/main.zkc
LINKER_SCRIPT := $(MAKEFILE_DIR)linker_script.ld
# Assembly, Zig, or Rust test file to execute with exec target (TEST=src_optional_subfolder/your_test(.s|.zig|.rs)
TEST ?=

# Already compiled binary to execute with exec-elf target (BIN_EXT=path/to/your_bin)
BIN_EXT  ?=
JSON_EXT ?= $(BIN_EXT).json

# If TEST=src_optional_subfolder/your_test(.s|.zig|.rs), the variables below are set as follows:
# src_optional_subfolder/your_test
BASENAME     := $(basename $(TEST))

# your_test
NAME 		 := $(notdir $(BASENAME))

# src_optional_subfolder/
SUBDIR       := $(dir $(TEST))

# your_test(.s|.zig|.rs)
NOTSUBDIR    := $(notdir $(TEST))

# (.s|.zig|.rs)
EXT          := $(suffix $(TEST))

# Paths below are determined by TEST and the extension
SRC  := $(MAKEFILE_DIR)$(if $(filter .s,$(EXT)),asm,$(if $(filter .zig,$(EXT)),zig,rust))/src/$(TEST)
BIN  := $(MAKEFILE_DIR)$(if $(filter .s,$(EXT)),asm/bin/,$(if $(filter .zig,$(EXT)),zig/zig-out/bin/,rust/target/riscv64im-unknown-none-elf/release/))$(NAME)
JSON := $(BIN).json
# Note: in the future the JSON file will be removed and main.zkc will receive directly the ELF file

# Inputs and offsets for the program
IN_BYTES        ?= ""

# Keccak guest: compile-time vector count
KECCAK_N_VECTORS ?= 10

# Values to generate the linker script
# 128 MiB (0x00000000 -> 0x07FFFFFF)
PROGRAM_OFFSET  ?= 0x00000000

# 1 GiB   (0x08800000 -> 0x48FFFFFF)
IN_BYTES_OFFSET ?= 0x08800000

# Since program starts from _start, the entry point is the same as the program offset
ENTRY_POINT     := $(PROGRAM_OFFSET)

# 8 MiB   (0x08000000 <- 0x08800000)
SP              ?= 0x08800000

# Parsed values are just to verify that the generated ELF is correct (lazy evaluation allows to parse the ELF only after it is generated)
PROGRAM_OFFSET_FROM_ELF  = $(shell LANG=C riscv64-unknown-elf-readelf -S $(BIN) | grep '\.text' | awk '{print "0x"$$5}')
IN_BYTES_OFFSET_FROM_ELF = $(shell LANG=C riscv64-unknown-elf-nm $(BIN) | grep _input_start | awk '{print "0x"$$1}') 
ENTRY_POINT_FROM_ELF     = $(shell LANG=C riscv64-unknown-elf-readelf -h $(BIN) | grep 'Entry point' | awk '{print $$4}')
SP_FROM_ELF              = $(shell LANG=C riscv64-unknown-elf-nm $(BIN) | grep _stack_start | awk '{print "0x"$$1}')
VERIFY_ELF               ?= false             

# ACT4 testing framework specific variables
ACT4_CONFIG_DIR := $(MAKEFILE_DIR)act4/config
ACT4_WORK_DIR   := $(MAKEFILE_DIR)act4/bin/work
ACT4_CONFIG     := $(ACT4_CONFIG_DIR)/linea-rv64im-zicclsm/test_config.yaml
# The 3 parameters below must be consistent with the ones provided to arithmetization/Makefile
ACT4_REF        ?= 9798a554ce4139f472c9ccd3a18c9061d0f7024d
ACT4_REPO       ?= $(abspath $(shell git -C $(MAKEFILE_DIR) rev-parse --show-toplevel)/../riscv-arch-test)
ACT4_RISCV_DIR  ?= $(HOME)/riscv
ACT4_IMAGE      := riscv-act4:$(ACT4_REF)
ACT4_EXTENSIONS := I,M
ACT4_JOBS       := 4
ACT4_ELF_DIR    := $(ACT4_WORK_DIR)/linea-rv64im-zicclsm/elfs
ACT4_LOGS       := $(MAKEFILE_DIR)act4/bin/logs
ACT4_RESULTS    := $(MAKEFILE_DIR)act4/bin/results.txt
# ACT4_BUILD_MODE = (host|docker)
ACT4_BUILD_MODE ?= host
# Parameters passed to https://github.com/riscv/riscv-arch-test/blob/act4/Makefile:
# If true, disables objdump generation for faster builds
ACT4_FAST       ?= false
# If true, enables debug output (signature objdump, trace files, and trap report)
ACT4_DEBUG      ?= true

# String to determine if a program is exiting with exit code 0
EXIT_SUCCESS_STRING := "EXIT CODE = 0"

# Support function to generate JSON from an ELF
define ELF_TO_JSON
	@echo "\nGenerating $(2) from $(1)"
	@go run $(GO_MAIN) "$(1)" "$(IN_BYTES)" $(IN_BYTES_OFFSET) > "$(2)"
endef

######################################################
# Targets
######################################################

# Default target (make)
exec: compile zkc-exec

debug: compile zkc-debug

compile: require-src linker-script
# Print all variables for debugging purposes
	@echo "TEST:      $(TEST)"
	@echo "BASENAME:  $(BASENAME)"
	@echo "NAME:      $(NAME)"
	@echo "SUBDIR:    $(SUBDIR)"
	@echo "NOTSUBDIR: $(NOTSUBDIR)"
	@echo "EXT:       $(EXT)"
	@echo "SRC:       $(SRC)"
	@echo "BIN:       $(BIN)"
	@echo "JSON:      $(JSON)"
# Generate ELF
	@if [ "$(EXT)" = ".s" ]; then \
		mkdir -p $(MAKEFILE_DIR)asm/bin; \
		echo "\nGenerating $(BIN) from $(SRC)"; \
		riscv64-unknown-elf-gcc -march=rv64im -mabi=lp64 -nostdlib -T$(LINKER_SCRIPT) -o $(BIN) $(SRC); \
	elif [ "$(EXT)" = ".zig" ]; then \
		echo "\nGenerating $(BIN) from $(SRC)"; \
		zig build --build-file $(MAKEFILE_DIR)zig/build.zig -Dpath=$(BASENAME); \
	elif [ "$(EXT)" = ".rs" ]; then \
		echo "\nGenerating $(BIN) from $(SRC)"; \
		if [ "$(NAME)" = "keccak" ]; then \
			env KECCAK_N_VECTORS="$(KECCAK_N_VECTORS)" cargo +nightly rustc \
				--manifest-path $(MAKEFILE_DIR)rust/Cargo.toml \
				--target $(MAKEFILE_DIR)rust/riscv64im-unknown-none-elf.json \
				-Z build-std=core \
				-Z json-target-spec \
				--bin $(NAME) \
				--release \
				-- -C link-arg=-T$(LINKER_SCRIPT); \
		else \
			cargo +nightly rustc \
				--manifest-path $(MAKEFILE_DIR)rust/Cargo.toml \
				--target $(MAKEFILE_DIR)rust/riscv64im-unknown-none-elf.json \
				-Z build-std=core \
				-Z json-target-spec \
				--bin $(NAME) \
				--release \
				-- -C link-arg=-T$(LINKER_SCRIPT); \
		fi; \
	else \
		echo "Unsupported file extension: $(EXT)"; \
		exit 1; \
	fi
# Disassemble ELF
	@echo "\nDisassembling $(BIN)"
	@LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(BIN) > $(BIN).objdump
# Verify that PROGRAM_OFFSET, IN_BYTES_OFFSET, ENTRY_POINT, SP match the ELF values
	@if [ "$(VERIFY_ELF)" = "true" ]; then \
		$(MAKE) -f $(MAKEFILE_DIR)Makefile verify-elf TEST=$(TEST); \
	fi
# Generate JSON from ELF
	$(call ELF_TO_JSON,$(BIN),$(JSON))

# Note: compiled Rust libraries may contain instructions from excluded extensions. For instance, the core library is compiled
# for the IMAC extension, and contains COMPRESSED instructions. The output ELF may thus contain such instructions, too, even
# if these extensions were explicitly exluded from the target.
#
# Note: we use opt-level=2 to remove unused code.

# Convert and execute an already compiled external ELF
exec-elf: elf-to-json
	zkc exec $(JSON_EXT) $(ZKC_MAIN)

# Convert an already compiled external ELF to a JSON
elf-to-json: require-bin-ext
# Generate JSON from ELF
	$(call ELF_TO_JSON,$(BIN_EXT),$(JSON_EXT))

install-zkc:
	$(MAKE) -C $(ARITHMETIZATION_DIR) install-zkc

zkc-exec: require-test install-zkc
	zkc exec $(JSON) $(ZKC_MAIN)

zkc-debug: require-test install-zkc
	zkc debug $(JSON) $(ZKC_MAIN)

clean: require-test
	rm -f $(BIN) $(JSON) $(BIN).objdump $(LINKER_SCRIPT)

clean-all:
	rm -rf $(MAKEFILE_DIR)asm/bin/
	rm -rf $(MAKEFILE_DIR)zig/zig-out/
	rm -rf $(MAKEFILE_DIR)zig/.zig-cache/
	rm -rf $(MAKEFILE_DIR)rust/target/
	rm -rf $(MAKEFILE_DIR)act4/bin/
	rm -f  $(LINKER_SCRIPT)

# Generate the linker script with the specified memory layout
linker-script:
	@echo "\nGenerating $(LINKER_SCRIPT)"
	@printf '%s\n' \
		'ENTRY(_start)' \
		'' \
		'MEMORY {' \
		'    PROGRAM (rx) : ORIGIN = $(PROGRAM_OFFSET), LENGTH = 0x8000000' \
		'    STACK   (rw) : ORIGIN = $(shell printf "0x%08x" $$(($(SP) - 0x800000))), LENGTH = 0x800000' \
		'    IN  (r)  : ORIGIN = $(IN_BYTES_OFFSET), LENGTH = 0x40000000' \
		'}' \
		'' \
		'SECTIONS {' \
		'    . = ORIGIN(PROGRAM);' \
		'' \
		'    .text : {' \
		'        *(.text .text.*)' \
		'    } > PROGRAM' \
		'' \
		'    .rodata : {' \
		'        *(.rodata .rodata.*)' \
		'    } > PROGRAM' \
		'' \
		'    .data : {' \
		'        *(.data .data.*)' \
		'    } > PROGRAM' \
		'' \
		'    .bss : {' \
		'        *(.bss .bss.*)' \
		'    } > PROGRAM' \
		'' \
		'    /* stack grows downward from the top of the stack region */' \
		'    _stack_start = ORIGIN(STACK) + LENGTH(STACK);' \
		'' \
		'    /* symbol marking the start of the input region */' \
		'    _input_start = ORIGIN(IN);' \
		'}' \
		> $(LINKER_SCRIPT)

# Verify that PROGRAM_OFFSET, IN_BYTES_OFFSET, ENTRY_POINT, SP match the ELF values
verify-elf: require-test
	@echo "\nVerify that PROGRAM_OFFSET, IN_BYTES_OFFSET, ENTRY_POINT, SP match the ELF values"
	@[ "$(shell printf '0x%016x' $$(($(PROGRAM_OFFSET_FROM_ELF))))" = "$(shell printf '0x%016x' $$(($(PROGRAM_OFFSET))))" ] || \
		(echo "ERROR: PROGRAM_OFFSET mismatch: expected $(PROGRAM_OFFSET), got $(PROGRAM_OFFSET_FROM_ELF)" && exit 1)
	@echo "  PROGRAM_OFFSET:  $(PROGRAM_OFFSET) (expected) = $(PROGRAM_OFFSET_FROM_ELF) (from ELF)"
	@[ "$(shell printf '0x%016x' $$(($(IN_BYTES_OFFSET_FROM_ELF))))" = "$(shell printf '0x%016x' $$(($(IN_BYTES_OFFSET))))" ] || \
		(echo "ERROR: IN_BYTES_OFFSET mismatch: expected $(IN_BYTES_OFFSET), got $(IN_BYTES_OFFSET_FROM_ELF)" && exit 1)
	@echo "  IN_BYTES_OFFSET: $(IN_BYTES_OFFSET) (expected) = $(IN_BYTES_OFFSET_FROM_ELF) (from ELF)"
	@[ "$(shell printf '0x%016x' $$(($(ENTRY_POINT_FROM_ELF))))" = "$(shell printf '0x%016x' $$(($(ENTRY_POINT))))" ] || \
		(echo "ERROR: ENTRY_POINT mismatch: expected $(ENTRY_POINT), got $(ENTRY_POINT_FROM_ELF)" && exit 1)
	@echo "  ENTRY_POINT:     $(ENTRY_POINT) (expected) = $(ENTRY_POINT_FROM_ELF) (from ELF)"
	@[ "$(shell printf '0x%016x' $$(($(SP_FROM_ELF))))" = "$(shell printf '0x%016x' $$(($(SP))))" ] || \
		(echo "ERROR: SP mismatch: expected $(SP), got $(SP_FROM_ELF)" && exit 1)
	@echo "  SP:              $(SP) (expected) = $(SP_FROM_ELF) (from ELF)"
	@echo "All values match\n"

######################################################
# Keccak (Rust guest) — split build/json/exec for reuse from CI benchmarks.
#
# `keccak-rust-build` — ELF + helper binaries (no zkc dependency).
# `keccak-rust-json`  — pack first KECCAK_N_VECTORS vectors of KECCAK_ACCEPTS into
#                       a single batched IN_BYTES blob, then render the ZKC JSON
#                       at KECCAK_JSON. No zkc dependency.
# `keccak-rust-exec`  — depends on keccak-rust-json + install-zkc; runs
#                       `zkc exec` on the produced JSON.
######################################################
KECCAK_ACCEPTS      ?= $(MAKEFILE_DIR)rust/src/keccak/keccak.accepts

KECCAK_ELF          := $(MAKEFILE_DIR)rust/target/riscv64im-unknown-none-elf/release/keccak
KECCAK_JSON         ?= $(KECCAK_ELF).json
KECCAK_ELF2JSON     := $(MAKEFILE_DIR)rust/target/keccak_elf2json
KECCAK_ACCEPTS2HEX  := $(MAKEFILE_DIR)rust/target/keccak_accepts2hex
KECCAK_IN_BYTES_HEX := /tmp/keccak_in_bytes.hex

keccak-rust-build:
	cargo clean -p riscv-tests --manifest-path $(MAKEFILE_DIR)rust/Cargo.toml
	$(MAKE) -f $(MAKEFILE_DIR)Makefile compile TEST=keccak/keccak.rs IN_BYTES=0x00 KECCAK_N_VECTORS=$(KECCAK_N_VECTORS)
	@echo "building keccak_elf2json..."
	go build -o $(KECCAK_ELF2JSON) $(GO_MAIN)
	@echo "building keccak_accepts2hex..."
	go build -o $(KECCAK_ACCEPTS2HEX) $(MAKEFILE_DIR)scripts/keccak_accepts_to_in_bytes/main.go

keccak-rust-json: keccak-rust-build
	@echo "packing first $(KECCAK_N_VECTORS) vector(s) from $(KECCAK_ACCEPTS) into $(KECCAK_IN_BYTES_HEX)"
	$(KECCAK_ACCEPTS2HEX) "$(KECCAK_ACCEPTS)" $(KECCAK_N_VECTORS) > $(KECCAK_IN_BYTES_HEX)
	@echo "rendering ZKC JSON to $(KECCAK_JSON)"
	$(KECCAK_ELF2JSON) $(KECCAK_ELF) @$(KECCAK_IN_BYTES_HEX) $(IN_BYTES_OFFSET) > $(KECCAK_JSON)

keccak-rust-exec: install-zkc keccak-rust-json
	zkc exec -q $(KECCAK_JSON) $(ZKC_MAIN)

######################################################
# Blake (Rust guest) — split build/json/exec mirroring Keccak's structure.
#
# `blake-rust-build` — ELF + helper binaries (no zkc dependency).
# `blake-rust-json`  — select first BLAKE_N_VECTORS lines of BLAKE_ACCEPTS (0 =
#                      all lines) and render one ZKC JSON per vector under
#                      BLAKE_JSON_DIR. No zkc dependency.
# `blake-rust-exec`  — depends on blake-rust-json + install-zkc; loops over the
#                      produced JSONs and runs `zkc exec` with PASS/FAIL
#                      accounting (preserves the prior behaviour).
#
# Defaults preserve the current `arithmetization-sample-guest-programs-run.yml`
# workflow (BLAKE_ACCEPTS=blake10.all + BLAKE_N_VECTORS=0 -> all 10 vectors).
######################################################
BLAKE_ACCEPTS       ?= $(MAKEFILE_DIR)rust/src/blake/blake10.all
BLAKE_N_VECTORS     ?= 0

BLAKE_ELF           := $(MAKEFILE_DIR)rust/target/riscv64im-unknown-none-elf/release/blake_with_in_bytes
BLAKE_JSON_DIR      ?= $(MAKEFILE_DIR)rust/target/riscv64im-unknown-none-elf/release/blake_json
BLAKE_ELF2JSON      := $(MAKEFILE_DIR)rust/target/blake_elf2json
BLAKE_ACCEPTS2HEX   := $(MAKEFILE_DIR)rust/target/blake_accepts2hex
BLAKE_IN_BYTES_ALL  := /tmp/blake_in_bytes.all

blake-rust-build:
	$(MAKE) -f $(MAKEFILE_DIR)Makefile compile TEST=blake/blake_with_in_bytes.rs
	@echo "building blake_elf2json..."
	go build -o $(BLAKE_ELF2JSON) $(GO_MAIN)
	@echo "building blake_accepts2hex..."
	go build -o $(BLAKE_ACCEPTS2HEX) $(MAKEFILE_DIR)scripts/blake_accepts_to_in_bytes/main.go

blake-rust-json: blake-rust-build
	@echo "selecting vectors from $(BLAKE_ACCEPTS) (N=$(BLAKE_N_VECTORS), 0=all)"
	$(BLAKE_ACCEPTS2HEX) "$(BLAKE_ACCEPTS)" $(BLAKE_N_VECTORS) > $(BLAKE_IN_BYTES_ALL)
	@mkdir -p $(BLAKE_JSON_DIR)
	@rm -f $(BLAKE_JSON_DIR)/*.json
	@echo "rendering one ZKC JSON per vector under $(BLAKE_JSON_DIR)"
	@n=0; while IFS= read -r in_bytes; do \
		n=$$((n + 1)); idx=$$(printf '%03d' $$n); \
		$(BLAKE_ELF2JSON) $(BLAKE_ELF) "$$in_bytes" $(IN_BYTES_OFFSET) \
			> "$(BLAKE_JSON_DIR)/$$idx.json" || exit $$?; \
	done < $(BLAKE_IN_BYTES_ALL); \
	echo "generated $$n Blake JSON(s) in $(BLAKE_JSON_DIR)"

blake-rust-exec: install-zkc blake-rust-json
	@pass=0; fail=0; i=0; \
	for json in $(BLAKE_JSON_DIR)/*.json; do \
		i=$$((i + 1)); \
		echo "RUN   case $$i  ($$json)"; \
		if zkc exec -q "$$json" $(ZKC_MAIN) >/dev/null 2>&1; then \
			echo "PASS  case $$i"; \
			pass=$$((pass + 1)); \
		else \
			echo "FAIL  case $$i (zkc exec exited non-zero; rerun without -q for the panic message)"; \
			fail=$$((fail + 1)); \
		fi; \
	done; \
	echo "DONE: $$i vectors   PASS: $$pass   FAIL: $$fail"; \
	if [ "$$fail" -ne 0 ]; then echo "error: $$fail blake vector(s) failed" >&2; exit 1; fi

# Build the ACT4 self-checking ELFs with the Linea ACT4 config 
# Note: build if needed
# Note: clone https://github.com/riscv/riscv-arch-test.git if needed
build-act4:
	@if [ -n "$$(find "$(ACT4_ELF_DIR)" -name '*.elf' -print -quit 2>/dev/null)" ]; then \
		echo "ACT4 ELFs already built in $(ACT4_ELF_DIR)"; \
		exit 0; \
	fi; \
	mkdir -p "$(ACT4_WORK_DIR)" || exit $$?; \
	work_dir=$$(cd "$(ACT4_WORK_DIR)" && pwd) || exit $$?; \
	[ -d "$(ACT4_CONFIG_DIR)/linea-rv64im-zicclsm" ] || { echo "error: missing $(ACT4_CONFIG_DIR)/linea-rv64im-zicclsm"; exit 2; }; \
	ensure_act4_repo() { \
		if [ ! -d "$(ACT4_REPO)" ]; then \
			git clone https://github.com/riscv/riscv-arch-test.git "$(ACT4_REPO)" || exit $$?; \
		fi; \
		if ! git -C "$(ACT4_REPO)" rev-parse "$(ACT4_REF)^{commit}" >/dev/null 2>&1; then \
			git -C "$(ACT4_REPO)" fetch --tags origin || exit $$?; \
		fi; \
		if [ "$$(git -C "$(ACT4_REPO)" rev-parse HEAD)" != "$$(git -C "$(ACT4_REPO)" rev-parse "$(ACT4_REF)^{commit}")" ]; then \
			git -C "$(ACT4_REPO)" checkout --detach "$(ACT4_REF)" || exit $$?; \
		fi; \
	}; \
	if [ "$(ACT4_BUILD_MODE)" = "docker" ]; then \
		config_dir=$$(cd "$(ACT4_CONFIG_DIR)" && pwd) || exit $$?; \
		if ! docker image inspect "$(ACT4_IMAGE)" >/dev/null 2>&1; then \
			ensure_act4_repo; \
			echo "building Docker image $(ACT4_IMAGE) from $(ACT4_REPO)"; \
			docker build -t "$(ACT4_IMAGE)" "$(ACT4_REPO)" || exit $$?; \
		fi; \
		echo "building ACT4 ELFs in $$work_dir with Docker"; \
		docker run --rm \
			-v "$$config_dir:/act4/config/cores/linea:ro" \
			-v "$$work_dir:/act4/work" \
			"$(ACT4_IMAGE)" \
			bash -c "rm -rf /act4/work/linea-rv64im-zicclsm && CONFIG_FILES=config/cores/linea/linea-rv64im-zicclsm/test_config.yaml EXTENSIONS=$(ACT4_EXTENSIONS) $(if $(filter true,$(ACT4_DEBUG)),DEBUG=true) $(if $(filter true,$(ACT4_FAST)),FAST=true) make --jobs $(ACT4_JOBS) --keep-going" || exit $$?; \
	elif [ "$(ACT4_BUILD_MODE)" = "host" ]; then \
		ensure_act4_repo; \
		echo "building ACT4 ELFs in $$work_dir on host"; \
		export PATH="$(ACT4_RISCV_DIR)/bin:$$PATH"; \
		$(MAKE) --jobs $(ACT4_JOBS) --keep-going -C "$(ACT4_REPO)" \
			CONFIG_FILES="$(ACT4_CONFIG)" \
			WORKDIR="$$work_dir" \
			EXTENSIONS="$(ACT4_EXTENSIONS)" \
			$(if $(filter true,$(ACT4_DEBUG)),DEBUG=true) \
			$(if $(filter true,$(ACT4_FAST)),FAST=true) || exit $$?; \
	else \
		echo "error: ACT4_BUILD_MODE must be docker or host"; \
		exit 2; \
	fi

# Build and run the ACT4 self-checking ELFs with the Linea ACT4 config.
run-act4: install-zkc build-act4
	@rm -f "$(ACT4_LOGS)"/*; \
	mkdir -p "$(ACT4_LOGS)"; \
	> "$(ACT4_RESULTS)"; \
	count=0; pass=0; fail=0; \
	for elf in $$(find "$(ACT4_ELF_DIR)" -name '*.elf' | sort); do \
		name=$$(basename "$$elf" .elf); \
		base="$(ACT4_LOGS)/$$name"; \
		count=$$((count + 1)); \
		go run $(GO_MAIN) "$$elf" "" 0 > "$$base.json" 2> "$$base.json.err"; \
		[ -s "$$base.json.err" ] || rm -f "$$base.json.err"; \
		if zkc exec -q "$$base.json" $(ZKC_MAIN) >/dev/null 2>&1; then \
			echo "PASS  $$name"; \
			echo "PASS  $$name" >> "$(ACT4_RESULTS)"; \
			pass=$$((pass + 1)); \
		else \
			zkc exec "$$base.json" $(ZKC_MAIN) > "$$base.full" 2>&1 || true; \
			awk '(previous == "ECALL for write" || previous == "ECALL for exit") && $$0 != "" { print } { previous = $$0 }' "$$base.full" > "$$base.out"; \
			echo "FAIL  $$name"; \
			echo "FAIL  $$name" >> "$(ACT4_RESULTS)"; \
			awk '{ print "  " $$0 }' "$$base.out"; \
			awk '{ print "  " $$0 }' "$$base.out" >> "$(ACT4_RESULTS)"; \
			fail=$$((fail + 1)); \
		fi; \
	done; \
	echo "DONE: $$count tests   PASS: $$pass   FAIL: $$fail" | tee -a "$(ACT4_RESULTS)"; \
	echo "results: $(ACT4_RESULTS)"; \
	echo "logs:    $(ACT4_LOGS)/"; \
	if [ "$$count" -eq 0 ]; then echo "error: no ACT4 ELFs found in $(ACT4_ELF_DIR)" >&2; exit 1; fi; \
	if [ "$$fail" -ne 0 ]; then echo "error: $$fail of $$count ACT4 test(s) failed" >&2; exit 1; fi

######################################################
# Support targets to validate mandatory parameters
######################################################

require-test:
	@[ -n "$(TEST)" ] || { echo "TEST is mandatory. Usage: make TEST=src_optional_subfolder/your_test(.s|.zig|.rs)"; exit 1; }

require-src: require-test
	@[ -f $(SRC) ] || { echo "TEST file not found: $(SRC)"; exit 1; }

require-bin-ext:
	@[ -n "$(BIN_EXT)" ] || { echo "BIN_EXT is mandatory. Usage: make BIN_EXT=path/to/your_bin"; exit 1; }
	@[ -f $(BIN_EXT) ] || { echo "BIN_EXT file not found: $(BIN_EXT)"; exit 1; }
	
