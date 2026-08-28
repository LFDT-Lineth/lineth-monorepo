package main

import (
	"debug/elf"
	"encoding/binary"
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
	BLOBS_EXECUTABLE            = "blobs_executable"
	BLOBS_DATA                  = "blobs_data"
	INSTRUCTION_BASE            = "instruction_base"
	DECODED                     = "decoded"
)

// Instruction type identifiers. These MUST match the Type constants in
// arithmetization/src/main/common/constants.zkc.
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
	case opcodeOP, opcodeOP32, opcodeCUSTOM1:
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

// shouldUseNoOp reports whether a valid instruction with rd=x0 should emit
// NO_OP at ELF time. Control-flow, side-effect, and syscall
// instructions keep their semantic compute_op even when rd is x0.
func shouldUseNoOp(instrType, rd, localOp, opcode uint32) bool {
	if rd != 0 {
		return false
	}
	switch instrType {
	case miscMemType:
		return true
	case iType:
		if localOp == itypeInvalid {
			return false
		}
		switch localOp {
		case itypeJalr, itypeEcall, itypeEbreak:
			return false
		default:
			return true
		}
	case rType:
		if localOp == rtypeInvalid {
			return false
		}
		switch localOp {
		case rtypeOpKeccak, rtypeOpPoseidon2, rtypeOpWriteOutput:
			return false
		default:
			return true
		}
	case uType:
		return localOp != utypeInvalid
	case jType:
		return false
	default:
		return false
	}
}

func finalizeComputeOp(instrType, localOp, rd, opcode uint32) uint32 {
	op := unifiedComputeOp(instrType, localOp)
	if op == computeInvalid {
		return computeInvalid
	}
	if shouldUseNoOp(instrType, rd, localOp, opcode) {
		return computeNoOp
	}
	return op
}

// I-type semantic micro-op local indices. Unified value = computeITypeBase + index.
const (
	itypeRead8SgnWB     = 0
	itypeRead16SgnWB    = 1
	itypeRead32SgnWB    = 2
	itypeRead64WB       = 3
	itypeRead8ZextWB     = 4
	itypeRead16ZextWB    = 5
	itypeRead32ZextWB    = 6
	itypeOpAddiWB       = 7
	itypeOpSltiWB       = 8
	itypeOpSltiuWB      = 9
	itypeOpXoriWB       = 10
	itypeOpOriWB        = 11
	itypeOpAndiWB       = 12
	itypeOpSlliWB       = 13
	itypeOpSrliWB       = 14
	itypeOpSraiWB       = 15
	itypeOpAddiwWB      = 16
	itypeOpSlliwWB      = 17
	itypeOpSrliwWB      = 18
	itypeOpSraiwWB      = 19
	itypeJalr           = 20
	itypeJalrWB         = 21
	itypeEcall          = 22
	itypeEbreak         = 23
	itypeInvalid        = 63

	wbNone     = 0
	wbStoreReg = 1
	wbMem8     = 2
	wbMem16    = 3
	wbMem32    = 4
	wbMem64    = 5
)

// itypeOpForRd selects ITYPE_JALR_WB when rd != x0; other ops already use *_WB indices.
func itypeOpForRd(localOp, rd uint32) uint32 {
	if rd == 0 || localOp == itypeEcall || localOp == itypeEbreak || localOp == itypeInvalid {
		return localOp
	}
	if localOp == itypeJalr {
		return itypeJalrWB
	}
	return localOp
}

// R-type semantic micro-op local indices. Unified value = computeRTypeBase + index.
const (
	rtypeOpAddWB       = 0
	rtypeOpSubWB       = 1
	rtypeOpSllWB       = 2
	rtypeOpSltWB       = 3
	rtypeOpSltuWB      = 4
	rtypeOpXorWB       = 5
	rtypeOpSrlWB       = 6
	rtypeOpSraWB       = 7
	rtypeOpOrWB        = 8
	rtypeOpAndWB       = 9
	rtypeOpMulWB       = 10
	rtypeOpMulhWB      = 11
	rtypeOpMulhsuWB    = 12
	rtypeOpMulhuWB     = 13
	rtypeOpDivWB       = 14
	rtypeOpDivuWB      = 15
	rtypeOpRemWB       = 16
	rtypeOpRemuWB      = 17
	rtypeOpAddwWB      = 18
	rtypeOpSubwWB      = 19
	rtypeOpSllwWB      = 20
	rtypeOpSrlwWB      = 21
	rtypeOpSrawWB      = 22
	rtypeOpMulwWB      = 23
	rtypeOpDivwWB      = 24
	rtypeOpDivuwWB     = 25
	rtypeOpRemwWB      = 26
	rtypeOpRemuwWB     = 27
	rtypeOpKeccak      = 28
	rtypeOpPoseidon2   = 29
	rtypeOpWriteOutput = 30
	rtypeInvalid       = 63
)

func rtypeOpForRd(localOp, rd uint32) uint32 {
	return localOp
}

// S-type semantic micro-op constants. These MUST match constants.zkc.
const (
	stypeStore8  = 0
	stypeStore16 = 1
	stypeStore32 = 2
	stypeStore64 = 3
	stypeInvalid = 63
)

// B-type funct3 constants. Valid branch funct3 values are stored directly in
// decoded_btype; BTYPE_INVALID (63) marks non-B slots and unrecognised funct3.
const (
	btypeInvalid = 63
)

// J-type semantic micro-op constants. These MUST match constants.zkc.
const (
	jtypeJal     = 0
	jtypeJalWB   = 1
	jtypeInvalid = 63
)

func jtypeOpForRd(baseOp, rd uint32) uint32 {
	if rd == 0 || baseOp == jtypeInvalid {
		return baseOp
	}
	if baseOp == jtypeJal {
		return jtypeJalWB
	}
	return baseOp
}

// U-type semantic micro-op local indices. Unified value = computeUTypeBase + index.
const (
	utypeLuiWB     = 0
	utypeAuipcWB   = 1
	utypeInvalid   = 63
)

func utypeOpForRd(localOp, rd uint32) uint32 {
	return localOp
}

const (
	funct12Ecall  = 0b000000000000
	funct12Ebreak = 0b000000000001
)

// Unified compute_op bases. These MUST match the ComputeOp constants in
// arithmetization/src/main/common/constants.zkc.
const (
	computeNoOp   = 0
	computeITypeBase = 1
	computeRTypeBase = 25
	computeSTypeBase = 56
	computeBTypeBase = 60
	computeJTypeBase = 66
	computeUTypeBase = 68
	computeInvalid   = 255
)

var bTypeUnifiedIndex = map[uint32]uint32{
	0b000: 0,
	0b001: 1,
	0b100: 2,
	0b101: 3,
	0b110: 4,
	0b111: 5,
}

func unifiedComputeOp(instrType, localOp uint32) uint32 {
	switch instrType {
	case miscMemType:
		return computeNoOp
	case iType:
		if localOp == itypeInvalid {
			return computeInvalid
		}
		return computeITypeBase + localOp
	case rType:
		if localOp == rtypeInvalid {
			return computeInvalid
		}
		return computeRTypeBase + localOp
	case sType:
		if localOp == stypeInvalid {
			return computeInvalid
		}
		return computeSTypeBase + localOp
	case bType:
		idx, ok := bTypeUnifiedIndex[localOp]
		if !ok {
			return computeInvalid
		}
		return computeBTypeBase + idx
	case jType:
		if localOp == jtypeInvalid {
			return computeInvalid
		}
		return computeJTypeBase + localOp
	case uType:
		if localOp == utypeInvalid {
			return computeInvalid
		}
		return computeUTypeBase + localOp
	default:
		return computeInvalid
	}
}

// decodeITypeSemantic maps a raw I-type encoding to a local op index and normalized immediate.
// Shift amounts are stripped to their low uimm6/uimm5 bits; funct6/funct7
// validation happens here.
func decodeITypeSemantic(opcode, funct3, imm12 uint32) (computeOp, normalizedImm12 uint32) {
	funct6 := (imm12 >> 6) & 0x3f
	funct7FromImm := (imm12 >> 5) & 0x7f
	uimm6 := imm12 & 0x3f
	uimm5 := imm12 & 0x1f

	switch opcode {
	case opcodeLOAD:
		switch funct3 {
		case 0b000:
			return itypeRead8SgnWB, imm12
		case 0b001:
			return itypeRead16SgnWB, imm12
		case 0b010:
			return itypeRead32SgnWB, imm12
		case 0b011:
			return itypeRead64WB, imm12
		case 0b100:
			return itypeRead8ZextWB, imm12
		case 0b101:
			return itypeRead16ZextWB, imm12
		case 0b110:
			return itypeRead32ZextWB, imm12
		default:
			return itypeInvalid, imm12
		}
	case opcodeOPIMM:
		switch funct3 {
		case 0b000:
			return itypeOpAddiWB, imm12
		case 0b010:
			return itypeOpSltiWB, imm12
		case 0b011:
			return itypeOpSltiuWB, imm12
		case 0b100:
			return itypeOpXoriWB, imm12
		case 0b110:
			return itypeOpOriWB, imm12
		case 0b111:
			return itypeOpAndiWB, imm12
		case 0b001:
			if funct6 != 0b000000 {
				return itypeInvalid, imm12
			}
			return itypeOpSlliWB, uimm6
		case 0b101:
			switch funct6 {
			case 0b000000:
				return itypeOpSrliWB, uimm6
			case 0b010000:
				return itypeOpSraiWB, uimm6
			default:
				return itypeInvalid, imm12
			}
		default:
			return itypeInvalid, imm12
		}
	case opcodeOPIMM32:
		switch funct3 {
		case 0b000:
			return itypeOpAddiwWB, imm12
		case 0b001:
			if funct7FromImm != 0b0000000 {
				return itypeInvalid, imm12
			}
			return itypeOpSlliwWB, uimm5
		case 0b101:
			switch funct7FromImm {
			case 0b0000000:
				return itypeOpSrliwWB, uimm5
			case 0b0100000:
				return itypeOpSraiwWB, uimm5
			default:
				return itypeInvalid, imm12
			}
		default:
			return itypeInvalid, imm12
		}
	case opcodeJALR:
		if funct3 != 0b000 {
			return itypeInvalid, imm12
		}
		return itypeJalr, imm12
	case opcodeSYSTEM:
		switch funct3 {
		case 0b000:
			switch imm12 {
			case funct12Ecall:
				return itypeEcall, imm12
			case funct12Ebreak:
				return itypeEbreak, imm12
			default:
				return itypeInvalid, imm12
			}
		default:
			return itypeInvalid, imm12
		}
	default:
		return itypeInvalid, imm12
	}
}

// decodeRTypeSemantic maps a raw R-type encoding to a local op index.
func decodeRTypeSemantic(opcode, funct3, funct7 uint32) (computeOp uint32) {
	switch opcode {
	case opcodeOP:
		if funct7 == 0b0000001 {
			switch funct3 {
			case 0b000:
				return rtypeOpMulWB
			case 0b001:
				return rtypeOpMulhWB
			case 0b010:
				return rtypeOpMulhsuWB
			case 0b011:
				return rtypeOpMulhuWB
			case 0b100:
				return rtypeOpDivWB
			case 0b101:
				return rtypeOpDivuWB
			case 0b110:
				return rtypeOpRemWB
			case 0b111:
				return rtypeOpRemuWB
			}
		} else if funct7 == 0b0000000 {
			switch funct3 {
			case 0b000:
				return rtypeOpAddWB
			case 0b001:
				return rtypeOpSllWB
			case 0b010:
				return rtypeOpSltWB
			case 0b011:
				return rtypeOpSltuWB
			case 0b100:
				return rtypeOpXorWB
			case 0b101:
				return rtypeOpSrlWB
			case 0b110:
				return rtypeOpOrWB
			case 0b111:
				return rtypeOpAndWB
			}
		} else if funct7 == 0b0100000 {
			switch funct3 {
			case 0b000:
				return rtypeOpSubWB
			case 0b101:
				return rtypeOpSraWB
			}
		}
		return rtypeInvalid
	case opcodeOP32:
		if funct7 == 0b0000001 {
			switch funct3 {
			case 0b000:
				return rtypeOpMulwWB
			case 0b100:
				return rtypeOpDivwWB
			case 0b101:
				return rtypeOpDivuwWB
			case 0b110:
				return rtypeOpRemwWB
			case 0b111:
				return rtypeOpRemuwWB
			}
		} else if funct7 == 0b0000000 {
			switch funct3 {
			case 0b000:
				return rtypeOpAddwWB
			case 0b001:
				return rtypeOpSllwWB
			case 0b101:
				return rtypeOpSrlwWB
			}
		} else if funct7 == 0b0100000 {
			switch funct3 {
			case 0b000:
				return rtypeOpSubwWB
			case 0b101:
				return rtypeOpSrawWB
			}
		}
		return rtypeInvalid
	case opcodeCUSTOM1:
		if funct7 != 0b0000000 {
			return rtypeInvalid
		}
		switch funct3 {
		case 0b000:
			return rtypeOpKeccak
		case 0b001:
			return rtypeOpPoseidon2
		case 0b010:
			return rtypeOpWriteOutput
		default:
			return rtypeInvalid
		}
	default:
		return rtypeInvalid
	}
}

// decodeSTypeSemantic maps a raw S-type funct3 to a semantic store compute op.
func decodeSTypeSemantic(funct3 uint32) (computeOp uint32) {
	switch funct3 {
	case 0b000:
		return stypeStore8
	case 0b001:
		return stypeStore16
	case 0b010:
		return stypeStore32
	case 0b011:
		return stypeStore64
	default:
		return stypeInvalid
	}
}

// decodeBTypeSemantic returns the branch funct3 when valid, otherwise BTYPE_INVALID.
func decodeBTypeSemantic(funct3 uint32) uint32 {
	switch funct3 {
	case 0b000, 0b001, 0b100, 0b101, 0b110, 0b111:
		return funct3
	default:
		return btypeInvalid
	}
}

// decodeJTypeSemantic maps a raw J-type encoding to a semantic base compute op.
func decodeJTypeSemantic(opcode uint32) (computeOp uint32) {
	if opcode == opcodeJAL {
		return jtypeJal
	}
	return jtypeInvalid
}

// assembleJTypeImm reassembles the split J-type immediate from a raw instruction
// word and sign-extends it to 64 bits for decoded_jtype.imm.
func assembleJTypeImm(instr uint32) uint64 {
	imm20 := (instr >> 31) & 0x1
	imm10_1 := (instr >> 21) & 0x3ff
	imm11 := (instr >> 20) & 0x1
	imm19_12 := (instr >> 12) & 0xff
	imm21 := uint32((imm20 << 20) | (imm19_12 << 12) | (imm11 << 11) | (imm10_1 << 1))
	return uint64(signExtend21(imm21))
}

func signExtend21(x uint32) int64 {
	x &= 0x1fffff
	return int64(int32(x<<11) >> 11)
}

// assembleBTypeImm reassembles the split B-type immediate from a raw instruction
// word and sign-extends it to 64 bits for decoded_btype.imm.
func assembleBTypeImm(instr uint32) uint64 {
	immSign := (instr >> 31) & 0x1
	imm10_5 := (instr >> 25) & 0x3f
	imm4_1 := (instr >> 8) & 0xf
	imm11 := (instr >> 7) & 0x1
	imm13 := uint32((immSign << 12) | (imm11 << 11) | (imm10_5 << 5) | (imm4_1 << 1))
	return uint64(signExtend13(imm13))
}

func signExtend13(x uint32) int64 {
	x &= 0x1fff
	return int64(int32(x<<19) >> 19)
}

// assembleUTypeImm sign-extends the U-type upper immediate (imm[31:12]) to 64 bits.
func assembleUTypeImm(instr uint32) uint64 {
	imm20 := (instr >> 12) & 0xfffff
	word := uint32(imm20 << 12)
	return uint64(int64(int32(word)))
}

// assembleSTypeImm sign-extends the reassembled 12-bit S-type store offset to 64 bits.
func assembleSTypeImm(simm12 uint32) uint64 {
	return uint64(signExtend12(simm12))
}

func signExtend12(x uint32) int64 {
	x &= 0xfff
	return int64(int32(x<<20) >> 20)
}

// assembleITypeImm sign-extends the normalized 12-bit I-type immediate to 64 bits.
func assembleITypeImm(normImm12 uint32) uint64 {
	return assembleSTypeImm(normImm12)
}

// decodeUTypeSemantic maps a raw U-type opcode to a local op index.
func decodeUTypeSemantic(opcode uint32) (computeOp uint32) {
	switch opcode {
	case opcodeLUI:
		return utypeLuiWB
	case opcodeAUIPC:
		return utypeAuipcWB
	default:
		return utypeInvalid
	}
}

// unifiedOperands packs pre-decoded operands into the decoded record layout:
// record layout: imm, rs1, rs2, rd.
func unifiedOperands(instrType uint32, normImm12, simm12 uint32, bImm, jImm, uImm uint64, rs1, rs2, rd uint32) (imm, opRs1, opRs2, opRd uint64) {
	switch instrType {
	case iType:
		return assembleITypeImm(normImm12), uint64(rs1), 0, uint64(rd)
	case rType:
		return 0, uint64(rs1), uint64(rs2), uint64(rd)
	case sType:
		return assembleSTypeImm(simm12), uint64(rs1), uint64(rs2), 0
	case bType:
		return bImm, uint64(rs1), uint64(rs2), 0
	case jType:
		return jImm, 0, 0, uint64(rd)
	case uType:
		return uImm, 0, 0, uint64(rd)
	default:
		return assembleITypeImm(normImm12), uint64(rs1), uint64(rs2), uint64(rd)
	}
}

type memoryBlob struct {
	offset     uint64
	data       []byte
	name       string
	executable bool
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

type inputBytes struct {
	data  []byte
	isSsz bool
}

// The purpose of this program is simply to generate a suitable ZkC json input
// file for a given RISC-V binary program.
func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: go run main.go <elfFile> <inBytes|@hexFile|@sszFile> <inBytesOffset>")
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
	if inBytes.isSsz {
		blobs = append(blobs, sszInputBlobs(inBytesOffset, inBytes.data)...)
	} else if len(inBytes.data) > 0 {
		blobs = append(blobs, memoryBlob{offset: inBytesOffset, data: inBytes.data, name: "in_bytes", executable: false})
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
	// Statically decode the executable region into the pre-decoded instruction
	// input tables consumed by the interpreter. Must use the same executable
	// blobs as blobs_data / read_instruction_from_blobs (not raw SHF_EXECINSTR sections).
	base, _, decodedHex := buildDecodedProgram(blobs)
	printJson(blobs, elfFile.Entry, base, decodedHex, predecodingProofFromEnv())
}

// predecodingProofFromEnv controls emission of blobs_executable, an input used
// only by the offline predecoding justification pass (predecoding/*.zkc). Decode
// tables (instruction_base, decoded) are always emitted regardless.
func predecodingProofFromEnv() bool {
	switch v := os.Getenv("ELF2JSON_PREDECODING_PROOF"); v {
	case "", "false":
		return false
	case "true":
		return true
	default:
		fmt.Fprintf(os.Stderr, "ELF2JSON_PREDECODING_PROOF must be true or false, got %q\n", v)
		os.Exit(1)
		return false
	}
}

// parseInBytes turns an arg into input bytes. Four forms:
// - `*.ssz` (optional `@` prefix): raw SSZ payload; framing is added by sszInputBlobs.
// - `0x...`: expects big-endian hex, byte-reversed before reaching RAM.
// - `@path`: same as `0x…`, but reads the hex from a file.
// - anything else: raw bytes, verbatim.
func parseInBytes(arg string) (inputBytes, error) {
	// input ≡ ssz file
	if strings.HasSuffix(arg, ".ssz") {
		ssz, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return inputBytes{}, fmt.Errorf("reading inBytes .ssz file: %w", err)
		}
		return inputBytes{data: ssz, isSsz: true}, nil
	}

	// input ≡ non ssz file
	if strings.HasPrefix(arg, "@") {
		data, err := os.ReadFile(strings.TrimPrefix(arg, "@"))
		if err != nil {
			return inputBytes{}, fmt.Errorf("reading inBytes file: %w", err)
		}
		fields := strings.Fields(string(data))
		if len(fields) != 1 {
			return inputBytes{}, fmt.Errorf("expected @path to contain one 0x-prefixed input, got %d", len(fields))
		}
		inBytes, err := parseHexInBytes(fields[0])
		return inputBytes{data: inBytes}, err
	}

	// input ≡ hex string
	if strings.HasPrefix(arg, "0x") || strings.HasPrefix(arg, "0X") {
		inBytes, err := parseHexInBytes(arg)
		return inputBytes{data: inBytes}, err
	}

	// input ≡ raw bytes
	return inputBytes{data: []byte(arg)}, nil
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

func sszInputBlobs(inBytesOffset uint64, ssz []byte) []memoryBlob {
	payloadOffset := inBytesOffset + 8
	if payloadOffset < inBytesOffset {
		panic("SSZ input offset overflow")
	}

	prefix := make([]byte, 8)
	binary.LittleEndian.PutUint64(prefix, uint64(len(ssz)))
	blobs := []memoryBlob{{offset: inBytesOffset, data: prefix, name: "ssz_length", executable: false}}
	if len(ssz) > 0 {
		blobs = append(blobs, memoryBlob{offset: payloadOffset, data: ssz, name: "ssz_payload", executable: false})
	}
	return blobs
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
			sectionBlobs = append(sectionBlobs, memoryBlob{
				offset:     s.Addr,
				data:       readSectionBytes(s),
				name:       s.Name,
				executable: s.Flags&elf.SHF_EXECINSTR != 0,
			})
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

// classifyInstruction mirrors buildDecodedProgram classification and
// predecoding/classify/classify.zkc.
func classifyInstruction(instr uint32) uint32 {
	opcode := instr & 0x7f
	rd := (instr >> 7) & 0x1f
	funct3 := (instr >> 12) & 0x7
	imm12 := (instr >> 20) & 0xfff
	funct7 := (instr >> 25) & 0x7f

	instrType := instructionTypeFromOpcode(opcode)

	itypeLocalOp, _ := decodeITypeSemantic(opcode, funct3, imm12)
	if instrType != iType {
		itypeLocalOp = itypeInvalid
	}
	itypeLocalOp = itypeOpForRd(itypeLocalOp, rd)

	rtypeLocalOp := decodeRTypeSemantic(opcode, funct3, funct7)
	if instrType != rType {
		rtypeLocalOp = rtypeInvalid
	}
	rtypeLocalOp = rtypeOpForRd(rtypeLocalOp, rd)

	stypeLocalOp := decodeSTypeSemantic(funct3)
	if instrType != sType {
		stypeLocalOp = stypeInvalid
	}

	btypeLocalOp := decodeBTypeSemantic(funct3)
	if instrType != bType {
		btypeLocalOp = btypeInvalid
	}

	jtypeLocalOp := decodeJTypeSemantic(opcode)
	if instrType != jType {
		jtypeLocalOp = jtypeInvalid
	}
	jtypeLocalOp = jtypeOpForRd(jtypeLocalOp, rd)

	utypeLocalOp := decodeUTypeSemantic(opcode)
	if instrType != uType {
		utypeLocalOp = utypeInvalid
	}
	utypeLocalOp = utypeOpForRd(utypeLocalOp, rd)

	var localOp uint32
	switch instrType {
	case miscMemType:
		localOp = 0
	case iType:
		localOp = itypeLocalOp
	case rType:
		localOp = rtypeLocalOp
	case sType:
		localOp = stypeLocalOp
	case bType:
		localOp = btypeLocalOp
	case jType:
		localOp = jtypeLocalOp
	case uType:
		localOp = utypeLocalOp
	default:
		localOp = itypeInvalid
	}
	return finalizeComputeOp(instrType, localOp, rd, opcode)
}

// collectExecutableImage builds the dense zero-filled executable span used by
// buildDecodedProgram from executable-flagged blobs (same bytes as blobs_data and
// read_instruction_from_blobs in ZkC). Non-allocated SHF_EXECINSTR sections that
// never appear in extractProgramBlobs are intentionally excluded.
func collectExecutableImage(blobs []memoryBlob) (base uint64, image []byte, nRecords uint64) {
	var (
		minAddr = ^uint64(0)
		maxEnd  uint64
		nExec   int
	)
	for _, blob := range blobs {
		if !blob.executable {
			continue
		}
		nExec++
		if blob.offset < minAddr {
			minAddr = blob.offset
		}
		if end := blob.offset + uint64(len(blob.data)); end > maxEnd {
			maxEnd = end
		}
	}
	if nExec == 0 {
		fmt.Fprintln(os.Stderr, "error: no executable blobs found for instruction decoding")
		os.Exit(1)
	}
	base = minAddr &^ 0x3
	end := (maxEnd + 3) &^ uint64(0x3)
	nRecords = (end - base) / 4
	image = make([]byte, end-base)
	for _, blob := range blobs {
		if !blob.executable {
			continue
		}
		copy(image[blob.offset-base:], blob.data)
	}
	return base, image, nRecords
}

// buildDecodedProgram statically decodes every 4-byte instruction word across
// the executable region, producing the base address plus the hex-encoded decoded
// input array. The array is dense (one record per word in [base, end)), indexed
// at runtime by index = (pc - base) >> 2.
func buildDecodedProgram(blobs []memoryBlob) (base uint64, nRecords uint64, decodedHex string) {
	base, image, nRecords := collectExecutableImage(blobs)
	maxRecords := maxDecodedRecordsFromEnv()
	if nRecords > maxRecords {
		fmt.Fprintf(os.Stderr,
			"error: decoded program would have %d records (cap %d); executable span [%#x, %#x) is likely non-contiguous\n",
			nRecords, maxRecords, base, base+nRecords*4)
		os.Exit(1)
	}
	// Decode each instruction word. Field bit widths MUST match the semantic
	// types declared for the inputs in memory.zkc, because zkc packs input
	// records tightly by bit width:
	//   decoded: compute_op:ComputeOp(u8), imm:DoubleWord(u64), rs1:Register(u5), rs2:Register(u5), rd:Register(u5)
	var decodedBits bitWriter
	for off := uint64(0); off+4 <= uint64(len(image)); off += 4 {
		instr := uint32(image[off]) | uint32(image[off+1])<<8 | uint32(image[off+2])<<16 | uint32(image[off+3])<<24

		opcode := instr & 0x7f
		rd := (instr >> 7) & 0x1f
		funct3 := (instr >> 12) & 0x7
		rs1 := (instr >> 15) & 0x1f
		rs2 := (instr >> 20) & 0x1f
		imm12 := (instr >> 20) & 0xfff

		instrType := instructionTypeFromOpcode(opcode)

		// S-type immediate is split in the encoding (imm[11] :: imm[10:5] :: imm[4:0]);
		// reassemble it into the 12-bit store immediate.
		simm12 := (((instr >> 31) & 0x1) << 11) | (((instr >> 25) & 0x3f) << 5) | ((instr >> 7) & 0x1f)

		// B-type immediate: reassembled and sign-extended at ELF time.
		bImm := assembleBTypeImm(instr)
		// J-type immediate: reassembled and sign-extended at ELF time.
		jImm := assembleJTypeImm(instr)

		// U-type immediate: upper 20 bits sign-extended at ELF time.
		uImm := assembleUTypeImm(instr)

		_, normImm12 := decodeITypeSemantic(opcode, funct3, imm12)
		if instrType != iType {
			normImm12 = imm12
		}

		computeOp := classifyInstruction(instr)
		decodedBits.writeBits(uint64(computeOp), 8)

		opImm, opRs1, opRs2, opRd := unifiedOperands(instrType, normImm12, simm12, bImm, jImm, uImm, rs1, rs2, rd)
		decodedBits.writeBits(opImm, 64)
		decodedBits.writeBits(opRs1, 5)
		decodedBits.writeBits(opRs2, 5)
		decodedBits.writeBits(opRd, 5)
	}

	return base, nRecords, hex.EncodeToString(decodedBits.buf)
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
	fmt.Fprintln(file, "index, offset,             size,               exec, name")
	for i, blob := range blobs {
		exec := "no"
		if blob.executable {
			exec = "yes"
		}
		fmt.Fprintf(file, "%-5d, 0x%016x, 0x%016x, %-3s, %s\n", i, blob.offset, len(blob.data), exec, blob.name)
	}
}

func printJson(blobs []memoryBlob, entryPoint, instructionBase uint64, decodedHex string, includePredecodingProof bool) {
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
	if includePredecodingProof {
		var executableBits bitWriter
		for _, blob := range blobs {
			if blob.executable {
				executableBits.writeBits(1, 1)
			} else {
				executableBits.writeBits(0, 1)
			}
		}
		fmt.Printf("\t\"%s\": \"0x%s\",\n", BLOBS_EXECUTABLE, hex.EncodeToString(executableBits.buf))
	}
	fmt.Printf("\t\"%s\": \"0x%s\",\n", BLOBS_DATA, strings.Join(blobData, "____"))
	fmt.Printf("\t\"%s\": \"0x%016x\",\n", INSTRUCTION_BASE, instructionBase)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", DECODED, decodedHex)
	fmt.Println("}")
}
