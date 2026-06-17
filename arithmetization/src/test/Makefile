# For the target we refer to https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md.
# Note that Zicclsm extension does not affect the generated ELF so it is omitted.
# Moreover, ABI being LP64 (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

# Declare all targets as phony to prevent conflicts with files of the same name
.PHONY: exec debug compile elf-exec elf-debug elf-to-json install-zkc zkc-exec zkc-debug clean clean-all linker-script verify-elf vector-build vector-json vector-exec keccak-rust-build keccak-rust-json keccak-rust-exec blake-rust-build blake-rust-json blake-rust-exec act4-build act4-exec require-test require-src require-bin-ext require-vector-file require-n-vectors require-vector-build-artifacts require-vector-json-mode

# Keep validation, build, and execution targets running in order
.NOTPARALLEL:

######################################################
# Generic variables and support functions
######################################################

# Directory of this Makefile (used for relative paths)
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ARITHMETIZATION_DIR := $(abspath $(MAKEFILE_DIR)../..)

# Paths to go main (to turn BIN into JSON), zkc main (to execute/debug the json) and linker script
ELF_TO_JSON_GEN := $(MAKEFILE_DIR)scripts/elf_to_json_gen/main.go
ZKC_MAIN        := $(MAKEFILE_DIR)../main/riscv/main.zkc
LINKER_SCRIPT   := $(MAKEFILE_DIR)linker_script.ld
# Assembly, Zig, or Rust test file to execute with exec target (TEST=src_optional_subfolder/your_test(.s|.zig|.rs)
TEST ?=

# Flags to use when invoking zkc exec within zkc-exec and elf-exec targets:  
# 	-q for quiet mode
ZKC_EXEC_FLAGS ?=

# Already compiled binary to execute with elf-exec target (BIN_EXT=path/to/your_bin)
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

# Hex big-endian input or `@path/to/in_bytes`
IN_BYTES        ?= ""

# Support function to evaluate expressions and format them as hex
EVAL = $(shell printf "0x%08x" $$(( $(1) )))

# Configurable parameters for the linker script
STACK_ORIGIN   ?= 0x00000000
SP             ?= 0x00800000
PROGRAM_ORIGIN ?= 0x00800000
IN_ORIGIN      ?= 0x08800000
HEAP_ORIGIN    ?= 0x48800000

# Derived parameters for the linker script
ENTRY_POINT    := $(PROGRAM_ORIGIN)
STACK_LENGTH   := $(call EVAL,$(SP) - $(STACK_ORIGIN))
PROGRAM_LENGTH := $(call EVAL,$(IN_ORIGIN) - $(PROGRAM_ORIGIN))
IN_LENGTH      := $(call EVAL,$(HEAP_ORIGIN) - $(IN_ORIGIN))

# If true, generates an objdump of the generated ELF for debugging purposes
OBJDUMP        ?= false

# If true, verifies ELF entry point, origins, lengths, stack and heap symbols match the ones set
VERIFY_ELF     ?= false

# Support function to generate JSON from an ELF
define ELF_TO_JSON
	@echo "\nGenerating $(2) from $(1)"
	@go run $(ELF_TO_JSON_GEN) "$(1)" "$(IN_BYTES)" $(IN_ORIGIN) > "$(2)"
endef

#####################################################################
# Compilation, execution, debugging and verification targets
#####################################################################

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
		cargo +nightly rustc \
			--manifest-path $(MAKEFILE_DIR)rust/Cargo.toml \
			--target $(MAKEFILE_DIR)rust/riscv64im-unknown-none-elf.json \
			-Z build-std=core \
			-Z json-target-spec \
			--bin $(NAME) \
			--release \
			-- -C link-arg=-T$(LINKER_SCRIPT); \
	else \
		echo "Unsupported file extension: $(EXT)"; \
		exit 1; \
	fi
# Disassemble ELF
	@if [ "$(OBJDUMP)" = "true" ]; then \
		echo "\nDisassembling $(BIN)"; \
		LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(BIN) > $(BIN).objdump; \
	fi
# Verify that the configured memory layout matches the ELF values
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
elf-exec: elf-to-json
	zkc exec $(ZKC_EXEC_FLAGS) $(JSON_EXT) $(ZKC_MAIN)

# Convert and debug an already compiled external ELF
elf-debug: elf-to-json
	zkc debug $(JSON_EXT) $(ZKC_MAIN)

# Convert an already compiled external ELF to a JSON
elf-to-json: require-bin-ext
# Generate JSON from ELF
	$(call ELF_TO_JSON,$(BIN_EXT),$(JSON_EXT))

install-zkc:
	$(MAKE) -C $(ARITHMETIZATION_DIR) install-zkc

zkc-exec: require-test install-zkc
	zkc exec $(ZKC_EXEC_FLAGS) $(JSON) $(ZKC_MAIN)

zkc-debug: require-test install-zkc
	zkc debug $(JSON) $(ZKC_MAIN)

clean: require-test
	rm -f $(BIN) $(JSON) $(BIN).objdump $(VECTOR_SUBSET_FILE) $(LINKER_SCRIPT)

clean-all:
	rm -rf $(MAKEFILE_DIR)asm/bin/
	rm -rf $(MAKEFILE_DIR)zig/zig-out/
	rm -rf $(MAKEFILE_DIR)zig/.zig-cache/
	rm -rf $(MAKEFILE_DIR)rust/target/
	rm -rf $(MAKEFILE_DIR)act4/bin/
	rm -rf $(MAKEFILE_DIR)bin/
	rm -f  $(LINKER_SCRIPT)

# Generate the linker script with the specified memory layout
linker-script:
	@echo "\nGenerating $(LINKER_SCRIPT)"
	@printf '%s\n' \
		'ENTRY(_start)' \
		'' \
		'MEMORY {' \
		'    STACK  (rw) : ORIGIN = $(STACK_ORIGIN), LENGTH = $(STACK_LENGTH)' \
		'    PROGRAM (rx) : ORIGIN = $(PROGRAM_ORIGIN), LENGTH = $(PROGRAM_LENGTH)' \
		'    IN      (r)  : ORIGIN = $(IN_ORIGIN), LENGTH = $(IN_LENGTH)' \
		'    /* HEAP  (rw) : ORIGIN = $(HEAP_ORIGIN), LENGTH = infinite */' \
		'}' \
		'' \
		'SECTIONS {' \
		'    . = ORIGIN(PROGRAM);' \
		'    _program_start = ORIGIN(PROGRAM);' \
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
		'    _stack_end = ORIGIN(STACK);' \
		'    _stack_start = ORIGIN(STACK) + LENGTH(STACK);' \
		'' \
		'    /* program and input regions grow upward */' \
		'    _program_end = ORIGIN(PROGRAM) + LENGTH(PROGRAM);' \
		'    _in_start = ORIGIN(IN);' \
		'    _in_end = ORIGIN(IN) + LENGTH(IN);' \
		'    /* aliases */' \
		'    _input_start = _in_start;' \
		'    _input_end = _in_end;' \
		'' \
		'    /* heap grows upward from this address by program convention */' \
		'    _heap_start = $(HEAP_ORIGIN);' \
		'}' \
		> $(LINKER_SCRIPT)

# Verify that the generated ELF matches the configured memory layout
verify-elf: require-test
# Support function descriptions:
# elf_symbol:   reads a linker-defined symbol address from the ELF symbol table
# section_addr: reads a section virtual address from the ELF section headers
# hex:          normalizes decimal or hex values to 16-digit hex before comparing them
# verify:       compares one expected Makefile value with the corresponding ELF value
	@echo "\nVerify that the generated ELF matches the configured memory layout"
	@set -e; \
	elf_symbol() { \
		LANG=C riscv64-unknown-elf-nm $(BIN) | awk -v symbol="$$1" '$$3 == symbol { print "0x" $$1; found = 1; exit } END { if (!found) exit 1 }'; \
	}; \
	section_addr() { \
		LANG=C riscv64-unknown-elf-readelf -S $(BIN) | awk -v section="$$1" '{ for (i = 1; i <= NF; i++) if ($$i == section) { print "0x" $$(i + 2); found = 1; exit } } END { if (!found) exit 1 }'; \
	}; \
	hex() { printf '0x%016x' "$$(( $$1 ))"; }; \
	verify() { \
		label="$$1"; \
		expected_raw="$$2"; \
		actual_raw="$$3"; \
		expected="$$(hex "$$expected_raw")"; \
		actual="$$(hex "$$actual_raw")"; \
		if [ "$$actual" != "$$expected" ]; then \
			echo "ERROR: $$label mismatch: expected $$expected, got $$actual"; \
			exit 1; \
		fi; \
		printf '  %-16s %s (expected) = %s (from ELF)\n' "$$label:" "$$expected" "$$actual"; \
	}; \
	verify "ENTRY_POINT" "$(ENTRY_POINT)" "$$(LANG=C riscv64-unknown-elf-readelf -h $(BIN) | awk '/Entry point/ { print $$4; found = 1; exit } END { if (!found) exit 1 }')"; \
	verify "STACK_ORIGIN" "$(STACK_ORIGIN)" "$$(elf_symbol _stack_end)"; \
	verify "STACK_LENGTH" "$(STACK_LENGTH)" "$$(( $$(elf_symbol _stack_start) - $$(elf_symbol _stack_end) ))"; \
	verify "PROGRAM_ORIGIN" "$(PROGRAM_ORIGIN)" "$$(section_addr .text)"; \
	verify "PROGRAM_LENGTH" "$(PROGRAM_LENGTH)" "$$(( $$(elf_symbol _program_end) - $$(elf_symbol _program_start) ))"; \
	verify "IN_ORIGIN" "$(IN_ORIGIN)" "$$(elf_symbol _in_start)"; \
	verify "IN_LENGTH" "$(IN_LENGTH)" "$$(( $$(elf_symbol _in_end) - $$(elf_symbol _in_start) ))"; \
	verify "HEAP_ORIGIN" "$(HEAP_ORIGIN)" "$$(elf_symbol _heap_start)"; \
	verify "_stack_end" "$(STACK_ORIGIN)" "$$(elf_symbol _stack_end)"; \
	verify "_stack_start" "$(SP)" "$$(elf_symbol _stack_start)"; \
	verify "_program_start" "$(PROGRAM_ORIGIN)" "$$(elf_symbol _program_start)"; \
	verify "_program_end" "$(IN_ORIGIN)" "$$(elf_symbol _program_end)"; \
	verify "_in_start" "$(IN_ORIGIN)" "$$(elf_symbol _in_start)"; \
	verify "_in_end" "$(HEAP_ORIGIN)" "$$(elf_symbol _in_end)"; \
	verify "_heap_start" "$(HEAP_ORIGIN)" "$$(elf_symbol _heap_start)"; \
	echo "All values match\n"

#####################################################################
# Test vector targets
#####################################################################

VECTOR_FILE                    ?=
VECTOR_N_VECTORS               ?=
# `VECTOR_JSON_MODE` is `per-vector` for one JSON per vector or `batched` for one batched JSON
VECTOR_JSON_MODE               ?= per-vector
VECTOR_JSON_FILE               ?= $(JSON)
VECTOR_JSON_DIR                ?= $(dir $(JSON))vector_json
VECTOR_SUBSET_FILE             ?= $(BIN).all

vector-build: require-src
	$(MAKE) -f $(MAKEFILE_DIR)Makefile compile TEST=$(TEST)
	@elf_to_json_bin="$(MAKEFILE_DIR)bin/elf_to_json_gen"; \
	mkdir -p "$(MAKEFILE_DIR)bin"; \
	echo "building $$elf_to_json_bin..."; \
	go build -o "$$elf_to_json_bin" $(ELF_TO_JSON_GEN)

vector-json: require-src require-vector-file require-n-vectors require-vector-build-artifacts require-vector-json-mode
	@echo "selecting $(if $(filter -1,$(VECTOR_N_VECTORS)),all,$(VECTOR_N_VECTORS)) vector(s) from $(VECTOR_FILE)"
# VECTOR_FILE must contain one `IN_BYTES` value per line
# `per-vector` keeps one line per JSON; `batched` writes one big-endian blob for the whole batch
	@available=$$(awk 'END { print NR }' $(VECTOR_FILE)); \
	if [ "$(VECTOR_N_VECTORS)" != "-1" ] && [ "$$available" -lt "$(VECTOR_N_VECTORS)" ]; then \
		echo "$(VECTOR_FILE): only $$available vector(s) available, requested $(VECTOR_N_VECTORS)"; \
		exit 1; \
	fi; \
	if [ "$(VECTOR_JSON_MODE)" = "batched" ]; then \
		if [ "$(VECTOR_N_VECTORS)" = "-1" ]; then cat "$(VECTOR_FILE)"; else head -n $(VECTOR_N_VECTORS) "$(VECTOR_FILE)"; fi | \
			awk '{ sub(/^0[xX]/, ""); line[NR] = $$0 } END { printf "0x"; for (i = NR; i >= 1; i--) printf "%s", line[i]; printf "\n" }' > $(VECTOR_SUBSET_FILE); \
	elif [ "$(VECTOR_N_VECTORS)" = "-1" ]; then \
		cat "$(VECTOR_FILE)" > $(VECTOR_SUBSET_FILE); \
	else \
		head -n $(VECTOR_N_VECTORS) "$(VECTOR_FILE)" > $(VECTOR_SUBSET_FILE); \
	fi
	@elf_to_json_bin="$(MAKEFILE_DIR)bin/elf_to_json_gen"; \
	[ -x "$$elf_to_json_bin" ] || { echo "ELF-to-JSON helper not found. Run vector-build first"; exit 1; }; \
	if [ "$(VECTOR_JSON_MODE)" = "batched" ]; then \
		echo "rendering ZKC JSON to $(VECTOR_JSON_FILE)"; \
		"$$elf_to_json_bin" $(BIN) @$(VECTOR_SUBSET_FILE) $(IN_ORIGIN) > $(VECTOR_JSON_FILE) || exit $$?; \
	elif [ "$(VECTOR_JSON_MODE)" = "per-vector" ]; then \
		mkdir -p $(VECTOR_JSON_DIR); \
		rm -f $(VECTOR_JSON_DIR)/*.json; \
		echo "rendering one ZKC JSON per vector under $(VECTOR_JSON_DIR)"; \
		n=0; while IFS= read -r in_bytes; do \
			n=$$((n + 1)); idx=$$(printf '%03d' $$n); \
			"$$elf_to_json_bin" $(BIN) "$$in_bytes" $(IN_ORIGIN) > "$(VECTOR_JSON_DIR)/$$idx.json" || exit $$?; \
		done < $(VECTOR_SUBSET_FILE); \
		echo "generated $$n vector JSON(s) in $(VECTOR_JSON_DIR)"; \
	fi

vector-exec: require-vector-json-mode install-zkc
	@if [ "$(VECTOR_JSON_MODE)" = "batched" ]; then \
		[ -f $(VECTOR_JSON_FILE) ] || { echo "VECTOR_JSON_FILE not found: $(VECTOR_JSON_FILE). Run vector-json first"; exit 1; }; \
		echo "RUN   $(VECTOR_JSON_FILE)"; \
		err="$(VECTOR_JSON_FILE).err"; \
		zkc exec -q $(VECTOR_JSON_FILE) $(ZKC_MAIN) > /dev/null 2> "$$err"; \
		status=$$?; \
		if [ "$$status" -eq 0 ] && [ ! -s "$$err" ]; then \
			echo "PASS"; \
			rm -f "$$err"; \
		else \
			echo "FAIL"; \
			if [ ! -s "$$err" ]; then echo "zkc exited with code $$status" > "$$err"; fi; \
			cat "$$err"; \
			exit 1; \
		fi; \
	elif [ "$(VECTOR_JSON_MODE)" = "per-vector" ]; then \
		pass=0; fail=0; i=0; \
		for json in $(VECTOR_JSON_DIR)/*.json; do \
			[ -f "$$json" ] || { echo "error: no vector JSONs found in $(VECTOR_JSON_DIR)" >&2; exit 1; }; \
			i=$$((i + 1)); \
			echo "RUN   case $$i  $$json"; \
			err="$$json.err"; \
			zkc exec -q "$$json" $(ZKC_MAIN) > /dev/null 2> "$$err"; \
			status=$$?; \
			if [ "$$status" -eq 0 ] && [ ! -s "$$err" ]; then \
				echo "PASS"; \
				rm -f "$$err"; \
				pass=$$((pass + 1)); \
			else \
				echo "FAIL"; \
				if [ ! -s "$$err" ]; then echo "zkc exited with code $$status" > "$$err"; fi; \
				cat "$$err"; \
				fail=$$((fail + 1)); \
			fi; \
		done; \
		echo "DONE: $$i vectors   PASS: $$pass   FAIL: $$fail"; \
		if [ "$$fail" -ne 0 ]; then echo "error: $$fail vector(s) failed" >&2; exit 1; fi; \
	fi

#####################################################################
# Keccak wrapper targets
#####################################################################

KECCAK_ALL_FILE       ?= $(MAKEFILE_DIR)rust/src/keccak/keccak.all
KECCAK_N_VECTORS      ?= 10
KECCAK_RUST_N_VECTORS = $(if $(filter -1,$(KECCAK_N_VECTORS)),$(shell awk 'END { print NR }' $(KECCAK_ALL_FILE)),$(KECCAK_N_VECTORS))
KECCAK_JSON_FILE      ?= $(MAKEFILE_DIR)rust/target/riscv64im-unknown-none-elf/release/keccak.json

keccak-rust-build:
	cargo clean -p riscv-tests --manifest-path $(MAKEFILE_DIR)rust/Cargo.toml
	KECCAK_N_VECTORS=$(KECCAK_RUST_N_VECTORS) $(MAKE) -f $(MAKEFILE_DIR)Makefile vector-build \
		TEST=keccak/keccak.rs \
		IN_BYTES=0x00

keccak-rust-json: keccak-rust-build
	$(MAKE) -f $(MAKEFILE_DIR)Makefile vector-json \
		TEST=keccak/keccak.rs \
		VECTOR_FILE=$(KECCAK_ALL_FILE) \
		VECTOR_N_VECTORS=$(KECCAK_N_VECTORS) \
		VECTOR_JSON_MODE=batched \
		VECTOR_JSON_FILE=$(KECCAK_JSON_FILE)

keccak-rust-exec: keccak-rust-json
	$(MAKE) -f $(MAKEFILE_DIR)Makefile vector-exec VECTOR_JSON_MODE=batched VECTOR_JSON_FILE=$(KECCAK_JSON_FILE)

#####################################################################
# Blake wrapper targets
#####################################################################

BLAKE_ALL_FILE  ?= $(MAKEFILE_DIR)rust/src/blake/blake10.all
BLAKE_N_VECTORS ?= -1
BLAKE_JSON_DIR  ?= $(MAKEFILE_DIR)rust/target/riscv64im-unknown-none-elf/release/blake_json

blake-rust-build:
	$(MAKE) -f $(MAKEFILE_DIR)Makefile vector-build \
		TEST=blake/blake_with_in_bytes.rs

blake-rust-json: blake-rust-build
	$(MAKE) -f $(MAKEFILE_DIR)Makefile vector-json \
		TEST=blake/blake_with_in_bytes.rs \
		VECTOR_FILE=$(BLAKE_ALL_FILE) \
		VECTOR_N_VECTORS=$(BLAKE_N_VECTORS) \
		VECTOR_JSON_MODE=per-vector \
		VECTOR_JSON_DIR=$(BLAKE_JSON_DIR)

blake-rust-exec: blake-rust-json
	$(MAKE) -f $(MAKEFILE_DIR)Makefile vector-exec VECTOR_JSON_MODE=per-vector VECTOR_JSON_DIR=$(BLAKE_JSON_DIR)

#####################################################################
# ACT4 targets
#####################################################################

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

# Build the ACT4 self-checking ELFs with the Linea ACT4 config 
# Note: 
#	- clone https://github.com/riscv/riscv-arch-test.git if needed
#	- ELFs are built at every invocation (in case Linea ACT4 config changes)
#   - JSONs are generated from the ELFs at every invocation
act4-build:
	@mkdir -p "$(ACT4_WORK_DIR)" "$(ACT4_LOGS)" || exit $$?; \
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
	fi; \
	rm -f "$(ACT4_LOGS)"/*.json "$(ACT4_LOGS)"/*.json.err "$(ACT4_LOGS)"/*.err "$(ACT4_LOGS)"/*.out "$(ACT4_LOGS)"/*.full || exit $$?; \
	elf_to_json_bin="$(MAKEFILE_DIR)bin/elf_to_json_gen"; \
	mkdir -p "$(MAKEFILE_DIR)bin" || exit $$?; \
	echo "building $$elf_to_json_bin..."; \
	go build -o "$$elf_to_json_bin" $(ELF_TO_JSON_GEN) || exit $$?; \
	count=0; \
	for elf in $$(find "$(ACT4_ELF_DIR)" -name '*.elf' | sort); do \
		name=$$(basename "$$elf" .elf); \
		base="$(ACT4_LOGS)/$$name"; \
		count=$$((count + 1)); \
		"$$elf_to_json_bin" "$$elf" "" 0 > "$$base.json" 2> "$$base.json.err" || exit $$?; \
		[ -s "$$base.json.err" ] || rm -f "$$base.json.err"; \
	done; \
	if [ "$$count" -eq 0 ]; then echo "error: no ACT4 ELFs found in $(ACT4_ELF_DIR)" >&2; exit 1; fi; \
	echo "generated $$count ACT4 JSON input(s) in $(ACT4_LOGS)"

# Run the ACT4 self-checking JSON inputs generated by act4-build.
act4-exec: install-zkc act4-build
	@rm -f "$(ACT4_LOGS)"/*.err "$(ACT4_LOGS)"/*.out "$(ACT4_LOGS)"/*.full; \
	> "$(ACT4_RESULTS)"; \
	count=0; pass=0; fail=0; \
	for json in $$(find "$(ACT4_LOGS)" -name '*.json' | sort); do \
		name=$$(basename "$$json" .json); \
		base="$(ACT4_LOGS)/$$name"; \
		count=$$((count + 1)); \
		echo "RUN   $$name  $$json"; \
		zkc exec -q "$$json" $(ZKC_MAIN) > /dev/null 2> "$$base.err"; \
		status=$$?; \
		if [ "$$status" -eq 0 ] && [ ! -s "$$base.err" ]; then \
			echo "PASS"; \
			echo "PASS  $$name" >> "$(ACT4_RESULTS)"; \
			pass=$$((pass + 1)); \
			rm -f "$$base.err"; \
		else \
			echo "FAIL"; \
			echo "FAIL  $$name" >> "$(ACT4_RESULTS)"; \
			if [ ! -s "$$base.err" ]; then echo "zkc exited with code $$status" > "$$base.err"; fi; \
			cat "$$base.err"; \
			cat "$$base.err" >> "$(ACT4_RESULTS)"; \
			fail=$$((fail + 1)); \
		fi; \
	done; \
	echo "DONE: $$count tests   PASS: $$pass   FAIL: $$fail" | tee -a "$(ACT4_RESULTS)"; \
	echo "results: $(ACT4_RESULTS)"; \
	echo "logs:    $(ACT4_LOGS)/"; \
	if [ "$$count" -eq 0 ]; then echo "error: no ACT4 JSON inputs found in $(ACT4_LOGS)" >&2; exit 1; fi; \
	if [ "$$fail" -ne 0 ]; then echo "error: $$fail of $$count ACT4 test(s) failed" >&2; exit 1; fi

#####################################################################
# Support targets to validate mandatory parameters
#####################################################################

require-test:
	@[ -n "$(TEST)" ] || { echo "TEST is mandatory. Usage: make TEST=src_optional_subfolder/your_test(.s|.zig|.rs)"; exit 1; }

require-src: require-test
	@[ -f $(SRC) ] || { echo "TEST file not found: $(SRC)"; exit 1; }

require-bin-ext:
	@[ -n "$(BIN_EXT)" ] || { echo "BIN_EXT is mandatory. Usage: make BIN_EXT=path/to/your_bin"; exit 1; }
	@[ -f $(BIN_EXT) ] || { echo "BIN_EXT file not found: $(BIN_EXT)"; exit 1; }

require-vector-file:
	@[ -n "$(VECTOR_FILE)" ] || { echo "VECTOR_FILE is mandatory. Usage: make vector-json TEST=src_optional_subfolder/your_test(.s|.zig|.rs) VECTOR_FILE=path/to/vectors"; exit 1; }
	@[ -f $(VECTOR_FILE) ] || { echo "VECTOR_FILE file not found: $(VECTOR_FILE)"; exit 1; }

require-n-vectors:
	@[ -n "$(VECTOR_N_VECTORS)" ] || { echo "VECTOR_N_VECTORS is mandatory. Use -1 for all vectors or a positive integer"; exit 1; }
	@if [ "$(VECTOR_N_VECTORS)" != "-1" ]; then \
		[ "$(VECTOR_N_VECTORS)" -gt 0 ] 2>/dev/null || { echo "VECTOR_N_VECTORS must be -1 or a positive integer, got: $(VECTOR_N_VECTORS)"; exit 1; }; \
	fi

require-vector-build-artifacts:
	@[ -f $(BIN) ] || { echo "BIN not found: $(BIN). Run vector-build first"; exit 1; }

require-vector-json-mode:
	@[ "$(VECTOR_JSON_MODE)" = "per-vector" ] || [ "$(VECTOR_JSON_MODE)" = "batched" ] || { echo "VECTOR_JSON_MODE must be per-vector or batched, got: $(VECTOR_JSON_MODE)"; exit 1; }
