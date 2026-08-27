package zkcr5

import minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/internal/minimal-elf"

// AllInOneGuestELF is a tiny valid RISC-V ELF that exercises the full RV64I
// + M-extension + custom-precompile surface used by
// arithmetization/src/main/riscv/main.zkc, then exits with code 0 through
// the Linux-style syscall path (or code 1 if any check disagrees with its
// independently-computed expected value).
var AllInOneGuestELF = minimalelf.AllInOneElfProgram
