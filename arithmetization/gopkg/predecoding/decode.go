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
	itypeRead8SgnWB   = 0
	itypeRead16SgnWB  = 1
	itypeRead32SgnWB  = 2
	itypeRead64WB     = 3
	itypeRead8ZextWB  = 4
	itypeRead16ZextWB = 5
	itypeRead32ZextWB = 6
	itypeOpAddiWB     = 7
	itypeOpSltiWB     = 8
	itypeOpSltiuWB    = 9
	itypeOpXoriWB     = 10
	itypeOpOriWB      = 11
	itypeOpAndiWB     = 12
	itypeOpSlliWB     = 13
	itypeOpSrliWB     = 14
	itypeOpSraiWB     = 15
	itypeOpAddiwWB    = 16
	itypeOpSlliwWB    = 17
	itypeOpSrliwWB    = 18
	itypeOpSraiwWB    = 19
	itypeJalr         = 20
	itypeJalrWB       = 21
	itypeEcall        = 22
	itypeEbreak       = 23
	itypeInvalid      = 63
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
	utypeLuiWB   = 0
	utypeAuipcWB = 1
	utypeInvalid = 63
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
	computeNoOp      = 0
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
		return rtypeInvalid
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

func classifyInstruction(instruction uint32) uint32 {
	opcode := instruction & 0x7f
	rd := (instruction >> 7) & 0x1f
	funct3 := (instruction >> 12) & 0x7
	imm12 := (instruction >> 20) & 0xfff
	funct7 := (instruction >> 25) & 0x7f
	instructionType := instructionTypeFromOpcode(opcode)

	itypeOp, _ := decodeITypeSemantic(opcode, funct3, imm12)
	if instructionType != iType {
		itypeOp = itypeInvalid
	}
	itypeOp = itypeOpForRd(itypeOp, rd)
	rtypeOp := decodeRTypeSemantic(opcode, funct3, funct7)
	if instructionType != rType {
		rtypeOp = rtypeInvalid
	}
	rtypeOp = rtypeOpForRd(rtypeOp, rd)
	stypeOp := decodeSTypeSemantic(funct3)
	if instructionType != sType {
		stypeOp = stypeInvalid
	}
	btypeOp := decodeBTypeSemantic(funct3)
	if instructionType != bType {
		btypeOp = btypeInvalid
	}
	jtypeOp := decodeJTypeSemantic(opcode)
	if instructionType != jType {
		jtypeOp = jtypeInvalid
	}
	jtypeOp = jtypeOpForRd(jtypeOp, rd)
	utypeOp := decodeUTypeSemantic(opcode)
	if instructionType != uType {
		utypeOp = utypeInvalid
	}
	utypeOp = utypeOpForRd(utypeOp, rd)

	localOp := uint32(itypeInvalid)
	switch instructionType {
	case miscMemType:
		localOp = 0
	case iType:
		localOp = itypeOp
	case rType:
		localOp = rtypeOp
	case sType:
		localOp = stypeOp
	case bType:
		localOp = btypeOp
	case jType:
		localOp = jtypeOp
	case uType:
		localOp = utypeOp
	}
	return finalizeComputeOp(instructionType, localOp, rd, opcode)
}
