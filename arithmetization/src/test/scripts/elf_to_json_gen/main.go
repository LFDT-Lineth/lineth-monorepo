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
	DECODED_CORE                = "decoded_core"
	DECODED_ITYPE               = "decoded_itype"
	DECODED_RTYPE               = "decoded_rtype"
	DECODED_STYPE               = "decoded_stype"
	DECODED_BTYPE               = "decoded_btype"
	DECODED_JTYPE               = "decoded_jtype"
	DECODED_UTYPE               = "decoded_utype"
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
// Each writeback-capable op has a pair: BASE (even) and BASE_WB (odd).
const (
	itypeRead8Sgn   = 0
	itypeRead8SgnWB = 1
	itypeRead16Sgn  = 2
	itypeRead16SgnWB = 3
	itypeRead32Sgn  = 4
	itypeRead32SgnWB = 5
	itypeRead64     = 6
	itypeRead64WB   = 7
	itypeRead8Zext  = 8
	itypeRead8ZextWB = 9
	itypeRead16Zext = 10
	itypeRead16ZextWB = 11
	itypeRead32Zext = 12
	itypeRead32ZextWB = 13

	itypeOpAddi  = 14
	itypeOpAddiWB = 15
	itypeOpSlti  = 16
	itypeOpSltiWB = 17
	itypeOpSltiu = 18
	itypeOpSltiuWB = 19
	itypeOpXori  = 20
	itypeOpXoriWB = 21
	itypeOpOri   = 22
	itypeOpOriWB = 23
	itypeOpAndi  = 24
	itypeOpAndiWB = 25
	itypeOpSlli  = 26
	itypeOpSlliWB = 27
	itypeOpSrli  = 28
	itypeOpSrliWB = 29
	itypeOpSrai  = 30
	itypeOpSraiWB = 31

	itypeOpAddiw = 32
	itypeOpAddiwWB = 33
	itypeOpSlliw = 34
	itypeOpSlliwWB = 35
	itypeOpSrliw = 36
	itypeOpSrliwWB = 37
	itypeOpSraiw = 38
	itypeOpSraiwWB = 39

	itypeJalr    = 40
	itypeJalrWB  = 41
	itypeEcall   = 42
	itypeEbreak  = 43
	itypeInvalid = 63

	wbNone      = 0
	wbStoreReg  = 1
	wbMem8      = 2
	wbMem16     = 3
	wbMem32     = 4
	wbMem64     = 5
)

// itypeOpForRd selects the *_WB variant when rd != x0 and the base op supports
// register writeback. Base ops are always even; *_WB variants are odd.
func itypeOpForRd(baseOp, rd uint32) uint32 {
	if rd == 0 || baseOp == itypeEcall || baseOp == itypeEbreak || baseOp == itypeInvalid {
		return baseOp
	}
	if baseOp&1 == 0 && baseOp < itypeEcall {
		return baseOp + 1
	}
	return baseOp
}

// R-type semantic micro-op constants. These MUST match constants.zkc.
const (
	rtypeOpAdd    = 0
	rtypeOpAddWB  = 1
	rtypeOpSub    = 2
	rtypeOpSubWB  = 3
	rtypeOpSll    = 4
	rtypeOpSllWB  = 5
	rtypeOpSlt    = 6
	rtypeOpSltWB  = 7
	rtypeOpSltu   = 8
	rtypeOpSltuWB = 9
	rtypeOpXor    = 10
	rtypeOpXorWB  = 11
	rtypeOpSrl    = 12
	rtypeOpSrlWB  = 13
	rtypeOpSra    = 14
	rtypeOpSraWB  = 15
	rtypeOpOr     = 16
	rtypeOpOrWB   = 17
	rtypeOpAnd    = 18
	rtypeOpAndWB  = 19

	rtypeOpMul    = 20
	rtypeOpMulWB  = 21
	rtypeOpMulh   = 22
	rtypeOpMulhWB = 23
	rtypeOpMulhsu = 24
	rtypeOpMulhsuWB = 25
	rtypeOpMulhu  = 26
	rtypeOpMulhuWB = 27
	rtypeOpDiv    = 28
	rtypeOpDivWB  = 29
	rtypeOpDivu   = 30
	rtypeOpDivuWB = 31
	rtypeOpRem    = 32
	rtypeOpRemWB  = 33
	rtypeOpRemu   = 34
	rtypeOpRemuWB = 35

	rtypeOpAddw  = 36
	rtypeOpAddwWB = 37
	rtypeOpSubw  = 38
	rtypeOpSubwWB = 39
	rtypeOpSllw  = 40
	rtypeOpSllwWB = 41
	rtypeOpSrlw  = 42
	rtypeOpSrlwWB = 43
	rtypeOpSraw  = 44
	rtypeOpSrawWB = 45

	rtypeOpMulw  = 46
	rtypeOpMulwWB = 47
	rtypeOpDivw  = 48
	rtypeOpDivwWB = 49
	rtypeOpDivuw = 50
	rtypeOpDivuwWB = 51
	rtypeOpRemw  = 52
	rtypeOpRemwWB = 53
	rtypeOpRemuw = 54
	rtypeOpRemuwWB = 55

	rtypeOpKeccak  = 56
	rtypeInvalid   = 63
)

func rtypeOpForRd(baseOp, rd uint32) uint32 {
	if rd == 0 || baseOp == rtypeOpKeccak || baseOp == rtypeInvalid {
		return baseOp
	}
	if baseOp&1 == 0 && baseOp < rtypeOpKeccak {
		return baseOp + 1
	}
	return baseOp
}

// S-type semantic micro-op constants. These MUST match constants.zkc.
const (
	stypeStore8   = 0
	stypeStore16  = 1
	stypeStore32  = 2
	stypeStore64  = 3
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

// U-type semantic micro-op constants. These MUST match constants.zkc.
const (
	utypeLui     = 0
	utypeLuiWB   = 1
	utypeAuipc   = 2
	utypeAuipcWB = 3
	utypeInvalid = 63
)

func utypeOpForRd(baseOp, rd uint32) uint32 {
	if rd == 0 || baseOp == utypeInvalid {
		return baseOp
	}
	if baseOp&1 == 0 && baseOp < utypeInvalid {
		return baseOp + 1
	}
	return baseOp
}

const (
	funct12Ecall  = 0b000000000000
	funct12Ebreak = 0b000000000001
)

// decodeITypeSemantic maps a raw I-type encoding to a semantic base compute op
// (even; *_WB is selected later by itypeOpForRd) and normalized immediate.
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
			return itypeRead8Sgn, imm12
		case 0b001:
			return itypeRead16Sgn, imm12
		case 0b010:
			return itypeRead32Sgn, imm12
		case 0b011:
			return itypeRead64, imm12
		case 0b100:
			return itypeRead8Zext, imm12
		case 0b101:
			return itypeRead16Zext, imm12
		case 0b110:
			return itypeRead32Zext, imm12
		default:
			return itypeInvalid, imm12
		}
	case opcodeOPIMM:
		switch funct3 {
		case 0b000:
			return itypeOpAddi, imm12
		case 0b010:
			return itypeOpSlti, imm12
		case 0b011:
			return itypeOpSltiu, imm12
		case 0b100:
			return itypeOpXori, imm12
		case 0b110:
			return itypeOpOri, imm12
		case 0b111:
			return itypeOpAndi, imm12
		case 0b001:
			if funct6 != 0b000000 {
				return itypeInvalid, imm12
			}
			return itypeOpSlli, uimm6
		case 0b101:
			switch funct6 {
			case 0b000000:
				return itypeOpSrli, uimm6
			case 0b010000:
				return itypeOpSrai, uimm6
			default:
				return itypeInvalid, imm12
			}
		default:
			return itypeInvalid, imm12
		}
	case opcodeOPIMM32:
		switch funct3 {
		case 0b000:
			return itypeOpAddiw, imm12
		case 0b001:
			if funct7FromImm != 0b0000000 {
				return itypeInvalid, imm12
			}
			return itypeOpSlliw, uimm5
		case 0b101:
			switch funct7FromImm {
			case 0b0000000:
				return itypeOpSrliw, uimm5
			case 0b0100000:
				return itypeOpSraiw, uimm5
			default:
				return itypeInvalid, imm12
			}
		default:
			return itypeInvalid, imm12
		}
	case opcodeJALR:
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

// decodeRTypeSemantic maps a raw R-type encoding to a semantic base compute op
// (even; *_WB is selected later by rtypeOpForRd). funct3/funct7 validation happens here.
func decodeRTypeSemantic(opcode, funct3, funct7 uint32) (computeOp uint32) {
	switch opcode {
	case opcodeOP:
		if funct7 == 0b0000001 {
			switch funct3 {
			case 0b000:
				return rtypeOpMul
			case 0b001:
				return rtypeOpMulh
			case 0b010:
				return rtypeOpMulhsu
			case 0b011:
				return rtypeOpMulhu
			case 0b100:
				return rtypeOpDiv
			case 0b101:
				return rtypeOpDivu
			case 0b110:
				return rtypeOpRem
			case 0b111:
				return rtypeOpRemu
			}
		} else if funct7 == 0b0000000 {
			switch funct3 {
			case 0b000:
				return rtypeOpAdd
			case 0b001:
				return rtypeOpSll
			case 0b010:
				return rtypeOpSlt
			case 0b011:
				return rtypeOpSltu
			case 0b100:
				return rtypeOpXor
			case 0b101:
				return rtypeOpSrl
			case 0b110:
				return rtypeOpOr
			case 0b111:
				return rtypeOpAnd
			}
		} else if funct7 == 0b0100000 {
			switch funct3 {
			case 0b000:
				return rtypeOpSub
			case 0b101:
				return rtypeOpSra
			}
		}
		return rtypeInvalid
	case opcodeOP32:
		if funct7 == 0b0000001 {
			switch funct3 {
			case 0b000:
				return rtypeOpMulw
			case 0b100:
				return rtypeOpDivw
			case 0b101:
				return rtypeOpDivuw
			case 0b110:
				return rtypeOpRemw
			case 0b111:
				return rtypeOpRemuw
			}
		} else if funct7 == 0b0000000 {
			switch funct3 {
			case 0b000:
				return rtypeOpAddw
			case 0b001:
				return rtypeOpSllw
			case 0b101:
				return rtypeOpSrlw
			}
		} else if funct7 == 0b0100000 {
			switch funct3 {
			case 0b000:
				return rtypeOpSubw
			case 0b101:
				return rtypeOpSraw
			}
		}
		return rtypeInvalid
	case opcodeCUSTOM1:
		if funct3 == 0b000 && funct7 == 0b0000000 {
			return rtypeOpKeccak
		}
		return rtypeInvalid
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

// decodeUTypeSemantic maps a raw U-type opcode to a semantic base compute op.
func decodeUTypeSemantic(opcode uint32) (computeOp uint32) {
	switch opcode {
	case opcodeLUI:
		return utypeLui
	case opcodeAUIPC:
		return utypeAuipc
	default:
		return utypeInvalid
	}
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
	// Statically decode the executable region into the pre-decoded instruction
	// input tables consumed by the interpreter.
	base, coreHex, itypeHex, rtypeHex, stypeHex, btypeHex, jtypeHex, utypeHex := buildDecodedProgram(elfFile.Sections)
	printJson(blobs, elfFile.Entry, base, coreHex, itypeHex, rtypeHex, stypeHex, btypeHex, jtypeHex, utypeHex)
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
// hex-encoded decoded_core / decoded_itype / decoded_rtype / decoded_stype /
// decoded_btype / decoded_jtype / decoded_utype input arrays. The arrays are
// dense (one record per word in [base, end)), indexed at runtime by
// index = (pc - base) >> 2.
func buildDecodedProgram(sections []*elf.Section) (base uint64, coreHex, itypeHex, rtypeHex, stypeHex, btypeHex, jtypeHex, utypeHex string) {
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
	// Decode each instruction word. Field bit widths MUST match the semantic
	// types declared for the inputs in memory.zkc, because zkc packs input
	// records tightly by bit width:
	//   decoded_core : opcode:Opcode(u7), instruction_type:Type(u3), instruction_parameters:u25
	//   decoded_itype: compute_op:ITypeComputeOp(u6), imm12:Imm12(u12), rs1:Register(u5), rd:Register(u5)
	//   decoded_rtype: compute_op:RTypeComputeOp(u6), rs1:Register(u5), rs2:Register(u5), rd:Register(u5)
	//   decoded_stype: compute_op:STypeComputeOp(u6), imm12:Imm12(u12), rs2:Register(u5), rs1:Register(u5)
	//   decoded_btype: compute_op:BTypeComputeOp(u6), imm_sign:u1, imm_10_5:u6, rs2:Register(u5), rs1:Register(u5), imm_4_1:u4, imm_11:u1
	//   decoded_jtype: compute_op:JTypeComputeOp(u6), imm20:SignBit(u1), imm10_1:u10, imm11:u1, imm19_12:u8, rd:Register(u5)
	//   decoded_utype: compute_op:UTypeComputeOp(u6), imm20:Imm20(u20), rd:Register(u5)
	var (
		coreBits  bitWriter
		itypeBits bitWriter
		rtypeBits bitWriter
		stypeBits bitWriter
		btypeBits bitWriter
		jtypeBits bitWriter
		utypeBits bitWriter
	)
	for off := uint64(0); off+4 <= uint64(len(image)); off += 4 {
		instr := uint32(image[off]) | uint32(image[off+1])<<8 | uint32(image[off+2])<<16 | uint32(image[off+3])<<24

		opcode := instr & 0x7f
		params := (instr >> 7) & 0x1ffffff
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

		// S-type immediate is split in the encoding (imm[11] :: imm[10:5] :: imm[4:0]);
		// reassemble it into the 12-bit store immediate.
		simm12 := (((instr >> 31) & 0x1) << 11) | (((instr >> 25) & 0x3f) << 5) | ((instr >> 7) & 0x1f)

		// B-type immediate sub-fields (kept split; reassembled at runtime in b_type.zkc).
		bImmSign := (instr >> 31) & 0x1  // imm[12]
		bImm10_5 := (instr >> 25) & 0x3f // imm[10:5]
		bImm4_1 := (instr >> 8) & 0xf    // imm[4:1]
		bImm11 := (instr >> 7) & 0x1     // imm[11]
		// J-type immediate sub-fields (kept split; reassembled at runtime in j_type.zkc).
		jImm20 := (instr >> 31) & 0x1    // imm[20]
		jImm10_1 := (instr >> 21) & 0x3ff // imm[10:1]
		jImm11 := (instr >> 20) & 0x1    // imm[11]
		jImm19_12 := (instr >> 12) & 0xff // imm[19:12]

		// U-type immediate: imm[31:12] (20 bits).
		uImm20 := (instr >> 12) & 0xfffff

		coreBits.writeBits(uint64(opcode), 7)
		coreBits.writeBits(uint64(instrType), 3)
		coreBits.writeBits(uint64(params), 25)

		computeOp, normImm12 := decodeITypeSemantic(opcode, funct3, imm12)
		if instrType != iType {
			computeOp, normImm12 = itypeInvalid, imm12
		}
		computeOp = itypeOpForRd(computeOp, rd)

		itypeBits.writeBits(uint64(computeOp), 6)
		itypeBits.writeBits(uint64(normImm12), 12)
		itypeBits.writeBits(uint64(rs1), 5)
		itypeBits.writeBits(uint64(rd), 5)

		rtypeComputeOp := decodeRTypeSemantic(opcode, funct3, funct7)
		if instrType != rType {
			rtypeComputeOp = rtypeInvalid
		}
		rtypeComputeOp = rtypeOpForRd(rtypeComputeOp, rd)

		rtypeBits.writeBits(uint64(rtypeComputeOp), 6)
		rtypeBits.writeBits(uint64(rs1), 5)
		rtypeBits.writeBits(uint64(rs2), 5)
		rtypeBits.writeBits(uint64(rd), 5)

		stypeComputeOp := decodeSTypeSemantic(funct3)
		if instrType != sType {
			stypeComputeOp = stypeInvalid
		}
		stypeBits.writeBits(uint64(stypeComputeOp), 6)
		stypeBits.writeBits(uint64(simm12), 12)
		stypeBits.writeBits(uint64(rs2), 5)
		stypeBits.writeBits(uint64(rs1), 5)

		btypeComputeOp := decodeBTypeSemantic(funct3)
		if instrType != bType {
			btypeComputeOp = btypeInvalid
		}
		btypeBits.writeBits(uint64(btypeComputeOp), 6)
		btypeBits.writeBits(uint64(bImmSign), 1)
		btypeBits.writeBits(uint64(bImm10_5), 6)
		btypeBits.writeBits(uint64(rs2), 5)
		btypeBits.writeBits(uint64(rs1), 5)
		btypeBits.writeBits(uint64(bImm4_1), 4)
		btypeBits.writeBits(uint64(bImm11), 1)

		jtypeComputeOp := decodeJTypeSemantic(opcode)
		if instrType != jType {
			jtypeComputeOp = jtypeInvalid
		}
		jtypeComputeOp = jtypeOpForRd(jtypeComputeOp, rd)
		jtypeBits.writeBits(uint64(jtypeComputeOp), 6)
		jtypeBits.writeBits(uint64(jImm20), 1)
		jtypeBits.writeBits(uint64(jImm10_1), 10)
		jtypeBits.writeBits(uint64(jImm11), 1)
		jtypeBits.writeBits(uint64(jImm19_12), 8)
		jtypeBits.writeBits(uint64(rd), 5)

		utypeComputeOp := decodeUTypeSemantic(opcode)
		if instrType != uType {
			utypeComputeOp = utypeInvalid
		}
		utypeComputeOp = utypeOpForRd(utypeComputeOp, rd)
		utypeBits.writeBits(uint64(utypeComputeOp), 6)
		utypeBits.writeBits(uint64(uImm20), 20)
		utypeBits.writeBits(uint64(rd), 5)
	}

	return base,
		hex.EncodeToString(coreBits.buf),
		hex.EncodeToString(itypeBits.buf),
		hex.EncodeToString(rtypeBits.buf),
		hex.EncodeToString(stypeBits.buf),
		hex.EncodeToString(btypeBits.buf),
		hex.EncodeToString(jtypeBits.buf),
		hex.EncodeToString(utypeBits.buf)
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

func printJson(blobs []memoryBlob, entryPoint, instructionBase uint64, coreHex, itypeHex, rtypeHex, stypeHex, btypeHex, jtypeHex, utypeHex string) {
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
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_CORE, coreHex)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_ITYPE, itypeHex)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_RTYPE, rtypeHex)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_STYPE, stypeHex)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_BTYPE, btypeHex)
	fmt.Printf("\t\"%s\": \"0x%s\",\n", DECODED_JTYPE, jtypeHex)
	fmt.Printf("\t\"%s\": \"0x%s\"\n", DECODED_UTYPE, utypeHex)
	fmt.Println("}")
}
