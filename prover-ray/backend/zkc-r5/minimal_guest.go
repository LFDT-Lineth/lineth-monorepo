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

// ArithmeticGuestELF is a tiny valid RISC-V ELF that exercises LUI, JAL,
// JALR, an R-type base op, and an M-extension op, then exits with code 0
// through the same Linux-style syscall path as ExitZeroGuestELF (or code 1
// if the two independently-computed results disagree).
var ArithmeticGuestELF = minimalelf.ArithmeticElfProgram

// ExitOneGuestELF is a tiny valid RISC-V ELF that exits with a nonzero code
// through the guest syscall interface. main.zkc's process_syscall must
// reject this trace rather than accept it.
var ExitOneGuestELF = minimalelf.ExitOneElfProgram

// BranchesGuestELF is a tiny valid RISC-V ELF that exercises all six B-type
// variants (BEQ, BNE, BLT, BGE, BLTU, BGEU), both taken and not-taken,
// then exits with code 0 (or code 1 if any variant misbehaves).
var BranchesGuestELF = minimalelf.BranchesElfProgram

// LoadStoreWidthsGuestELF is a tiny valid RISC-V ELF that exercises the
// SB/LB, SH/LH, and SD/LD memory widths (including sign extension), then
// exits with code 0 (or code 1 if any round trip disagrees).
var LoadStoreWidthsGuestELF = minimalelf.LoadStoreWidthsElfProgram

// Poseidon2GuestELF is a tiny valid RISC-V ELF that invokes the
// R_POSEIDON2 precompile over an all-zero input block, then exits with
// code 0 (or code 1 if the output disagrees with the known-good vector).
var Poseidon2GuestELF = minimalelf.Poseidon2ElfProgram

// KeccakGuestELF is a tiny valid RISC-V ELF that invokes the R_KECCAK
// precompile over an empty message, then exits with code 0 (or code 1 if
// the digest disagrees with the well-known Keccak-256 empty-string digest).
var KeccakGuestELF = minimalelf.KeccakElfProgram

// WriteOutputGuestELF is a tiny valid RISC-V ELF that invokes the
// R_WRITE_OUTPUT precompile to copy 3 known bytes into the public
// guest_output buffer, then exits with code 0.
var WriteOutputGuestELF = minimalelf.WriteOutputElfProgram

// ImmediateALUGuestELF is a tiny valid RISC-V ELF that exercises SLTI,
// SLTIU, XORI, ORI, ANDI, SLLI, SRLI, SRAI, ADDIW, SLLIW, SRLIW, and
// SRAIW, then exits with code 0 (or code 1 if any result disagrees with
// its independently-computed expected value).
var ImmediateALUGuestELF = minimalelf.ImmediateALUElfProgram

// WordWidthGuestELF is a tiny valid RISC-V ELF that exercises ADDW, SUBW,
// SLLW, SRLW, and SRAW, then exits with code 0 (or code 1 if any result
// disagrees with its independently-computed expected value).
var WordWidthGuestELF = minimalelf.WordWidthElfProgram
