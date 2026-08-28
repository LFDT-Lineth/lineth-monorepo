package zkcr5

// DefaultINOrigin is the canonical _in_start RAM address for all Lineth guest
// programs: the address where the input data is mapped before the VM starts
// executing.
//
// Source: riscv-guests/build_common/Makefile, IN_ORIGIN = 0x08800000.
const DefaultINOrigin uint64 = 0x08800000
