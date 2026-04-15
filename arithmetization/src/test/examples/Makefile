# Usage (from anywhere):
#   make -f /path/to/linea-monorepo/arithmetization/src/test/examples/Makefile TEST=your_test
# Or:
#   cd arithmetization/src/test/examples && make TEST=your_test
# Optional shell alias:
#   alias zkctest='make -f /path/to/linea-monorepo/arithmetization/src/test/examples/Makefile'

# define the assembler to use
AS = riscv64-unknown-elf-as
AS_FLAGS = -march=rv64im

# Directory of this Makefile (used for relative paths)
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Name of the test to run (without extension)
ifndef TEST
$(error TEST is mandatory. Usage: make TEST=your_test)
endif

ASM ?= $(MAKEFILE_DIR)asm/src/$(TEST).s
BIN ?= $(MAKEFILE_DIR)asm/bin/$(TEST)
JSON ?= $(MAKEFILE_DIR)asm/bin/$(TEST).json
GO_MAIN ?= $(MAKEFILE_DIR)main.go
ZKC_MAIN ?= $(abspath $(MAKEFILE_DIR)../../main/riscv/main.zkc)

# default target (make)
exec: compile zkc-exec

debug: compile zkc-debug

compile: $(ASM)
	@mkdir -p $(MAKEFILE_DIR)asm/bin
	$(AS) ${AS_FLAGS} -o $(BIN) $^
	go run $(GO_MAIN) $(BIN) > $(JSON)

zkc-exec:
	zkc exec $(JSON) $(ZKC_MAIN)

zkc-debug:
	zkc debug $(JSON) $(ZKC_MAIN)

clean:
	rm $(BIN) $(JSON)

clean-all:
	rm -rf $(MAKEFILE_DIR)asm/bin/
