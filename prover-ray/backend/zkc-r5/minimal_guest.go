package zkcr5

import minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"

// ExitZeroGuestELF is a tiny valid RISC-V ELF that exits with code 0 through
// the Linux-style syscall path modeled by arithmetization/src/main/riscv/main.zkc.
var ExitZeroGuestELF = minimalelf.ExitZeroElfProgram

// MemoryRoundTripGuestELF is a tiny valid RISC-V ELF that stores and loads a
// scratch word, then exits with code 0 through the same Linux-style syscall
// path as ExitZeroGuestELF (or code 1 if the round-tripped value doesn't
// match, which would indicate a bug in the S-type/I-type memory path).
var MemoryRoundTripGuestELF = minimalelf.MemoryRoundTripElfProgram
