# For the target we refer to https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md.
# Note that Zicclsm extension does not affect the generated ELF so it is omitted.
# Moreover, ABI being LP64 (soft-float) is relevant only for float numbers, which we do not use, so it can be omitted as well.

# Declare all targets as phony to prevent conflicts with files of the same name
.PHONY: exec debug compile zkc-exec zkc-debug clean clean-all linker-script verify-elf blake-all build-act4 run-act4

# Directory of this Makefile (used for relative paths)
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Test to run (with src optional subfolder and extension), mandatory unless clean-all or linker-script are the targets
# src_optional_subfolder/your_test(.s|.zig|.rs)
ifeq ($(filter clean-all linker-script blake-all build-act4 run-act4,$(MAKECMDGOALS)),)
ifndef TEST
$(error TEST is mandatory. Usage: make TEST=src_optional_subfolder/your_test(.s|.zig|.rs))
endif
endif

# src_optionaL_subfolder/your_test
BASENAME     := $(basename $(TEST))

# your_test
NAME 		 := $(notdir $(BASENAME))

# src_optional_subfolder/
SUBDIR       := $(dir $(TEST))

# your_test(.s|.zig|.rs)
NOTSUBDIR    := $(notdir $(TEST))

# (.s|.zig|.rs)
EXT          := $(suffix $(TEST))

# Paths below are determined by the extension
SRC  := $(MAKEFILE_DIR)$(if $(filter .s,$(EXT)),asm,$(if $(filter .zig,$(EXT)),zig,rust))/src/$(TEST)
BIN  := $(MAKEFILE_DIR)$(if $(filter .s,$(EXT)),asm/bin/,$(if $(filter .zig,$(EXT)),zig/zig-out/bin/,rust/target/riscv64im-unknown-none-elf/release/))$(NAME)
JSON := $(MAKEFILE_DIR)$(if $(filter .s,$(EXT)),asm/bin/,$(if $(filter .zig,$(EXT)),zig/zig-out/bin/,rust/target/riscv64im-unknown-none-elf/release/))$(NAME).json
# Note: in the future the JSON file will be removed and main.zkc will receive directly the ELF file

# Paths to go main (to turn BIN into JSON), zkc main (to execute/debug the json) and linker script
GO_MAIN       := $(MAKEFILE_DIR)main.go
ZKC_MAIN      := $(MAKEFILE_DIR)../../main/riscv/main.zkc
LINKER_SCRIPT := $(MAKEFILE_DIR)linker_script.ld

# Inputs and offsets for the program
IN_BYTES        ?= ""

# Values to generate the linker script

# 128 MiB (0x00000000 -> 0x07FFFFFF)
PROGRAM_OFFSET  ?= 0x00000000

# 1 GiB   (0x08800000 -> 0x48FFFFFF)
IN_BYTES_OFFSET ?= 0x08800000

# Since program starts from _start, the entry point is the same as the program offset
ENTRY_POINT     := $(PROGRAM_OFFSET)

# 8 MiB   (0x08000000 <- 0x08800000)
SP              ?= 0x08800000

# Parsed values are just to verify that the generate ELF is correct (lazy evaluation allows to parse the ELF only after it is generated)
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

# Default target (make)
exec: compile zkc-exec

debug: compile zkc-debug

compile: $(SRC) linker-script
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
	@echo "\nDisassembling $(BIN)"
	@LANG=C riscv64-unknown-elf-objdump -D -s --line-numbers $(BIN) > $(BIN).objdump
# Verify that PROGRAM_OFFSET, IN_BYTES_OFFSET, ENTRY_POINT, SP match the ELF values
	@if [ "$(VERIFY_ELF)" = "true" ]; then \
		$(MAKE) -f $(MAKEFILE_DIR)Makefile verify-elf TEST=$(TEST); \
	fi
# Generate JSON from ELF
	@echo "\nGenerating $(JSON) from $(BIN)"
	@go run $(GO_MAIN) $(BIN) "$(IN_BYTES)" $(PROGRAM_OFFSET) $(IN_BYTES_OFFSET) $(ENTRY_POINT) > $(JSON)

# Note: compiled Rust libraries may contain instructions from excluded extensions. For instance, the core library is compiled
# for the IMAC extension, and contains COMPRESSED instructions. The output ELF may thus contain such instructions, too, even
# if these extensions were explicitly exluded from the target.
#
# Note: we use opt-level=2 to remove unused code.

zkc-exec:
	zkc exec $(JSON) $(ZKC_MAIN)

zkc-debug:
	zkc debug $(JSON) $(ZKC_MAIN)

clean:
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
verify-elf: $(BIN)
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

# Run all blake test vectors in `rust/src/blake/blake.all`
blake-all:
	@i=0; while IFS= read -r in_bytes; do \
		i=$$((i + 1)); \
		output=$$(make -f $(MAKEFILE_DIR)Makefile TEST=blake/blake_with_in_bytes.rs IN_BYTES="$$in_bytes" 2>&1); \
		result_line=$$(printf '%s\n' "$$output" | sed '/^[[:space:]]*$$/d' | tail -n 2 | head -n 1); \
		echo "case $$i: $$result_line"; \
	done < $(MAKEFILE_DIR)rust/src/blake/blake.all

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

# Build and run the ACT4 self-checking ELFs with the Linea ACT4 config
run-act4: build-act4
	@rm -f "$(ACT4_LOGS)"/*; \
	mkdir -p "$(ACT4_LOGS)"; \
	> "$(ACT4_RESULTS)"; \
	count=0; pass=0; fail=0; \
	for elf in $$(find "$(ACT4_ELF_DIR)" -name '*.elf' | sort); do \
		name=$$(basename "$$elf" .elf); \
		base="$(ACT4_LOGS)/$$name"; \
		count=$$((count + 1)); \
		go run $(GO_MAIN) "$$elf" "" "$(PROGRAM_OFFSET)" 0 "$(ENTRY_POINT)" > "$$base.json" 2> "$$base.json.err"; \
		[ -s "$$base.json.err" ] || rm -f "$$base.json.err"; \
		zkc exec "$$base.json" $(ZKC_MAIN) > "$$base.full" 2>&1; \
		awk 'previous == "ECALL for write" || previous == "ECALL for exit" { print } { previous = $$0 }' "$$base.full" > "$$base.out"; \
		exit_line=$$(awk 'previous == "ECALL for exit" { print; exit } { previous = $$0 }' "$$base.full"); \
		if [ "$$exit_line" = "Program exited successfully (exit with code 0)." ]; then \
			echo "PASS  $$name"; \
			echo "PASS  $$name" >> "$(ACT4_RESULTS)"; \
			awk '{ print "  " $$0 }' "$$base.out"; \
			awk '{ print "  " $$0 }' "$$base.out" >> "$(ACT4_RESULTS)"; \
			pass=$$((pass + 1)); \
			rm -f "$$base.full"; \
		else \
			echo "FAIL  $$name"; \
			echo "FAIL  $$name" >> "$(ACT4_RESULTS)"; \
			awk '{ print "  " $$0 }' "$$base.out"; \
			awk '{ print "  " $$0 }' "$$base.out" >> "$(ACT4_RESULTS)"; \
			fail=$$((fail + 1)); \
		fi; \
	done; \
	echo "DONE: $$count tests   PASS: $$pass   FAIL: $$fail" | tee -a "$(ACT4_RESULTS)"; \
	echo "results: $(ACT4_RESULTS)"; \
	echo "logs:    $(ACT4_LOGS)/"
