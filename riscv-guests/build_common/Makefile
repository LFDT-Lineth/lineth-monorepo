.PHONY: linker-script

MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
# Makefile containing the logic for generating a linker script, executing and debugging generic programs
ARITHMETIZATION_TESTING_MAKEFILE := $(abspath $(MAKEFILE_DIR)../../arithmetization/src/test)/Makefile

# The following variables
# STACK_ORIGIN
# SP
# PROGRAM_ORIGIN
# IN_ORIGIN
# HEAP_ORIGIN
# in arithmetization/src/test/Makefile have default values but can be overriden.
linker-script:
	$(MAKE) -f $(ARITHMETIZATION_TESTING_MAKEFILE) linker-script LINKER_SCRIPT=$(MAKEFILE_DIR)/linker_script.ld

build-common: linker-script