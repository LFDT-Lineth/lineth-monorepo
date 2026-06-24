package main

import (
	"debug/elf"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	ENTRY_POINT_AND_BLOBS_COUNT = "entry_point_and_blobs_count"
	BLOBS_OFFSET_AND_SIZE       = "blobs_offset_and_size"
	BLOBS_DATA                  = "blobs_data"
	INSTRUCTION_BASE            = "instruction_base"
	DECODED                     = "decoded"
)

// Instruction type identifiers. These MUST match the Type constants in
// arithmetization/src/main/riscv/utils/constants.zkc.
const (
	undefinedType = 0
	rType         = 1
	iType         = 2
	sType         = 3
	bType         = 4
	uType         = 5
	jType         = 6
	miscMemType   = 7
)

// RISC-V opcodes (low 7 bits), mirroring the Opcode constants in constants.zkc.
const (
	opcodeOP      = 0b0110011
	opcodeOP32    = 0b0111011
	opcodeLOAD    = 0b0000011
	opcodeOPIMM   = 0b0010011
	opcodeOPIMM32 = 0b0011011
	opcodeJALR    = 0b1100111
	opcodeSYSTEM  = 0b1110011
	opcodeMISCMEM = 0b0001111
	opcodeSTORE   = 0b0100011
	opcodeBRANCH  = 0b1100011
	opcodeLUI     = 0b0110111
	opcodeAUIPC   = 0b0010111
	opcodeJAL     = 0b1101111
	opcodeCUSTOM1 = 0b0101011
)

// defaultMaxDecodedRecords caps the number of pre-decoded instruction records
// (one per 4-byte word across the executable span). It guards against a
// non-contiguous executable layout causing a giant dense table (and an OOM).
// Overridable via the ELF2JSON_MAX_DECODED_RECORDS environment variable.
const defaultMaxDecodedRecords = 2_000_000

// instructionTypeFromOpcode mirrors instruction_type_from_opcode in
// constants.zkc.
func instructionTypeFromOpcode(opcode uint32) uint32 {
	switch opcode {
	case opcodeOP, opcodeOP32:
		return rType
	case opcodeLOAD, opcodeOPIMM, opcodeOPIMM32, opcodeJALR, opcodeSYSTEM:
		return iType
	case opcodeSTORE:
		return sType
	case opcodeBRANCH:
		return bType
	case opcodeLUI, opcodeAUIPC:
		return uType
	case opcodeJAL:
		return jType
	case opcodeMISCMEM:
		return miscMemType
	default:
		return undefinedType
	}
}

// isRdZeroNoop reports whether an instruction only discards its result into x0
// and has no other architecturally visible effects (no memory access, no
// control-flow change, no non-x0 register reads). Such slots are rewritten to
// MISC_MEM_TYPE at pre-decode time so the interpreter only advances PC by 4.
func isRdZeroNoop(opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7 uint32) bool {
	if rd != 0 {
		return false
	}
	switch instrType {
	case rType:
		// Custom-1 includes the Keccak precompile, which has memory side effects.
		if opcode == opcodeCUSTOM1 {
			return false
		}
		return rs1 == 0 && rs2 == 0
	case uType:
		return opcode == opcodeLUI
	case iType:
		switch opcode {
		case opcodeLOAD, opcodeJALR, opcodeSYSTEM:
			return false
		case opcodeOPIMM:
			if rs1 != 0 {
				return false
			}
			return opImmEncodingSupported(funct3, imm12)
		case opcodeOPIMM32:
			if rs1 != 0 {
				return false
			}
			return opImm32EncodingSupported(funct3, imm12)
		default:
			return false
		}
	default:
		return false
	}
}

// opImmEncodingSupported mirrors the OP-IMM funct3 arms implemented in i_type.zkc.
func opImmEncodingSupported(funct3, imm12 uint32) bool {
	funct6 := (imm12 >> 6) & 0x3f
	switch funct3 {
	case 0b000, 0b010, 0b011, 0b100, 0b110, 0b111: // ADDI, SLTI, SLTIU, XORI, ORI, ANDI
		return true
	case 0b001: // SLLI
		return funct6 == 0b000000
	case 0b101: // SRLI / SRAI
		return funct6 == 0b000000 || funct6 == 0b010000
	default:
		return false
	}
}

// opImm32EncodingSupported mirrors the OP-IMM-32 arms implemented in i_type.zkc.
func opImm32EncodingSupported(funct3, imm12 uint32) bool {
	funct7FromImm := (imm12 >> 5) & 0x7f
	switch funct3 {
	case 0b000: // ADDIW
		return true
	case 0b001: // SLLIW
		return funct7FromImm == 0b0000000
	case 0b101: // SRLIW / SRAIW
		return funct7FromImm == 0b0000000 || funct7FromImm == 0b0100000
	default:
		return false
	}
}

// I-type semantic micro-op constants. These MUST match constants.zkc.
const (
	itypeRead8Sgn   = 0
	itypeRead16Sgn  = 1
	itypeRead32Sgn  = 2
	itypeRead64     = 3
	itypeRead8Zext  = 4
	itypeRead16Zext = 5
	itypeRead32Zext = 6

	itypeOpAddi  = 7
	itypeOpSlti  = 8
	itypeOpSltiu = 9
	itypeOpXori  = 10
	itypeOpOri   = 11
	itypeOpAndi  = 12
	itypeOpSlli  = 13
	itypeOpSrli  = 14
	itypeOpSrai  = 15

	itypeOpAddiw = 16
	itypeOpSlliw = 17
	itypeOpSrliw = 18
	itypeOpSraiw = 19

	itypeJalr    = 20
	itypeEcall   = 21
	itypeEbreak  = 22
	itypeInvalid = 63

	wbNone     = 0
	wbStoreReg = 1
)

// R-type semantic micro-op constants. These MUST match constants.zkc.
const (
	rtypeOpAdd  = 0
	rtypeOpSub  = 1
	rtypeOpSll  = 2
	rtypeOpSlt  = 3
	rtypeOpSltu = 4
	rtypeOpXor  = 5
	rtypeOpSrl  = 6
	rtypeOpSra  = 7
	rtypeOpOr   = 8
	rtypeOpAnd  = 9

	rtypeOpMul    = 10
	rtypeOpMulh   = 11
	rtypeOpMulhsu = 12
	rtypeOpMulhu  = 13
	rtypeOpDiv    = 14
	rtypeOpDivu   = 15
	rtypeOpRem    = 16
	rtypeOpRemu   = 17

	rtypeOpAddw = 18
	rtypeOpSubw = 19
	rtypeOpSllw = 20
	rtypeOpSrlw = 21
	rtypeOpSraw = 22

	rtypeOpMulw  = 23
	rtypeOpDivw  = 24
	rtypeOpDivuw = 25
	rtypeOpRemw  = 26
	rtypeOpRemuw = 27

	rtypeOpKeccak = 28
	rtypeInvalid  = 63
)

const (
	funct12Ecall  = 0b000000000000
	funct12Ebreak = 0b000000000001
)

// Unified instruction-model constants for the zisk-style pipeline. These MUST
// match the OPR_*/A_*/B_*/RK_*/WK_*/PK_* constants in constants.zkc.
const (
	oprAdd        = 0
	oprSub        = 1
	oprSll        = 2
	oprSlt        = 3
	oprSltu       = 4
	oprXor        = 5
	oprSrl        = 6
	oprSra        = 7
	oprOr         = 8
	oprAnd        = 9
	oprAddw       = 10
	oprSubw       = 11
	oprSllw       = 12
	oprSrlw       = 13
	oprSraw       = 14
	oprMul        = 15
	oprMulh       = 16
	oprMulhsu     = 17
	oprMulhu      = 18
	oprDiv        = 19
	oprDivu       = 20
	oprRem        = 21
	oprRemu       = 22
	oprMulw       = 23
	oprDivw       = 24
	oprDivuw      = 25
	oprRemw       = 26
	oprRemuw      = 27
	oprMoveLoaded = 28
	oprMoveB      = 29
	oprLink       = 30
	oprCmpEq      = 31
	oprCmpNe      = 32
	oprCmpLt      = 33
	oprCmpGe      = 34
	oprCmpLtu     = 35
	oprCmpGeu     = 36
	oprKeccak     = 37
	oprNop        = 38
	oprInvalid    = 63

	aRS1 = 0
	aPC  = 1

	bRS2 = 0
	bIMM = 1

	rkNone = 0
	rk8S   = 1
	rk16S  = 2
	rk32S  = 3
	rk64   = 4
	rk8U   = 5
	rk16U  = 6
	rk32U  = 7

	wkNone  = 0
	wkReg   = 1
	wkMem8  = 2
	wkMem16 = 3
	wkMem32 = 4
	wkMem64 = 5

	pkNext    = 0
	pkBranch  = 1
	pkJumpRel = 2
	pkJumpAbs = 3
	pkSyscall = 4
	pkHalt    = 5
)

// decodeITypeSemantic maps a raw I-type encoding to a semantic compute op,
// writeback kind, and normalized immediate. Shift amounts are stripped to
// their low uimm6/uimm5 bits; funct6/funct7 validation happens here.
func decodeITypeSemantic(opcode, funct3, imm12 uint32) (computeOp, writeback, normalizedImm12 uint32) {
	funct6 := (imm12 >> 6) & 0x3f
	funct7FromImm := (imm12 >> 5) & 0x7f
	uimm6 := imm12 & 0x3f
	uimm5 := imm12 & 0x1f

	switch opcode {
	case opcodeLOAD:
		writeback = wbStoreReg
		switch funct3 {
		case 0b000:
			return itypeRead8Sgn, writeback, imm12
		case 0b001:
			return itypeRead16Sgn, writeback, imm12
		case 0b010:
			return itypeRead32Sgn, writeback, imm12
		case 0b011:
			return itypeRead64, writeback, imm12
		case 0b100:
			return itypeRead8Zext, writeback, imm12
		case 0b101:
			return itypeRead16Zext, writeback, imm12
		case 0b110:
			return itypeRead32Zext, writeback, imm12
		default:
			return itypeInvalid, wbNone, imm12
		}
	case opcodeOPIMM:
		writeback = wbStoreReg
		switch funct3 {
		case 0b000:
			return itypeOpAddi, writeback, imm12
		case 0b010:
			return itypeOpSlti, writeback, imm12
		case 0b011:
			return itypeOpSltiu, writeback, imm12
		case 0b100:
			return itypeOpXori, writeback, imm12
		case 0b110:
			return itypeOpOri, writeback, imm12
		case 0b111:
			return itypeOpAndi, writeback, imm12
		case 0b001:
			if funct6 != 0b000000 {
				return itypeInvalid, wbNone, imm12
			}
			return itypeOpSlli, writeback, uimm6
		case 0b101:
			switch funct6 {
			case 0b000000:
				return itypeOpSrli, writeback, uimm6
			case 0b010000:
				return itypeOpSrai, writeback, uimm6
			default:
				return itypeInvalid, wbNone, imm12
			}
		default:
			return itypeInvalid, wbNone, imm12
		}
	case opcodeOPIMM32:
		writeback = wbStoreReg
		switch funct3 {
		case 0b000:
			return itypeOpAddiw, writeback, imm12
		case 0b001:
			if funct7FromImm != 0b0000000 {
				return itypeInvalid, wbNone, imm12
			}
			return itypeOpSlliw, writeback, uimm5
		case 0b101:
			switch funct7FromImm {
			case 0b0000000:
				return itypeOpSrliw, writeback, uimm5
			case 0b0100000:
				return itypeOpSraiw, writeback, uimm5
			default:
				return itypeInvalid, wbNone, imm12
			}
		default:
			return itypeInvalid, wbNone, imm12
		}
	case opcodeJALR:
		return itypeJalr, wbStoreReg, imm12
	case opcodeSYSTEM:
		switch funct3 {
		case 0b000:
			switch imm12 {
			case funct12Ecall:
				return itypeEcall, wbNone, imm12
			case funct12Ebreak:
				return itypeEbreak, wbNone, imm12
			default:
				return itypeInvalid, wbNone, imm12
			}
		default:
			return itypeInvalid, wbNone, imm12
		}
	default:
		return itypeInvalid, wbNone, imm12
	}
}

// decodeRTypeSemantic maps a raw R-type encoding to a semantic compute op and
// writeback kind. funct3/funct7 validation happens here.
func decodeRTypeSemantic(opcode, funct3, funct7 uint32) (computeOp, writeback uint32) {
	switch opcode {
	case opcodeOP:
		if funct7 == 0b0000001 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpMul, writeback
			case 0b001:
				return rtypeOpMulh, writeback
			case 0b010:
				return rtypeOpMulhsu, writeback
			case 0b011:
				return rtypeOpMulhu, writeback
			case 0b100:
				return rtypeOpDiv, writeback
			case 0b101:
				return rtypeOpDivu, writeback
			case 0b110:
				return rtypeOpRem, writeback
			case 0b111:
				return rtypeOpRemu, writeback
			}
		} else if funct7 == 0b0000000 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpAdd, writeback
			case 0b001:
				return rtypeOpSll, writeback
			case 0b010:
				return rtypeOpSlt, writeback
			case 0b011:
				return rtypeOpSltu, writeback
			case 0b100:
				return rtypeOpXor, writeback
			case 0b101:
				return rtypeOpSrl, writeback
			case 0b110:
				return rtypeOpOr, writeback
			case 0b111:
				return rtypeOpAnd, writeback
			}
		} else if funct7 == 0b0100000 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpSub, writeback
			case 0b101:
				return rtypeOpSra, writeback
			}
		}
		return rtypeInvalid, wbNone
	case opcodeOP32:
		if funct7 == 0b0000001 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpMulw, writeback
			case 0b100:
				return rtypeOpDivw, writeback
			case 0b101:
				return rtypeOpDivuw, writeback
			case 0b110:
				return rtypeOpRemw, writeback
			case 0b111:
				return rtypeOpRemuw, writeback
			}
		} else if funct7 == 0b0000000 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpAddw, writeback
			case 0b001:
				return rtypeOpSllw, writeback
			case 0b101:
				return rtypeOpSrlw, writeback
			}
		} else if funct7 == 0b0100000 {
			writeback = wbStoreReg
			switch funct3 {
			case 0b000:
				return rtypeOpSubw, writeback
			case 0b101:
				return rtypeOpSraw, writeback
			}
		}
		return rtypeInvalid, wbNone
	case opcodeCUSTOM1:
		if funct3 == 0b000 && funct7 == 0b0000000 {
			return rtypeOpKeccak, wbNone
		}
		return rtypeInvalid, wbNone
	default:
		return rtypeInvalid, wbNone
	}
}

// unifiedRecord is the single pre-decoded record per instruction consumed by the
// interpreter's zisk-style pipeline. Field order/widths MUST match the `decoded`
// pub input in memory.zkc.
type unifiedRecord struct {
	operation uint32
	aSrc      uint32
	bSrc      uint32
	readKind  uint32
	writeKind uint32
	pcKind    uint32
	rs1       uint32
	rs2       uint32
	rd        uint32
	imm       uint64
}

// sext64 sign-extends the low `bits` bits of value to 64 bits.
func sext64(value uint32, bits uint) uint64 {
	mask := (uint64(1) << bits) - 1
	v := uint64(value) & mask
	if v&(uint64(1)<<(bits-1)) != 0 {
		v |= ^mask
	}
	return v
}

// rtypeToUnified maps an R-type semantic compute op to a unified Operation.
func rtypeToUnified(rop uint32) uint32 {
	switch rop {
	case rtypeOpAdd:
		return oprAdd
	case rtypeOpSub:
		return oprSub
	case rtypeOpSll:
		return oprSll
	case rtypeOpSlt:
		return oprSlt
	case rtypeOpSltu:
		return oprSltu
	case rtypeOpXor:
		return oprXor
	case rtypeOpSrl:
		return oprSrl
	case rtypeOpSra:
		return oprSra
	case rtypeOpOr:
		return oprOr
	case rtypeOpAnd:
		return oprAnd
	case rtypeOpAddw:
		return oprAddw
	case rtypeOpSubw:
		return oprSubw
	case rtypeOpSllw:
		return oprSllw
	case rtypeOpSrlw:
		return oprSrlw
	case rtypeOpSraw:
		return oprSraw
	case rtypeOpMul:
		return oprMul
	case rtypeOpMulh:
		return oprMulh
	case rtypeOpMulhsu:
		return oprMulhsu
	case rtypeOpMulhu:
		return oprMulhu
	case rtypeOpDiv:
		return oprDiv
	case rtypeOpDivu:
		return oprDivu
	case rtypeOpRem:
		return oprRem
	case rtypeOpRemu:
		return oprRemu
	case rtypeOpMulw:
		return oprMulw
	case rtypeOpDivw:
		return oprDivw
	case rtypeOpDivuw:
		return oprDivuw
	case rtypeOpRemw:
		return oprRemw
	case rtypeOpRemuw:
		return oprRemuw
	case rtypeOpKeccak:
		return oprKeccak
	default:
		return oprInvalid
	}
}

// itypeAluToUnified maps an I-type OP-IMM / OP-IMM-32 compute op to a unified
// Operation. Immediate and register ALU forms share the same op (operand b is
// the immediate, selected by b_src).
func itypeAluToUnified(iop uint32) uint32 {
	switch iop {
	case itypeOpAddi:
		return oprAdd
	case itypeOpSlti:
		return oprSlt
	case itypeOpSltiu:
		return oprSltu
	case itypeOpXori:
		return oprXor
	case itypeOpOri:
		return oprOr
	case itypeOpAndi:
		return oprAnd
	case itypeOpSlli:
		return oprSll
	case itypeOpSrli:
		return oprSrl
	case itypeOpSrai:
		return oprSra
	case itypeOpAddiw:
		return oprAddw
	case itypeOpSlliw:
		return oprSllw
	case itypeOpSrliw:
		return oprSrlw
	case itypeOpSraiw:
		return oprSraw
	default:
		return oprInvalid
	}
}

// isITypeLoad reports whether an I-type compute op is a memory load.
func isITypeLoad(iop uint32) bool {
	return iop >= itypeRead8Sgn && iop <= itypeRead32Zext
}

// loadReadKind maps an I-type load compute op to the read-phase load kind.
func loadReadKind(iop uint32) uint32 {
	switch iop {
	case itypeRead8Sgn:
		return rk8S
	case itypeRead16Sgn:
		return rk16S
	case itypeRead32Sgn:
		return rk32S
	case itypeRead64:
		return rk64
	case itypeRead8Zext:
		return rk8U
	case itypeRead16Zext:
		return rk16U
	case itypeRead32Zext:
		return rk32U
	default:
		return rkNone
	}
}

// isITypeShift reports whether an I-type compute op is a shift-immediate (its
// immediate carries a raw shift amount and is not sign-extended).
func isITypeShift(iop uint32) bool {
	switch iop {
	case itypeOpSlli, itypeOpSrli, itypeOpSrai, itypeOpSlliw, itypeOpSrliw, itypeOpSraiw:
		return true
	default:
		return false
	}
}

// decodeUnified reduces a raw 32-bit instruction (with its already-folded
// instruction type) to the single unified record. It reuses the per-type
// semantic decoders and folds operand sourcing, writeback, and pc-update into
// uniform selectors.
func decodeUnified(instr, instrType uint32) unifiedRecord {
	opcode := instr & 0x7f
	rd := (instr >> 7) & 0x1f
	funct3 := (instr >> 12) & 0x7
	rs1 := (instr >> 15) & 0x1f
	rs2 := (instr >> 20) & 0x1f
	imm12 := (instr >> 20) & 0xfff
	funct7 := (instr >> 25) & 0x7f

	rec := unifiedRecord{
		operation: oprNop,
		aSrc:      aRS1,
		bSrc:      bIMM,
		readKind:  rkNone,
		writeKind: wkNone,
		pcKind:    pkNext,
		rs1:       rs1,
		rs2:       rs2,
		rd:        rd,
	}

	switch instrType {
	case rType:
		rop, wb := decodeRTypeSemantic(opcode, funct3, funct7)
		rec.operation = rtypeToUnified(rop)
		rec.aSrc = aRS1
		rec.bSrc = bRS2
		if wb == wbStoreReg {
			rec.writeKind = wkReg
		}
		rec.pcKind = pkNext
	case iType:
		iop, _, normImm12 := decodeITypeSemantic(opcode, funct3, imm12)
		rec.aSrc = aRS1
		rec.bSrc = bIMM
		switch {
		case isITypeLoad(iop):
			rec.operation = oprMoveLoaded
			rec.readKind = loadReadKind(iop)
			rec.writeKind = wkReg
			rec.imm = sext64(imm12, 12)
		case iop == itypeJalr:
			rec.operation = oprLink
			rec.writeKind = wkReg
			rec.pcKind = pkJumpAbs
			rec.imm = sext64(imm12, 12)
		case iop == itypeEcall:
			rec.operation = oprNop
			rec.pcKind = pkSyscall
		case iop == itypeEbreak:
			rec.operation = oprNop
			rec.pcKind = pkHalt
		case iop == itypeInvalid:
			rec.operation = oprInvalid
			rec.imm = sext64(imm12, 12)
		default:
			rec.operation = itypeAluToUnified(iop)
			rec.writeKind = wkReg
			if isITypeShift(iop) {
				rec.imm = uint64(normImm12) // raw shift amount, no sign extension
			} else {
				rec.imm = sext64(imm12, 12)
			}
		}
	case sType:
		simm12 := (((instr >> 31) & 0x1) << 11) | (((instr >> 25) & 0x3f) << 5) | ((instr >> 7) & 0x1f)
		rec.aSrc = aRS1
		rec.bSrc = bIMM
		rec.imm = sext64(simm12, 12)
		switch funct3 {
		case 0b000:
			rec.writeKind = wkMem8
		case 0b001:
			rec.writeKind = wkMem16
		case 0b010:
			rec.writeKind = wkMem32
		case 0b011:
			rec.writeKind = wkMem64
		default:
			rec.operation = oprInvalid
		}
	case bType:
		bImmSign := (instr >> 31) & 0x1
		bImm10_5 := (instr >> 25) & 0x3f
		bImm4_1 := (instr >> 8) & 0xf
		bImm11 := (instr >> 7) & 0x1
		offset := (bImmSign << 12) | (bImm11 << 11) | (bImm10_5 << 5) | (bImm4_1 << 1)
		rec.aSrc = aRS1
		rec.bSrc = bRS2
		rec.imm = sext64(offset, 13)
		rec.pcKind = pkBranch
		switch funct3 {
		case 0b000:
			rec.operation = oprCmpEq
		case 0b001:
			rec.operation = oprCmpNe
		case 0b100:
			rec.operation = oprCmpLt
		case 0b101:
			rec.operation = oprCmpGe
		case 0b110:
			rec.operation = oprCmpLtu
		case 0b111:
			rec.operation = oprCmpGeu
		default:
			rec.operation = oprInvalid
			rec.pcKind = pkNext
		}
	case jType:
		jImm20 := (instr >> 31) & 0x1
		jPre := (jImm20 << 19) | (((instr >> 12) & 0xff) << 11) | (((instr >> 20) & 0x1) << 10) | ((instr >> 21) & 0x3ff)
		jImm := uint64(jPre) << 1
		if jImm20 == 1 {
			jImm |= 0xFFFFFFFFFFE00000
		}
		rec.operation = oprLink
		rec.aSrc = aPC
		rec.bSrc = bIMM
		rec.writeKind = wkReg
		rec.pcKind = pkJumpRel
		rec.imm = jImm
	case uType:
		uImm20 := (instr >> 12) & 0xfffff
		rec.bSrc = bIMM
		rec.writeKind = wkReg
		rec.imm = sext64(uImm20<<12, 32)
		if opcode == opcodeLUI {
			rec.operation = oprMoveB
			rec.aSrc = aRS1 // unused by OPR_MOVE_B
		} else { // AUIPC
			rec.operation = oprAdd
			rec.aSrc = aPC
		}
	case miscMemType:
		rec.operation = oprNop
	default: // undefinedType
		rec.operation = oprInvalid
	}

	// Fold register writes to x0 into no-ops: x0 is hard-wired to zero, so a
	// WK_REG write with rd == 0 has no effect. Folding it to WK_NONE at decode
	// time lets the interpreter's WK_REG path write registers[rd] without a
	// per-instruction rd != 0 guard. Memory stores and pc updates are untouched,
	// so loads/jumps with rd == 0 still perform their read/jump side effects.
	if rec.writeKind == wkReg && rec.rd == 0 {
		rec.writeKind = wkNone
	}

	return rec
}

type memoryBlob struct {
	offset uint64
	data   []byte
	name   string
}

// bitWriter accumulates values into a big-endian, MSB-first bit stream. This
// matches how zkc deserializes `pub input` records (see EncodeBytes /
// DecodeUnsignedInt in zkc): fields are packed tightly by their exact bit width
// (NOT rounded up to bytes), records are concatenated with no per-record
// alignment, and the final byte is zero-padded in its low bits.
type bitWriter struct {
	buf   []byte
	nbits int
}

// writeBits appends the low `width` bits of `val`, most-significant bit first.
func (w *bitWriter) writeBits(val uint64, width int) {
	for i := width - 1; i >= 0; i-- {
		if w.nbits%8 == 0 {
			w.buf = append(w.buf, 0)
		}
		if (val>>uint(i))&1 == 1 {
			w.buf[w.nbits/8] |= 1 << uint(7-(w.nbits%8))
		}
		w.nbits++
	}
}

// The purpose of this program is simply to generate a suitable ZkC json input
// file for a given RISC-V binary program.
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: go run main.go <elfFile> <inBytes|@hexFile> <inBytesOffset>")
		os.Exit(1)
	}

	elfFile, err := elf.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening ELF file: %v\n", err)
		os.Exit(1)
	}
	defer elfFile.Close()
	// Parse inBytes (supports inline 0x-hex, raw bytes, or @path-to-hex-file).
	inBytes, err := parseInBytes(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	// Parse inBytesOffset
	var inBytesOffset uint64
	inBytesOffset, err = strconv.ParseUint(os.Args[3], 0, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading input bytes offset: %v\n", err)
		os.Exit(1)
	}
	// The entry point, program blob offsets and program blob sizes are taken
	// directly from the ELF. Only the optional input bytes offset is external.
	var blobs = extractProgramBlobs(elfFile.Progs, elfFile.Sections)
	if len(inBytes) > 0 {
		blobs = append(blobs, memoryBlob{offset: inBytesOffset, data: inBytes, name: "in_bytes"})
	}
	// Optionally write a .sections file with the indexes, offsets, sizes and names of the blobs for debugging purposes.
	// This is controlled by the ELF2JSON_WRITE_SECTIONS environment variable, which must be set to "true" to enable this feature.
	switch writeSections := os.Getenv("ELF2JSON_WRITE_SECTIONS"); writeSections {
	case "", "false":
	case "true":
		sectionsFile, err := os.Create(strings.TrimSuffix(os.Args[1], ".elf") + ".sections")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error creating ELF sections file: %v\n", err)
			os.Exit(1)
		}
		writeSectionsFile(sectionsFile, blobs)
		if err := sectionsFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "error writing ELF sections file: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "ELF2JSON_WRITE_SECTIONS must be true or false, got %q\n", writeSections)
		os.Exit(1)
	}
	// Statically decode the executable region into the single pre-decoded
	// instruction input table consumed by the interpreter.
	base, decodedHex := buildDecodedProgram(elfFile.Sections)
	printJson(blobs, elfFile.Entry, base, decodedHex)
}

// parseInBytes turns an arg into raw input bytes. Four forms:
// - `*.ssz` (optional `@` prefix): returned verbatim — already a complete, length-framed input.
// - `0x...`: expects big-endian hex, byte-reversed before reaching RAM.
// - `@path`: same as `0x…`, but reads the hex from a file.
// - anything else: raw bytes, verbatim.
func parseInBytes(arg string) ([]byte, error) {
	// input ≡ ssz file
	if strings.HasSuffix(arg, ".ssz") {
		ssz, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading inBytes .ssz file: %w", err)
		}
		return ssz, nil
	}

	// input ≡ non ssz file
	if strings.HasPrefix(arg, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return nil, fmt.Errorf("reading inBytes file: %w", err)
		}
		fields := strings.Fields(string(data))
		if len(fields) != 1 {
			return nil, fmt.Errorf("expected @path to contain one 0x-prefixed input, got %d", len(fields))
		}
		return parseHexInBytes(fields[0])
	}

	// input ≡ hex string
	if strings.HasPrefix(arg, "0x") || strings.HasPrefix(arg, "0X") {
		return parseHexInBytes(arg)
	}

	// input ≡ raw bytes
	return []byte(arg), nil
}

func parseHexInBytes(arg string) ([]byte, error) {
	if !strings.HasPrefix(arg, "0x") && !strings.HasPrefix(arg, "0X") {
		return nil, fmt.Errorf("expected 0x-prefixed input bytes, got %q", arg)
	}
	inBytes, err := hex.DecodeString(arg[2:])
	if err != nil {
		return nil, fmt.Errorf("decoding hex input bytes: %w", err)
	}
	slices.Reverse(inBytes)
	return inBytes, nil
}

// Extract sparse memory blobs from allocated file-backed sections. Zero-filled
// memory such as .bss and section padding is not emitted because RAM is
// initialized to zero before the blobs are loaded.
//
// Our own tests contain .text, .rodata, .data and .bss sections.
// ACT4 tests contain .text.init, .text.rvtest, .text.rvmodel, .data,
// and .tohost sections. We do not filter by section names here.
func extractProgramBlobs(progs []*elf.Prog, sections []*elf.Section) []memoryBlob {
	var blobs []memoryBlob

	for _, p := range progs {
		if p.Type != elf.PT_LOAD || p.Memsz == 0 {
			continue
		}
		// Vaddr is where the segment is mapped in guest RAM. Memsz is the
		// number of bytes it occupies there; Filesz can be smaller when the
		// segment ends with zero-initialized memory.
		if p.Filesz > p.Memsz {
			panic(fmt.Sprintf("loadable segment at %#x has file size larger than memory size", p.Vaddr))
		}

		var sectionBlobs []memoryBlob
		progEnd := p.Vaddr + p.Memsz
		if progEnd < p.Vaddr {
			panic(fmt.Sprintf("loadable segment address overflow at %#x", p.Vaddr))
		}
		for _, s := range sections {
			if s.Size == 0 || s.Type == elf.SHT_NOBITS || s.Flags&elf.SHF_ALLOC == 0 {
				continue
			}
			sectionEnd := s.Addr + s.Size
			if sectionEnd < s.Addr {
				panic(fmt.Sprintf("section %s address overflow at %#x", s.Name, s.Addr))
			}
			if s.Addr < p.Vaddr || sectionEnd > progEnd {
				continue
			}
			sectionBlobs = append(sectionBlobs, memoryBlob{offset: s.Addr, data: readSectionBytes(s), name: s.Name})
		}
		sort.Slice(sectionBlobs, func(i, j int) bool { return sectionBlobs[i].offset < sectionBlobs[j].offset })
		blobs = append(blobs, sectionBlobs...)
	}

	if len(blobs) == 0 {
		panic("no loadable program sections found.")
	}

	return blobs
}

// readSectionBytes reads the bytes for an allocated ELF section that has file
// contents. SHT_NOBITS sections are skipped by extractProgramBlobs.
func readSectionBytes(s *elf.Section) []byte {
	data, err := s.Data()
	if err != nil {
		panic(fmt.Sprintf("error reading section %s: %v", s.Name, err))
	}
	if uint64(len(data)) != s.Size {
		panic(fmt.Sprintf("short read for section %s: got %d bytes, expected %d", s.Name, len(data), s.Size))
	}
	return data
}

// buildDecodedProgram statically decodes every 4-byte instruction word across
// the executable region of the ELF, producing the base address plus the
// hex-encoded `decoded` input array. The array is dense (one record per word in
// [base, end)), indexed at runtime by index = (pc - base) >> 2.
func buildDecodedProgram(sections []*elf.Section) (base uint64, decodedHex string) {
	var (
		execSections []*elf.Section
		minAddr      = ^uint64(0)
		maxEnd       uint64
		coveredBytes uint64
	)
	// Collect executable, file-backed sections.
	for _, s := range sections {
		if s.Size == 0 || s.Type == elf.SHT_NOBITS || s.Flags&elf.SHF_EXECINSTR == 0 {
			continue
		}
		execSections = append(execSections, s)
		coveredBytes += s.Size
		if s.Addr < minAddr {
			minAddr = s.Addr
		}
		if end := s.Addr + s.Size; end > maxEnd {
			maxEnd = end
		}
	}
	if len(execSections) == 0 {
		panic("no executable sections found for instruction decoding")
	}
	if len(execSections) > 1 {
		fmt.Fprintf(os.Stderr, "warning: %d executable sections found; the decoded tables densely cover the whole span\n",
			len(execSections))
	}
	// Align base down and end up to a 4-byte instruction boundary.
	base = minAddr &^ 0x3
	end := (maxEnd + 3) &^ uint64(0x3)
	nRecords := (end - base) / 4
	// OOM safeguard: reject an implausibly large span (e.g. far-apart
	// executable sections that would otherwise be densely filled).
	maxRecords := maxDecodedRecordsFromEnv()
	if nRecords > maxRecords {
		fmt.Fprintf(os.Stderr,
			"error: decoded program would have %d records (cap %d); executable span [%#x, %#x) is likely non-contiguous\n",
			nRecords, maxRecords, base, end)
		os.Exit(1)
	}
	// Build a flat byte image of the executable span (zero-filled gaps).
	image := make([]byte, end-base)
	for _, s := range execSections {
		data := readSectionBytes(s)
		copy(image[s.Addr-base:], data)
	}
	// Decode each instruction word into the unified record. Field bit widths
	// MUST match the `decoded` pub input declared in memory.zkc, because zkc
	// packs input records tightly by bit width:
	//   decoded: operation:Operation(u6), a_src:ASrc(u1), b_src:BSrc(u1),
	//            read_kind:ReadKind(u3), write_kind:WriteKind(u3), pc_kind:PcKind(u3),
	//            rs1:Register(u5), rs2:Register(u5), rd:Register(u5), imm:DoubleWord(u64)
	var decodedBits bitWriter
	for off := uint64(0); off+4 <= uint64(len(image)); off += 4 {
		instr := uint32(image[off]) | uint32(image[off+1])<<8 | uint32(image[off+2])<<16 | uint32(image[off+3])<<24

		opcode := instr & 0x7f
		rd := (instr >> 7) & 0x1f
		funct3 := (instr >> 12) & 0x7
		rs1 := (instr >> 15) & 0x1f
		rs2 := (instr >> 20) & 0x1f
		imm12 := (instr >> 20) & 0xfff
		funct7 := (instr >> 25) & 0x7f

		instrType := instructionTypeFromOpcode(opcode)
		if isRdZeroNoop(opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7) {
			instrType = miscMemType
		}

		rec := decodeUnified(instr, instrType)

		decodedBits.writeBits(uint64(rec.operation), 6)
		decodedBits.writeBits(uint64(rec.aSrc), 1)
		decodedBits.writeBits(uint64(rec.bSrc), 1)
		decodedBits.writeBits(uint64(rec.readKind), 3)
		decodedBits.writeBits(uint64(rec.writeKind), 3)
		decodedBits.writeBits(uint64(rec.pcKind), 3)
		decodedBits.writeBits(uint64(rec.rs1), 5)
		decodedBits.writeBits(uint64(rec.rs2), 5)
		decodedBits.writeBits(uint64(rec.rd), 5)
		decodedBits.writeBits(rec.imm, 64)
	}

	return base, hex.EncodeToString(decodedBits.buf)
}

// maxDecodedRecordsFromEnv returns the configured cap on decoded records.
func maxDecodedRecordsFromEnv() uint64 {
	if v := os.Getenv("ELF2JSON_MAX_DECODED_RECORDS"); v != "" {
		n, err := strconv.ParseUint(v, 0, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid ELF2JSON_MAX_DECODED_RECORDS %q: %v\n", v, err)
			os.Exit(1)
		}
		return n
	}
	return defaultMaxDecodedRecords
}

func writeSectionsFile(file *os.File, blobs []memoryBlob) {
	fmt.Fprintln(file, "index, offset,             size,               name")
	for i, blob := range blobs {
		fmt.Fprintf(file, "%-5d, 0x%016x, 0x%016x, %s\n", i, blob.offset, len(blob.data), blob.name)
	}
}

func printJson(blobs []memoryBlob, entryPoint, instructionBase uint64, decodedHex string) {
	var (
		entryPointString   = fmt.Sprintf("%016x", entryPoint)
		blobsCountString   = fmt.Sprintf("%016x", len(blobs))
		entryPointAndBlobs = entryPointString + "_" + blobsCountString
		blobMetadata       []string
		blobData           []string
	)

	for _, blob := range blobs {
		blobMetadata = append(blobMetadata, fmt.Sprintf("%016x_%016x", blob.offset, len(blob.data)))
		if len(blob.data) > 0 {
			blobData = append(blobData, hex.EncodeToString(blob.data))
		}
	}

	fmt.Println("{")
	fmt.Printf("\t\"%s\": \"0x%s\",\n", ENTRY_POINT_AND_BLOBS_COUNT, entryPointAndBlobs)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", BLOBS_OFFSET_AND_SIZE, strings.Join(blobMetadata, "____"))
	fmt.Printf("\t\"%s\": \"0x%s\",\n", BLOBS_DATA, strings.Join(blobData, "____"))
	fmt.Printf("\t\"%s\": \"0x%016x\",\n", INSTRUCTION_BASE, instructionBase)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", DECODED, decodedHex)
	fmt.Println("}")
}
