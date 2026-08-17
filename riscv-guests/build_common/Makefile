.PHONY: linker-script elf-to-json

BUILD_COMMON_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
LINKER_SCRIPT    := $(BUILD_COMMON_DIR)linker_script.ld

# Configurable parameters for the linker script
STACK_ORIGIN   ?= 0x00000000
SP             ?= 0x00800000
PROGRAM_ORIGIN ?= 0x00800000
IN_ORIGIN      ?= 0x08800000
HEAP_ORIGIN    ?= 0x48800000

# Support function to evaluate expressions and format them as hex
EVAL = $(shell printf "0x%08x" $$(( $(1) )))

# Derived parameters for the linker script
ENTRY_POINT    := $(PROGRAM_ORIGIN)
STACK_LENGTH   := $(call EVAL,$(SP) - $(STACK_ORIGIN))
PROGRAM_LENGTH := $(call EVAL,$(IN_ORIGIN) - $(PROGRAM_ORIGIN))
IN_LENGTH      := $(call EVAL,$(HEAP_ORIGIN) - $(IN_ORIGIN))

# ── ELF → ZKC JSON conversion ────────────────────────────────────────────────
# convert a statically-linked rv64im ELF to the ZKC JSON the interpreter loads
ELF_TO_JSON_GEN := $(BUILD_COMMON_DIR)../../arithmetization/src/test/scripts/elf_to_json_gen/main.go

# Hex big-endian input or `@path/to/in_bytes`
IN_BYTES ?= ""

# Already-compiled external ELF to convert (and its derived output JSON path)
BIN_EXT  ?=
JSON_EXT ?= $(BIN_EXT).json

# Support function to generate JSON from an ELF: $(call ELF_TO_JSON,<elf>,<out.json>)
define ELF_TO_JSON
	@echo "\nGenerating $(2) from $(1)"
	@go run $(ELF_TO_JSON_GEN) "$(1)" "$(IN_BYTES)" $(IN_ORIGIN) > "$(2)"
endef

# The following variables
# STACK_ORIGIN
# SP
# PROGRAM_ORIGIN
# IN_ORIGIN
# HEAP_ORIGIN
# in arithmetization/src/test/Makefile have default values but can be overriden.
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

# Convert an already-compiled external ELF to a ZKC JSON (BIN_EXT in, JSON_EXT out)
elf-to-json:
	@[ -n "$(BIN_EXT)" ] || { echo "BIN_EXT is mandatory. Usage: make BIN_EXT=path/to/your_bin"; exit 1; }
	@[ -f $(BIN_EXT) ] || { echo "BIN_EXT file not found: $(BIN_EXT)"; exit 1; }
	$(call ELF_TO_JSON,$(BIN_EXT),$(JSON_EXT))
