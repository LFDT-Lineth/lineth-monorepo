package predecoding

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
)

func TestPredecodeRejectsMissingExecutableBlob(t *testing.T) {
	_, err := Predecode(elfmapping.Program{Blobs: []elfmapping.Blob{{
		Address: 0x1000,
		Data:    []byte{1, 2, 3, 4},
	}}})
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("no executable blobs")) {
		t.Fatalf("Predecode() error = %v, want missing executable blob error", err)
	}
}

func TestPredecodeHonorsExplicitRecordLimit(t *testing.T) {
	program := elfmapping.Program{Blobs: []elfmapping.Blob{{
		Address:    0x1000,
		Data:       make([]byte, 8),
		Executable: true,
	}}}
	_, err := Predecode(program, WithMaxDecodedRecords(1))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("2 records (cap 1)")) {
		t.Fatalf("Predecode() error = %v, want record cap error", err)
	}
	_, err = Predecode(program, WithMaxDecodedRecords(0))
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("cap 0")) {
		t.Fatalf("Predecode() zero-cap error = %v, want record cap error", err)
	}
}

func TestPredecodeDenseExecutableSpan(t *testing.T) {
	first := make([]byte, 4)
	second := make([]byte, 4)
	binary.LittleEndian.PutUint32(first, encodeIType(opcodeOPIMM, 0, 1, 0, 1))
	binary.LittleEndian.PutUint32(second, encodeIType(opcodeOPIMM, 0, 2, 0, 2))
	decoded, err := Predecode(elfmapping.Program{Blobs: []elfmapping.Blob{
		{Address: 0x1008, Data: second, Executable: true},
		{Address: 0x1000, Data: first, Executable: true},
	}})
	if err != nil {
		t.Fatalf("Predecode() error = %v", err)
	}
	if decoded.InstructionBase != 0x1000 {
		t.Errorf("InstructionBase = %#x, want 0x1000", decoded.InstructionBase)
	}
	ops := decodeComputeOpsFromHex(hex.EncodeToString(decoded.Decoded), 3)
	want := []uint32{computeITypeBase + itypeOpAddiWB, computeInvalid, computeITypeBase + itypeOpAddiWB}
	for i := range want {
		if ops[i] != want[i] {
			t.Errorf("compute op %d = %d, want %d", i, ops[i], want[i])
		}
	}
}

func TestPredecodeRejectsInvalidExecutableRanges(t *testing.T) {
	tests := []struct {
		name    string
		blobs   []elfmapping.Blob
		wantErr string
	}{
		{
			name: "overlap",
			blobs: []elfmapping.Blob{
				{Address: 0x1000, Data: make([]byte, 4), Executable: true},
				{Address: 0x1002, Data: make([]byte, 4), Executable: true},
			},
			wantErr: "overlaps",
		},
		{
			name: "blob overflow",
			blobs: []elfmapping.Blob{{
				Address: math.MaxUint64, Data: []byte{1}, Executable: true,
			}},
			wantErr: "overflows address space",
		},
		{
			name: "alignment overflow",
			blobs: []elfmapping.Blob{{
				Address: math.MaxUint64 - 4, Data: []byte{1, 2}, Executable: true,
			}},
			wantErr: "aligning executable span",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Predecode(elfmapping.Program{Blobs: test.blobs})
			if err == nil || !bytes.Contains([]byte(err.Error()), []byte(test.wantErr)) {
				t.Fatalf("Predecode() error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestDecodedProgramEncodeInputsReturnsFreshBytes(t *testing.T) {
	program := DecodedProgram{InstructionBase: 0x11223344, Decoded: []byte{1, 2, 3}}
	first := program.EncodeInputs()
	second := program.EncodeInputs()
	if got := binary.BigEndian.Uint64(first[InstructionBaseInput]); got != program.InstructionBase {
		t.Errorf("instruction base = %#x, want %#x", got, program.InstructionBase)
	}
	first[DecodedInput][0] = 0xff
	if second[DecodedInput][0] != 1 || program.Decoded[0] != 1 {
		t.Fatal("EncodeInputs() returned bytes alias cached decoded program")
	}
}

type memoryBlob struct {
	offset     uint64
	data       []byte
	name       string
	executable bool
}

func buildDecodedProgram(blobs []memoryBlob) (uint64, uint64, string) {
	program := elfmapping.Program{Blobs: make([]elfmapping.Blob, len(blobs))}
	for i, blob := range blobs {
		program.Blobs[i] = elfmapping.Blob{
			Address: blob.offset, Data: blob.data, Name: blob.name,
			Executable: blob.executable,
		}
	}
	decoded, err := Predecode(program)
	if err != nil {
		panic(err)
	}
	_, _, records, err := executableImage(program.Blobs, DefaultMaxDecodedRecords)
	if err != nil {
		panic(err)
	}
	return decoded.InstructionBase, records, hex.EncodeToString(decoded.Decoded)
}

func collectExecutableImage(blobs []memoryBlob) (uint64, []byte, uint64) {
	program := make([]elfmapping.Blob, len(blobs))
	for i, blob := range blobs {
		program[i] = elfmapping.Blob{
			Address: blob.offset, Data: blob.data, Name: blob.name,
			Executable: blob.executable,
		}
	}
	base, image, records, err := executableImage(program, DefaultMaxDecodedRecords)
	if err != nil {
		panic(err)
	}
	return base, image, records
}

func encodeIType(opcode, funct3, rd, rs1, imm12 uint32) uint32 {
	return (imm12 << 20) | (rs1 << 15) | (funct3 << 12) | (rd << 7) | opcode
}

func encodeRType(opcode, funct7, rs2, rs1, funct3, rd uint32) uint32 {
	return (funct7 << 25) | (rs2 << 20) | (rs1 << 15) | (funct3 << 12) | (rd << 7) | opcode
}

func encodeSType(funct3, rs1, rs2, simm12 uint32) uint32 {
	imm11 := (simm12 >> 11) & 0x1
	imm10_5 := (simm12 >> 5) & 0x3f
	imm4_0 := simm12 & 0x1f
	return opcodeSTORE | (imm4_0 << 7) | (funct3 << 12) | (rs1 << 15) | (rs2 << 20) | (imm10_5 << 25) | (imm11 << 31)
}

func encodeUType(opcode, rd, imm20 uint32) uint32 {
	return (imm20 << 12) | (rd << 7) | opcode
}

func encodeJType(rd uint32, offset int32) uint32 {
	imm20 := (uint32(offset) >> 20) & 0x1
	imm10_1 := (uint32(offset) >> 1) & 0x3ff
	imm11 := (uint32(offset) >> 11) & 0x1
	imm19_12 := (uint32(offset) >> 12) & 0xff
	return opcodeJAL | (rd << 7) | (imm19_12 << 12) | (imm11 << 20) | (imm10_1 << 21) | (imm20 << 31)
}

func encodeBType(funct3, rs1, rs2 uint32, offset int32) uint32 {
	imm13 := uint32(offset) & 0x1fff
	imm12 := (imm13 >> 12) & 0x1
	imm11 := (imm13 >> 11) & 0x1
	imm10_5 := (imm13 >> 5) & 0x3f
	imm4_1 := (imm13 >> 1) & 0xf
	return opcodeBRANCH | (imm11 << 7) | (imm4_1 << 8) | (funct3 << 12) | (rs1 << 15) | (rs2 << 20) | (imm10_5 << 25) | (imm12 << 31)
}

func decodeFields(instr uint32) (opcode, instrType, rd, rs1, rs2, funct3, imm12, funct7 uint32) {
	opcode = instr & 0x7f
	instrType = instructionTypeFromOpcode(opcode)
	rd = (instr >> 7) & 0x1f
	funct3 = (instr >> 12) & 0x7
	rs1 = (instr >> 15) & 0x1f
	rs2 = (instr >> 20) & 0x1f
	imm12 = (instr >> 20) & 0xfff
	funct7 = (instr >> 25) & 0x7f
	return
}

func TestShouldUseNoOp(t *testing.T) {
	tests := []struct {
		name  string
		instr uint32
		want  bool
	}{
		{name: "addi x0 x0 0", instr: encodeIType(opcodeOPIMM, 0b000, 0, 0, 0), want: true},
		{name: "addi x0 t0 5", instr: encodeIType(opcodeOPIMM, 0b000, 0, 5, 5), want: true},
		{name: "lui x0 0", instr: encodeUType(opcodeLUI, 0, 0), want: true},
		{name: "auipc x0 0", instr: encodeUType(opcodeAUIPC, 0, 0), want: true},
		{name: "add x0 x0 x0", instr: encodeRType(opcodeOP, 0, 0, 0, 0b000, 0), want: true},
		{name: "add x0 t0 t1", instr: encodeRType(opcodeOP, 0, 6, 5, 0b000, 0), want: true},
		{name: "sub x0 x0 x0", instr: encodeRType(opcodeOP, 0b0100000, 0, 0, 0b000, 0), want: true},
		{
			name:  "invalid op x0 x0 x0",
			instr: encodeRType(opcodeOP, 0b0100000, 0, 0, 0b010, 0),
			want:  false,
		},
		{name: "ld x0 0 t0", instr: encodeIType(opcodeLOAD, 0b011, 0, 5, 0), want: true},
		{name: "jalr x0 t0 0", instr: encodeIType(opcodeJALR, 0, 0, 5, 0), want: false},
		{name: "xori x0 x0 0xff", instr: encodeIType(opcodeOPIMM, 0b100, 0, 0, 0xff), want: true},
		{name: "slli x0 x0 4", instr: encodeIType(opcodeOPIMM, 0b001, 0, 0, 4), want: true},
		{name: "keccak x0", instr: encodeRType(opcodeCUSTOM1, 0, 0, 0, 0b000, 0), want: false},
		{name: "jal x0", instr: encodeJType(0, 8), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opcode, instrType, rd, _, _, funct3, imm12, funct7 := decodeFields(tt.instr)
			var localOp uint32 = itypeInvalid
			switch instrType {
			case iType:
				localOp, _ = decodeITypeSemantic(opcode, funct3, imm12)
			case rType:
				localOp = decodeRTypeSemantic(opcode, funct3, funct7)
			case uType:
				localOp = decodeUTypeSemantic(opcode)
			}
			got := shouldUseNoOp(instrType, rd, localOp, opcode)
			if got != tt.want {
				t.Fatalf("shouldUseNoOp(%#x) = %v, want %v", tt.instr, got, tt.want)
			}
		})
	}
}

func TestBuildDecodedProgramRewritesRdZeroNoop(t *testing.T) {
	// A single `addi x0, x0, 0` (an rd-zero noop) in an executable blob at
	// base 0x1000. buildDecodedProgram must rewrite it to a NO_OP record so
	// the interpreter only advances PC by 4.
	const base uint64 = 0x1000
	instr := encodeIType(opcodeOPIMM, 0b000, 0, 0, 0)
	code := make([]byte, 4)
	binary.LittleEndian.PutUint32(code, instr)

	blobs := []memoryBlob{{offset: base, data: code, executable: true, name: ".text"}}
	gotBase, nRecords, decodedHex := buildDecodedProgram(blobs)
	if gotBase != base {
		t.Fatalf("base = %#x, want %#x", gotBase, base)
	}
	if nRecords != 1 {
		t.Fatalf("nRecords = %d, want 1", nRecords)
	}

	ops := decodeComputeOpsFromHex(decodedHex, nRecords)
	if ops[0] != computeNoOp {
		t.Fatalf("compute_op = %d, want %d (rd-zero noop rewritten to NO_OP)", ops[0], computeNoOp)
	}
}

func TestCollectExecutableImageUsesExecutableBlobsOnly(t *testing.T) {
	const base uint64 = 0x1000
	textInstr := encodeIType(opcodeOPIMM, 0b000, 5, 5, 1)
	textCode := make([]byte, 4)
	binary.LittleEndian.PutUint32(textCode, textInstr)

	// Only loadable executable blob bytes belong in the pre-decoded image; a
	// non-allocated SHF_EXECINSTR section at base+0x100 must not affect decoding.
	blobs := []memoryBlob{
		{offset: base, data: textCode, executable: true, name: ".text"},
		{offset: base + 0x100, data: []byte{0xde, 0xad, 0xbe, 0xef}, executable: false, name: ".data"},
	}

	gotBase, image, nRecords := collectExecutableImage(blobs)
	if gotBase != base {
		t.Fatalf("base = %#x, want %#x", gotBase, base)
	}
	if nRecords != 1 {
		t.Fatalf("nRecords = %d, want 1", nRecords)
	}
	if !bytes.Equal(image, textCode) {
		t.Fatalf("image = %x, want %x", image, textCode)
	}
}

func TestDecodeITypeSemantic(t *testing.T) {
	tests := []struct {
		name      string
		opcode    uint32
		funct3    uint32
		imm12     uint32
		wantOp    uint32
		wantImm12 uint32
	}{
		{name: "lb", opcode: opcodeLOAD, funct3: 0b000, imm12: 8, wantOp: itypeRead8SgnWB, wantImm12: 8},
		{name: "lh", opcode: opcodeLOAD, funct3: 0b001, imm12: 4, wantOp: itypeRead16SgnWB, wantImm12: 4},
		{name: "lw", opcode: opcodeLOAD, funct3: 0b010, imm12: 0, wantOp: itypeRead32SgnWB, wantImm12: 0},
		{name: "ld", opcode: opcodeLOAD, funct3: 0b011, imm12: 0, wantOp: itypeRead64WB, wantImm12: 0},
		{name: "lbu", opcode: opcodeLOAD, funct3: 0b100, imm12: 1, wantOp: itypeRead8ZextWB, wantImm12: 1},
		{name: "lhu", opcode: opcodeLOAD, funct3: 0b101, imm12: 2, wantOp: itypeRead16ZextWB, wantImm12: 2},
		{name: "lwu", opcode: opcodeLOAD, funct3: 0b110, imm12: 3, wantOp: itypeRead32ZextWB, wantImm12: 3},
		{name: "addi", opcode: opcodeOPIMM, funct3: 0b000, imm12: 42, wantOp: itypeOpAddiWB, wantImm12: 42},
		{name: "slli", opcode: opcodeOPIMM, funct3: 0b001, imm12: 4, wantOp: itypeOpSlliWB, wantImm12: 4},
		{name: "srli", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrliWB, wantImm12: 3},
		{name: "srai", opcode: opcodeOPIMM, funct3: 0b101, imm12: 0b010000000101, wantOp: itypeOpSraiWB, wantImm12: 5},
		{name: "xori", opcode: opcodeOPIMM, funct3: 0b100, imm12: 0xff, wantOp: itypeOpXoriWB, wantImm12: 0xff},
		{name: "addiw", opcode: opcodeOPIMM32, funct3: 0b000, imm12: 7, wantOp: itypeOpAddiwWB, wantImm12: 7},
		{name: "slliw", opcode: opcodeOPIMM32, funct3: 0b001, imm12: 2, wantOp: itypeOpSlliwWB, wantImm12: 2},
		{name: "srliw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b000000000011, wantOp: itypeOpSrliwWB, wantImm12: 3},
		{name: "sraiw", opcode: opcodeOPIMM32, funct3: 0b101, imm12: 0b010000000100, wantOp: itypeOpSraiwWB, wantImm12: 4},
		{name: "jalr", opcode: opcodeJALR, funct3: 0, imm12: 0, wantOp: itypeJalr, wantImm12: 0},
		{name: "invalid jalr funct3", opcode: opcodeJALR, funct3: 0b001, imm12: 0, wantOp: itypeInvalid, wantImm12: 0},
		{name: "ecall", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ecall, wantOp: itypeEcall, wantImm12: funct12Ecall},
		{name: "ebreak", opcode: opcodeSYSTEM, funct3: 0, imm12: funct12Ebreak, wantOp: itypeEbreak, wantImm12: funct12Ebreak},
		{name: "invalid slli funct6", opcode: opcodeOPIMM, funct3: 0b001, imm12: 0b010000000100, wantOp: itypeInvalid, wantImm12: 0b010000000100},
		{name: "invalid load funct3", opcode: opcodeLOAD, funct3: 0b111, imm12: 0, wantOp: itypeInvalid, wantImm12: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp, gotImm := decodeITypeSemantic(tt.opcode, tt.funct3, tt.imm12)
			if gotOp != tt.wantOp || gotImm != tt.wantImm12 {
				t.Fatalf("decodeITypeSemantic(op=%#x, f3=%#x, imm=%#x) = (%d, %#x), want (%d, %#x)",
					tt.opcode, tt.funct3, tt.imm12, gotOp, gotImm, tt.wantOp, tt.wantImm12)
			}
		})
	}
}

func TestItypeOpForRd(t *testing.T) {
	if got := itypeOpForRd(itypeOpAddiWB, 0); got != itypeOpAddiWB {
		t.Fatalf("itypeOpForRd(addi, x0) = %d, want %d", got, itypeOpAddiWB)
	}
	if got := itypeOpForRd(itypeOpAddiWB, 5); got != itypeOpAddiWB {
		t.Fatalf("itypeOpForRd(addi, x5) = %d, want %d", got, itypeOpAddiWB)
	}
	if got := itypeOpForRd(itypeJalr, 5); got != itypeJalrWB {
		t.Fatalf("itypeOpForRd(jalr, x5) = %d, want %d", got, itypeJalrWB)
	}
	if got := itypeOpForRd(itypeEcall, 5); got != itypeEcall {
		t.Fatalf("itypeOpForRd(ecall, x5) = %d, want %d", got, itypeEcall)
	}
	if got := itypeOpForRd(itypeInvalid, 5); got != itypeInvalid {
		t.Fatalf("itypeOpForRd(invalid, x5) = %d, want %d", got, itypeInvalid)
	}
}

func TestDecodeRTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint32
		funct3 uint32
		funct7 uint32
		wantOp uint32
	}{
		{name: "add", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAddWB},
		{name: "sub", opcode: opcodeOP, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSubWB},
		{name: "sll", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSllWB},
		{name: "slt", opcode: opcodeOP, funct3: 0b010, funct7: 0b0000000, wantOp: rtypeOpSltWB},
		{name: "sltu", opcode: opcodeOP, funct3: 0b011, funct7: 0b0000000, wantOp: rtypeOpSltuWB},
		{name: "xor", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000000, wantOp: rtypeOpXorWB},
		{name: "srl", opcode: opcodeOP, funct3: 0b101, funct7: 0b0000000, wantOp: rtypeOpSrlWB},
		{name: "sra", opcode: opcodeOP, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSraWB},
		{name: "or", opcode: opcodeOP, funct3: 0b110, funct7: 0b0000000, wantOp: rtypeOpOrWB},
		{name: "and", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000000, wantOp: rtypeOpAndWB},
		{name: "mul", opcode: opcodeOP, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMulWB},
		{name: "mulh", opcode: opcodeOP, funct3: 0b001, funct7: 0b0000001, wantOp: rtypeOpMulhWB},
		{name: "div", opcode: opcodeOP, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDivWB},
		{name: "remu", opcode: opcodeOP, funct3: 0b111, funct7: 0b0000001, wantOp: rtypeOpRemuWB},
		{name: "addw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpAddwWB},
		{name: "subw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0100000, wantOp: rtypeOpSubwWB},
		{name: "sllw", opcode: opcodeOP32, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpSllwWB},
		{name: "sraw", opcode: opcodeOP32, funct3: 0b101, funct7: 0b0100000, wantOp: rtypeOpSrawWB},
		{name: "mulw", opcode: opcodeOP32, funct3: 0b000, funct7: 0b0000001, wantOp: rtypeOpMulwWB},
		{name: "divw", opcode: opcodeOP32, funct3: 0b100, funct7: 0b0000001, wantOp: rtypeOpDivwWB},
		{name: "keccak", opcode: opcodeCUSTOM1, funct3: 0b000, funct7: 0b0000000, wantOp: rtypeOpKeccak},
		{name: "poseidon2", opcode: opcodeCUSTOM1, funct3: 0b001, funct7: 0b0000000, wantOp: rtypeOpPoseidon2},
		{name: "write_output", opcode: opcodeCUSTOM1, funct3: 0b010, funct7: 0b0000000, wantOp: rtypeOpWriteOutput},
		{name: "invalid op funct3", opcode: opcodeOP, funct3: 0b010, funct7: 0b0100000, wantOp: rtypeInvalid},
		{name: "invalid custom-1 funct3", opcode: opcodeCUSTOM1, funct3: 0b011, funct7: 0b0000000, wantOp: rtypeInvalid},
		{name: "invalid custom-1 funct7", opcode: opcodeCUSTOM1, funct3: 0b001, funct7: 0b0000001, wantOp: rtypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeRTypeSemantic(tt.opcode, tt.funct3, tt.funct7)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeRTypeSemantic(op=%#x, f3=%#x, f7=%#x) = %d, want %d",
					tt.opcode, tt.funct3, tt.funct7, gotOp, tt.wantOp)
			}
		})
	}
}

func TestRtypeOpForRd(t *testing.T) {
	if got := rtypeOpForRd(rtypeOpAddWB, 0); got != rtypeOpAddWB {
		t.Fatalf("rtypeOpForRd(add, x0) = %d, want %d", got, rtypeOpAddWB)
	}
	if got := rtypeOpForRd(rtypeOpAddWB, 5); got != rtypeOpAddWB {
		t.Fatalf("rtypeOpForRd(add, x5) = %d, want %d", got, rtypeOpAddWB)
	}
	if got := rtypeOpForRd(rtypeOpKeccak, 5); got != rtypeOpKeccak {
		t.Fatalf("rtypeOpForRd(keccak, x5) = %d, want %d", got, rtypeOpKeccak)
	}
	if got := rtypeOpForRd(rtypeOpPoseidon2, 5); got != rtypeOpPoseidon2 {
		t.Fatalf("rtypeOpForRd(poseidon2, x5) = %d, want %d", got, rtypeOpPoseidon2)
	}
	if got := rtypeOpForRd(rtypeOpWriteOutput, 5); got != rtypeOpWriteOutput {
		t.Fatalf("rtypeOpForRd(write_output, x5) = %d, want %d", got, rtypeOpWriteOutput)
	}
}

func TestDecodeSTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		funct3 uint32
		wantOp uint32
	}{
		{name: "sb", funct3: 0b000, wantOp: stypeStore8},
		{name: "sh", funct3: 0b001, wantOp: stypeStore16},
		{name: "sw", funct3: 0b010, wantOp: stypeStore32},
		{name: "sd", funct3: 0b011, wantOp: stypeStore64},
		{name: "invalid", funct3: 0b111, wantOp: stypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeSTypeSemantic(tt.funct3)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeSTypeSemantic(f3=%#x) = %d, want %d", tt.funct3, gotOp, tt.wantOp)
			}
		})
	}
}

func TestAssembleITypeImm(t *testing.T) {
	tests := []struct {
		name      string
		normImm12 uint32
		want      uint64
	}{
		{name: "zero", normImm12: 0, want: 0},
		{name: "positive_42", normImm12: 42, want: 42},
		{name: "shift_3", normImm12: 3, want: 3},
		{name: "negative_1", normImm12: 0xfff, want: 0xffffffffffffffff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assembleITypeImm(tt.normImm12)
			if got != tt.want {
				t.Fatalf("assembleITypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestAssembleSTypeImm(t *testing.T) {
	tests := []struct {
		name   string
		simm12 uint32
		want   uint64
	}{
		{name: "zero", simm12: 0, want: 0},
		{name: "positive_8", simm12: 8, want: 8},
		{name: "negative_1", simm12: 0xfff, want: 0xffffffffffffffff},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeSType(0b000, 1, 2, tt.simm12)
			simm12 := (((instr >> 31) & 0x1) << 11) | (((instr >> 25) & 0x3f) << 5) | ((instr >> 7) & 0x1f)
			got := assembleSTypeImm(simm12)
			if got != tt.want {
				t.Fatalf("assembleSTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeBTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		funct3 uint32
		wantOp uint32
	}{
		{name: "beq", funct3: 0b000, wantOp: 0b000},
		{name: "bne", funct3: 0b001, wantOp: 0b001},
		{name: "blt", funct3: 0b100, wantOp: 0b100},
		{name: "invalid", funct3: 0b010, wantOp: btypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeBTypeSemantic(tt.funct3)
			if got != tt.wantOp {
				t.Fatalf("decodeBTypeSemantic(f3=%#x) = %d, want %d", tt.funct3, got, tt.wantOp)
			}
		})
	}
}

func TestAssembleBTypeImm(t *testing.T) {
	tests := []struct {
		name   string
		offset int32
		want   uint64
	}{
		{name: "zero", offset: 0, want: 0},
		{name: "forward_8", offset: 8, want: 8},
		{name: "backward_16", offset: -16, want: 0xfffffffffffffff0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeBType(0b000, 1, 2, tt.offset)
			got := assembleBTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleBTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeJTypeSemantic(t *testing.T) {
	gotOp := decodeJTypeSemantic(opcodeJAL)
	if gotOp != jtypeJal {
		t.Fatalf("decodeJTypeSemantic(jal) = %d, want %d", gotOp, jtypeJal)
	}
}

func TestJtypeOpForRd(t *testing.T) {
	if got := jtypeOpForRd(jtypeJal, 0); got != jtypeJal {
		t.Fatalf("jtypeOpForRd(jal, x0) = %d, want %d", got, jtypeJal)
	}
	if got := jtypeOpForRd(jtypeJal, 5); got != jtypeJalWB {
		t.Fatalf("jtypeOpForRd(jal, x5) = %d, want %d", got, jtypeJalWB)
	}
}

func TestAssembleJTypeImm(t *testing.T) {
	tests := []struct {
		name   string
		offset int32
		want   uint64
	}{
		{name: "zero", offset: 0, want: 0},
		{name: "forward_4k", offset: 4096, want: 4096},
		{name: "backward_2k", offset: -2048, want: 0xfffffffffffff800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeJType(1, tt.offset)
			got := assembleJTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleJTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

func TestDecodeUTypeSemantic(t *testing.T) {
	tests := []struct {
		name   string
		opcode uint32
		wantOp uint32
	}{
		{name: "lui", opcode: opcodeLUI, wantOp: utypeLuiWB},
		{name: "auipc", opcode: opcodeAUIPC, wantOp: utypeAuipcWB},
		{name: "invalid", opcode: 0, wantOp: utypeInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp := decodeUTypeSemantic(tt.opcode)
			if gotOp != tt.wantOp {
				t.Fatalf("decodeUTypeSemantic(op=%#x) = %d, want %d", tt.opcode, gotOp, tt.wantOp)
			}
		})
	}
}

func TestUtypeOpForRd(t *testing.T) {
	if got := utypeOpForRd(utypeAuipcWB, 0); got != utypeAuipcWB {
		t.Fatalf("utypeOpForRd(auipc, x0) = %d, want %d", got, utypeAuipcWB)
	}
	if got := utypeOpForRd(utypeAuipcWB, 5); got != utypeAuipcWB {
		t.Fatalf("utypeOpForRd(auipc, x5) = %d, want %d", got, utypeAuipcWB)
	}
}

func TestAssembleUTypeImm(t *testing.T) {
	tests := []struct {
		name  string
		imm20 uint32
		want  uint64
	}{
		{name: "zero", imm20: 0, want: 0},
		{name: "positive", imm20: 0x12345, want: 0x12345000},
		{name: "negative", imm20: 0xfffff, want: 0xfffffffffffff000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instr := encodeUType(opcodeLUI, 1, tt.imm20)
			got := assembleUTypeImm(instr)
			if got != tt.want {
				t.Fatalf("assembleUTypeImm(%s) = %#x, want %#x", tt.name, got, tt.want)
			}
		})
	}
}

type bitReader struct {
	buf []byte
	pos int
}

func (r *bitReader) readBits(width int) uint64 {
	var val uint64
	for i := 0; i < width; i++ {
		bit := (r.buf[r.pos/8] >> uint(7-(r.pos%8))) & 1
		val = (val << 1) | uint64(bit)
		r.pos++
	}
	return val
}

func decodeComputeOpsFromHex(hexStr string, nRecords uint64) []uint32 {
	data, err := hex.DecodeString(hexStr)
	if err != nil {
		panic(err)
	}
	r := &bitReader{buf: data}
	ops := make([]uint32, nRecords)
	for i := uint64(0); i < nRecords; i++ {
		ops[i] = uint32(r.readBits(8))
		r.readBits(64)
		r.readBits(5)
		r.readBits(5)
		r.readBits(5)
	}
	return ops
}

func assertClassifyRoundTrip(t *testing.T, image []byte, decodedHex string) {
	t.Helper()
	nRecords := uint64(len(image) / 4)
	ops := decodeComputeOpsFromHex(decodedHex, nRecords)
	for i := uint64(0); i < nRecords; i++ {
		instr := binary.LittleEndian.Uint32(image[i*4:])
		got := classifyInstruction(instr)
		want := ops[i]
		if got != want {
			t.Fatalf("index %d instr=%#x: classify=%d decoded=%d", i, instr, got, want)
		}
	}
}

func TestClassifyInstructionExamples(t *testing.T) {
	tests := []struct {
		name  string
		instr uint32
		want  uint32
	}{
		{name: "undefined opcode", instr: 0, want: computeInvalid},
		{name: "csr csrrw", instr: encodeIType(opcodeSYSTEM, 0b001, 0, 5, 0xc02), want: computeInvalid},
		{name: "rd-zero noop", instr: encodeIType(opcodeOPIMM, 0b000, 0, 0, 0), want: computeNoOp},
		{name: "addi x0 t0 1", instr: encodeIType(opcodeOPIMM, 0b000, 0, 5, 1), want: computeNoOp},
		{name: "ld x0", instr: encodeIType(opcodeLOAD, 0b011, 0, 5, 0), want: computeNoOp},
		{name: "auipc x0", instr: encodeUType(opcodeAUIPC, 0, 0), want: computeNoOp},
		{name: "fence", instr: opcodeMISCMEM | (0b000 << 12) | (0b001 << 7), want: computeNoOp},
		{name: "addi", instr: encodeIType(opcodeOPIMM, 0b000, 5, 5, 1), want: computeITypeBase + itypeOpAddiWB},
		{name: "invalid jalr funct3", instr: encodeIType(opcodeJALR, 0b001, 5, 5, 0), want: computeInvalid},
		{name: "keccak", instr: encodeRType(opcodeCUSTOM1, 0, 0, 0, 0b000, 1), want: computeRTypeBase + rtypeOpKeccak},
		{name: "poseidon2", instr: encodeRType(opcodeCUSTOM1, 0, 0, 0, 0b001, 1), want: computeRTypeBase + rtypeOpPoseidon2},
		{name: "write_output", instr: encodeRType(opcodeCUSTOM1, 0, 0, 0, 0b010, 1), want: computeRTypeBase + rtypeOpWriteOutput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyInstruction(tt.instr); got != tt.want {
				t.Fatalf("classifyInstruction(%#x) = %d, want %d", tt.instr, got, tt.want)
			}
		})
	}
}

func TestClassifyRoundTripSyntheticImage(t *testing.T) {
	words := []uint32{
		0,
		encodeIType(opcodeOPIMM, 0b000, 0, 0, 0),
		encodeIType(opcodeSYSTEM, 0b001, 0, 5, 0xc02),
		encodeIType(opcodeOPIMM, 0b000, 5, 5, 1),
		encodeRType(opcodeCUSTOM1, 0, 0, 0, 0b001, 2),
		encodeBType(0b010, 1, 2, 8),
		opcodeMISCMEM | (0b000 << 12) | (0b001 << 7),
	}
	image := make([]byte, len(words)*4)
	var decodedBits bitWriter
	for i, instr := range words {
		binary.LittleEndian.PutUint32(image[i*4:], instr)
		opcode := instr & 0x7f
		rd := (instr >> 7) & 0x1f
		funct3 := (instr >> 12) & 0x7
		rs1 := (instr >> 15) & 0x1f
		imm12 := (instr >> 20) & 0xfff
		instrType := instructionTypeFromOpcode(opcode)
		_, normImm12 := decodeITypeSemantic(opcode, funct3, imm12)
		if instrType != iType {
			normImm12 = imm12
		}
		decodedBits.writeBits(uint64(classifyInstruction(instr)), 8)
		decodedBits.writeBits(assembleITypeImm(normImm12), 64)
		decodedBits.writeBits(uint64(rs1), 5)
		decodedBits.writeBits(0, 5)
		decodedBits.writeBits(uint64(rd), 5)
	}
	assertClassifyRoundTrip(t, image, hex.EncodeToString(decodedBits.buf))
}
