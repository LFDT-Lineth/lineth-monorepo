package predecoding

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"math"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
)

// TODO: do we want a max ?
const DefaultMaxDecodedRecords uint64 = 2_000_000

const (
	// InstructionBaseInput contains the lowest aligned executable address.
	InstructionBaseInput = "instruction_base"
	// DecodedInput contains the densely packed predecoded instruction rows.
	DecodedInput = "decoded"
)

const (
	undefinedType = 0
	rType         = 1
	iType         = 2
	sType         = 3
	bType         = 4
	uType         = 5
	jType         = 6
	// TODO : should be renamed
	miscMemType = 7
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

// checkNoOp folds the destination register into the already-unified compute op.
// rd is the only field that changes the op after format decoding:
//
//   - rd != x0: the two link ops gain their writeback variant
//     (JALR -> JALR_WB, JAL -> JAL_WB). Every other writeback op is already
//     stored as its *_WB value, so it is returned unchanged.
//   - rd == x0: architecturally inert writeback ops (loads, ALU imm/reg,
//     LUI/AUIPC) collapse to NO_OP, since their only effect is writing
//     registers[rd] and x0 is hardwired to zero. Control-flow, memory, syscall,
//     and precompile ops keep their semantic op, and COMPUTE_INVALID stays
//     invalid.
func checkNoOp(instrType, op, rd uint32) uint32 {
	if rd != 0 {
		switch op {
		case itypeJalr:
			return itypeJalrWB
		case jtypeJal:
			return jtypeJalWB
		}
		return op
	}

	// rd == x0: collapse inert writeback ops to NO_OP.
	switch instrType {
	case miscMemType:
		return computeNoOp
	case iType:
		switch op {
		case computeInvalid, itypeJalr, itypeEcall, itypeEbreak:
			return op
		default:
			return computeNoOp
		}
	case rType:
		switch op {
		case computeInvalid, rtypeOpKeccak, rtypeOpPoseidon2, rtypeOpWriteOutput:
			return op
		default:
			return computeNoOp
		}
	case uType:
		if op == computeInvalid {
			return op
		}
		return computeNoOp
	default: // sType, bType, jType, undefined
		return op
	}
}

// I-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc. WB means Write Back, when the result is written back
// to the register file.
const (
	itypeRead8SgnWB   = 1  // LB, read signed 8 bits, sign extend to 64 bits
	itypeRead16SgnWB  = 2  // LH
	itypeRead32SgnWB  = 3  // LW
	itypeRead64WB     = 4  // LD
	itypeRead8ZextWB  = 5  // LBU, read unsigned 8 bits, zero extend to 64 bits
	itypeRead16ZextWB = 6  // LHU
	itypeRead32ZextWB = 7  // LWU
	itypeOpAddiWB     = 8  // ADDI
	itypeOpSltiWB     = 9  // SLTI
	itypeOpSltiuWB    = 10 // SLTIU
	itypeOpXoriWB     = 11 // XORI
	itypeOpOriWB      = 12 // ORI
	itypeOpAndiWB     = 13 // ANDI
	itypeOpSlliWB     = 14 // SLLI
	itypeOpSrliWB     = 15 // SRLI
	itypeOpSraiWB     = 16 // SRAI
	itypeOpAddiwWB    = 17 // ADDIW
	itypeOpSlliwWB    = 18 // SLLIW
	itypeOpSrliwWB    = 19 // SRLIW
	itypeOpSraiwWB    = 20 // SRAIW
	itypeJalr         = 21 // JALR
	itypeJalrWB       = 22 // JALR_WB
	itypeEcall        = 23 // ECALL
	itypeEbreak       = 24 // EBREAK
)

// R-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc.
const (
	rtypeOpAddWB       = 25
	rtypeOpSubWB       = 26
	rtypeOpSllWB       = 27
	rtypeOpSltWB       = 28
	rtypeOpSltuWB      = 29
	rtypeOpXorWB       = 30
	rtypeOpSrlWB       = 31
	rtypeOpSraWB       = 32
	rtypeOpOrWB        = 33
	rtypeOpAndWB       = 34
	rtypeOpMulWB       = 35
	rtypeOpMulhWB      = 36
	rtypeOpMulhsuWB    = 37
	rtypeOpMulhuWB     = 38
	rtypeOpDivWB       = 39
	rtypeOpDivuWB      = 40
	rtypeOpRemWB       = 41
	rtypeOpRemuWB      = 42
	rtypeOpAddwWB      = 43
	rtypeOpSubwWB      = 44
	rtypeOpSllwWB      = 45
	rtypeOpSrlwWB      = 46
	rtypeOpSrawWB      = 47
	rtypeOpMulwWB      = 48
	rtypeOpDivwWB      = 49
	rtypeOpDivuwWB     = 50
	rtypeOpRemwWB      = 51
	rtypeOpRemuwWB     = 52
	rtypeOpKeccak      = 53
	rtypeOpPoseidon2   = 54
	rtypeOpWriteOutput = 55
)

// S-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc.
const (
	stypeStore8  = 56
	stypeStore16 = 57
	stypeStore32 = 58
	stypeStore64 = 59
)

// B-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc. COMPUTE_INVALID marks non-B slots and unrecognised
// funct3 values.
const (
	btypeBeq  = 60 // BEQ  (funct3 0b000)
	btypeBne  = 61 // BNE  (funct3 0b001)
	btypeBlt  = 62 // BLT  (funct3 0b100)
	btypeBge  = 63 // BGE  (funct3 0b101)
	btypeBltu = 64 // BLTU (funct3 0b110)
	btypeBgeu = 65 // BGEU (funct3 0b111)
)

// J-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc.
const (
	jtypeJal   = 66
	jtypeJalWB = 67
)

// U-type semantic compute_op values. These are the unified ComputeOp codes and
// MUST match constants.zkc.
const (
	utypeLuiWB   = 68
	utypeAuipcWB = 69
)

const (
	funct12Ecall  = 0b000000000000
	funct12Ebreak = 0b000000000001
)

// Unified compute_op sentinels. These MUST match the ComputeOp constants in
// arithmetization/src/main/common/constants.zkc. All semantic ops are assigned
// their unified ComputeOp value directly in the per-format const blocks above,
// so the decoders already return unified codes.
const (
	computeNoOp    = 0
	computeInvalid = 255
)

// imm12Funct6 extracts the funct6 field (imm12[11:6]) that validates RV64
// immediate shifts (slli/srli/srai).
func imm12Funct6(imm12 uint32) uint32 { return (imm12 >> 6) & 0x3f }

// imm12Funct7 extracts the funct7 field (imm12[11:5]) that validates RV64 word
// immediate shifts (slliw/srliw/sraiw).
func imm12Funct7(imm12 uint32) uint32 { return (imm12 >> 5) & 0x7f }

// imm12Uimm6 extracts the 6-bit shift amount (imm12[5:0]) for RV64 shifts.
func imm12Uimm6(imm12 uint32) uint32 { return imm12 & 0x3f }

// imm12Uimm5 extracts the 5-bit shift amount (imm12[4:0]) for RV64 word shifts.
func imm12Uimm5(imm12 uint32) uint32 { return imm12 & 0x1f }

// decodeITypeSemantic maps a raw I-type encoding to a unified compute op and normalized immediate.
// Shift amounts are stripped to their low uimm6/uimm5 bits; funct6/funct7
// validation happens here.
func decodeITypeSemantic(opcode, funct3, imm12 uint32) (computeOp, normalizedImm12 uint32) {
	funct6 := imm12Funct6(imm12)
	funct7FromImm := imm12Funct7(imm12)
	uimm6 := imm12Uimm6(imm12)
	uimm5 := imm12Uimm5(imm12)

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
			return computeInvalid, imm12
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
				return computeInvalid, imm12
			}
			return itypeOpSlliWB, uimm6
		case 0b101:
			switch funct6 {
			case 0b000000:
				return itypeOpSrliWB, uimm6
			case 0b010000:
				return itypeOpSraiWB, uimm6
			default:
				return computeInvalid, imm12
			}
		default:
			return computeInvalid, imm12
		}
	case opcodeOPIMM32:
		switch funct3 {
		case 0b000:
			return itypeOpAddiwWB, imm12
		case 0b001:
			if funct7FromImm != 0b0000000 {
				return computeInvalid, imm12
			}
			return itypeOpSlliwWB, uimm5
		case 0b101:
			switch funct7FromImm {
			case 0b0000000:
				return itypeOpSrliwWB, uimm5
			case 0b0100000:
				return itypeOpSraiwWB, uimm5
			default:
				return computeInvalid, imm12
			}
		default:
			return computeInvalid, imm12
		}
	case opcodeJALR:
		if funct3 != 0b000 {
			return computeInvalid, imm12
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
				return computeInvalid, imm12
			}
		default:
			return computeInvalid, imm12
		}
	default:
		return computeInvalid, imm12
	}
}

// decodeRTypeSemantic maps a raw R-type encoding to a unified compute op.
func decodeRTypeSemantic(opcode, funct3, funct7 uint32) (computeOp uint32) {
	switch opcode {
	case opcodeOP:
		switch funct7 {
		case 0b0000001:
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
		case 0b0000000:
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
		case 0b0100000:
			switch funct3 {
			case 0b000:
				return rtypeOpSubWB
			case 0b101:
				return rtypeOpSraWB
			}
		}
		return computeInvalid
	case opcodeOP32:
		switch funct7 {
		case 0b0000001:
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
		case 0b0000000:
			switch funct3 {
			case 0b000:
				return rtypeOpAddwWB
			case 0b001:
				return rtypeOpSllwWB
			case 0b101:
				return rtypeOpSrlwWB
			}
		case 0b0100000:
			switch funct3 {
			case 0b000:
				return rtypeOpSubwWB
			case 0b101:
				return rtypeOpSrawWB
			}
		}
		return computeInvalid
	case opcodeCUSTOM1:
		if funct7 != 0b0000000 {
			return computeInvalid
		}
		switch funct3 {
		case 0b000:
			return rtypeOpKeccak
		case 0b001:
			return rtypeOpPoseidon2
		case 0b010:
			return rtypeOpWriteOutput
		default:
			return computeInvalid
		}
	default:
		return computeInvalid
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
		return computeInvalid
	}
}

// decodeBTypeSemantic returns the branch funct3 when valid, otherwise BTYPE_INVALID.
func decodeBTypeSemantic(funct3 uint32) uint32 {
	switch funct3 {
	case 0b000:
		return btypeBeq
	case 0b001:
		return btypeBne
	case 0b100:
		return btypeBlt
	case 0b101:
		return btypeBge
	case 0b110:
		return btypeBltu
	case 0b111:
		return btypeBgeu
	default:
		return computeInvalid
	}
}

// decodeJTypeSemantic maps a raw J-type encoding to a semantic base compute op.
func decodeJTypeSemantic(opcode uint32) (computeOp uint32) {
	if opcode == opcodeJAL {
		return jtypeJal
	}
	return computeInvalid
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

// decodeUTypeSemantic maps a raw U-type opcode to a unified compute op.
func decodeUTypeSemantic(opcode uint32) (computeOp uint32) {
	switch opcode {
	case opcodeLUI:
		return utypeLuiWB
	case opcodeAUIPC:
		return utypeAuipcWB
	default:
		return computeInvalid
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

type config struct {
	maxDecodedRecords uint64
	mappingOptions    []elfmapping.Option
}

// Option configures predecoding.
type Option func(*config) error

// WithMaxDecodedRecords limits the aligned executable span decoded by
// [Predecode]. A zero limit rejects every non-empty executable program.
func WithMaxDecodedRecords(maximum uint64) Option {
	return func(cfg *config) error {
		cfg.maxDecodedRecords = maximum
		return nil
	}
}

// WithIncludeExecutable includes the executable-blob bitmap used by the
// standalone predecoding proof in the map returned by [PrepareInputs].
func WithIncludeExecutable() Option {
	return func(cfg *config) error {
		cfg.mappingOptions = append(
			cfg.mappingOptions,
			elfmapping.WithIncludeExecutable(),
		)
		return nil
	}
}

// WithSectionsWriter writes the legacy diagnostic blob table while
// [PrepareInputs] encodes the ELF mapping.
func WithSectionsWriter(writer io.Writer) Option {
	return func(cfg *config) error {
		if writer == nil {
			return fmt.Errorf("sections writer is nil")
		}
		cfg.mappingOptions = append(
			cfg.mappingOptions,
			elfmapping.WithSectionsWriter(writer),
		)
		return nil
	}
}

// PrepareInputs maps and predecodes a guest ELF and adds input data using the
// length-prefixed guest convention. Call [Predecode] and the elfmapping APIs
// separately for guests expecting raw input or when reusing the same ELF.
func PrepareInputs(
	elfBytes []byte,
	inputData []byte,
	options ...Option,
) (map[string][]byte, error) {
	cfg, err := applyOptions(options)
	if err != nil {
		return nil, err
	}
	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, err
	}
	decoded, err := predecode(program, cfg.maxDecodedRecords)
	if err != nil {
		return nil, err
	}
	inputBlobs, err := elfmapping.NewData(
		elfmapping.DefaultInputOrigin,
		inputData,
		elfmapping.WithLengthPrefix(),
	)
	if err != nil {
		return nil, err
	}
	inputs, err := elfmapping.EncodeInputs(
		program,
		inputBlobs,
		cfg.mappingOptions...,
	)
	if err != nil {
		return nil, err
	}
	maps.Copy(inputs, decoded.EncodeInputs())
	return inputs, nil
}

// DecodedProgram is the cached predecoded contribution to the R5 inputs.
type DecodedProgram struct {
	InstructionBase uint64
	Decoded         []byte
}

// EncodeInputs returns fresh raw input bytes suitable for ZkC input loading.
func (program DecodedProgram) EncodeInputs() map[string][]byte {
	base := binary.BigEndian.AppendUint64(nil, program.InstructionBase)
	decoded := append([]byte(nil), program.Decoded...)
	return map[string][]byte{
		InstructionBaseInput: base,
		DecodedInput:         decoded,
	}
}

// Predecode creates one packed instruction row for every four-byte word in the
// aligned span containing the program's executable blobs.
func Predecode(program elfmapping.Program, options ...Option) (DecodedProgram, error) {
	cfg, err := applyOptions(options)
	if err != nil {
		return DecodedProgram{}, err
	}
	return predecode(program, cfg.maxDecodedRecords)
}

func applyOptions(options []Option) (config, error) {
	cfg := config{maxDecodedRecords: DefaultMaxDecodedRecords}
	for _, option := range options {
		if option == nil {
			return config{}, fmt.Errorf("applying predecoding option: nil option")
		}
		if err := option(&cfg); err != nil {
			return config{}, fmt.Errorf("applying predecoding option: %w", err)
		}
	}
	return cfg, nil
}

func predecode(program elfmapping.Program, maxDecodedRecords uint64) (DecodedProgram, error) {
	base, image, records, err := executableImage(program.Blobs, maxDecodedRecords)
	if err != nil {
		return DecodedProgram{}, err
	}
	decoded := decodeImage(image, records)
	return DecodedProgram{InstructionBase: base, Decoded: decoded}, nil
}

func executableImage(
	blobs []elfmapping.Blob,
	maxRecords uint64,
) (uint64, []byte, uint64, error) {
	executable := make([]elfmapping.Blob, 0, len(blobs))
	for _, blob := range blobs {
		if blob.Executable {
			executable = append(executable, blob)
		}
	}
	if len(executable) == 0 {
		return 0, nil, 0, fmt.Errorf("no executable blobs found for instruction decoding")
	}
	sort.SliceStable(executable, func(i, j int) bool {
		return executable[i].Address < executable[j].Address
	})

	base := executable[0].Address &^ uint64(3)
	var previousEnd, maxEnd uint64
	for i, blob := range executable {
		end := blob.Address + uint64(len(blob.Data))
		if end < blob.Address {
			return 0, nil, 0, fmt.Errorf(
				"executable blob at %#x overflows address space",
				blob.Address,
			)
		}
		if i != 0 && len(blob.Data) != 0 && blob.Address < previousEnd {
			return 0, nil, 0, fmt.Errorf(
				"executable blob at %#x overlaps preceding blob ending at %#x",
				blob.Address,
				previousEnd,
			)
		}
		if len(blob.Data) != 0 {
			previousEnd = end
		}
		if end > maxEnd {
			maxEnd = end
		}
	}
	if maxEnd > math.MaxUint64-3 {
		return 0, nil, 0, fmt.Errorf("aligning executable span end %#x overflows address space", maxEnd)
	}
	alignedEnd := (maxEnd + 3) &^ uint64(3)
	records := (alignedEnd - base) / 4
	if records > maxRecords {
		return 0, nil, 0, fmt.Errorf(
			"decoded program would have %d records (cap %d); executable span [%#x, %#x) is likely non-contiguous",
			records,
			maxRecords,
			base,
			alignedEnd,
		)
	}
	span := alignedEnd - base
	if span > uint64(math.MaxInt) {
		return 0, nil, 0, fmt.Errorf("executable span has unsupported size %d", span)
	}
	image := make([]byte, int(span))
	for _, blob := range executable {
		copy(image[blob.Address-base:], blob.Data)
	}
	return base, image, records, nil
}

func decodeImage(image []byte, records uint64) []byte {
	var decoded bitWriter
	for record := range records {
		offset := record * 4
		instruction := binary.LittleEndian.Uint32(image[offset : offset+4])
		decodeInstruction(&decoded, instruction)
	}
	return decoded.buf
}

func decodeInstruction(decoded *bitWriter, instruction uint32) {
	opcode := instruction & 0x7f
	rd := (instruction >> 7) & 0x1f
	funct3 := (instruction >> 12) & 0x7
	rs1 := (instruction >> 15) & 0x1f
	rs2 := (instruction >> 20) & 0x1f
	imm12 := (instruction >> 20) & 0xfff
	instructionType := instructionTypeFromOpcode(opcode)
	simm12 := (((instruction >> 31) & 1) << 11) |
		(((instruction >> 25) & 0x3f) << 5) |
		((instruction >> 7) & 0x1f)
	_, normalizedImm12 := decodeITypeSemantic(opcode, funct3, imm12)
	if instructionType != iType {
		normalizedImm12 = imm12
	}

	decoded.writeBits(uint64(classifyInstruction(instruction)), 8)
	imm, operandRS1, operandRS2, operandRD := unifiedOperands(
		instructionType,
		normalizedImm12,
		simm12,
		assembleBTypeImm(instruction),
		assembleJTypeImm(instruction),
		assembleUTypeImm(instruction),
		rs1,
		rs2,
		rd,
	)
	decoded.writeBits(imm, 64)
	decoded.writeBits(operandRS1, 5)
	decoded.writeBits(operandRS2, 5)
	decoded.writeBits(operandRD, 5)
}

// instructionFields holds the raw RISC-V fields sliced out of a 32-bit
// instruction word.
type instructionFields struct {
	opcode uint32
	rd     uint32
	funct3 uint32
	imm12  uint32
	funct7 uint32
}

// extractFields slices the standard RISC-V fields out of a raw instruction word.
// Each field is shifted down to bit 0 and masked to its width;
//
//	bit: 31       25 24    20 19    15 14   12 11    7 6         0
//	    [  funct7  ][  rs2  ][  rs1  ][funct3][  rd  ][  opcode  ]  R-type view
//	    [         imm12     ][  rs1  ][funct3][  rd  ][ opcode ]  I-type view
func extractFields(instruction uint32) instructionFields {
	return instructionFields{
		opcode: instruction & 0x7f,
		rd:     (instruction >> 7) & 0x1f,
		funct3: (instruction >> 12) & 0x7,
		imm12:  (instruction >> 20) & 0xfff,
		funct7: (instruction >> 25) & 0x7f,
	}
}

func classifyInstruction(instruction uint32) uint32 {
	fields := extractFields(instruction)
	opcode := fields.opcode
	rd := fields.rd
	funct3 := fields.funct3
	imm12 := fields.imm12
	funct7 := fields.funct7
	instructionType := instructionTypeFromOpcode(opcode)

	var op uint32
	switch instructionType {
	case miscMemType:
		op = computeNoOp
	case iType:
		op, _ = decodeITypeSemantic(opcode, funct3, imm12)
	case rType:
		op = decodeRTypeSemantic(opcode, funct3, funct7)
	case sType:
		op = decodeSTypeSemantic(funct3)
	case bType:
		op = decodeBTypeSemantic(funct3)
	case jType:
		op = decodeJTypeSemantic(opcode)
	case uType:
		op = decodeUTypeSemantic(opcode)
	}
	return checkNoOp(instructionType, op, rd)
}
