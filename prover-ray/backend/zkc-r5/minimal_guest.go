package zkcr5

import minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"

// ExitZeroGuestELF is a tiny valid RISC-V ELF that exits with code 0 through
// the Linux-style syscall path modeled by arithmetization/src/main/riscv/main.zkc.
var ExitZeroGuestELF = minimalelf.ExitZeroElfProgram
