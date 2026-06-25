.PHONY: linker-script verify-elf

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
LINKER_SCRIPT   := $(MAKEFILE_DIR)linker_script.ld

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
