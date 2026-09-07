package predecoding

import (
	"fmt"
	"testing"
)

// TestInstructionTypeFromOpcode
// checks the opcode -> instruction-type mapping.
// Vectors are keyed by the 7-bit RISC-V opcode; the value is the expected type.
func TestInstructionTypeFromOpcode(t *testing.T) {
	vectors := map[uint32]uint32{
		// R-type: register-register ALU, 32-bit-word ALU, and Custom-1.
		opcodeOP:      rType,
		opcodeOP32:    rType,
		opcodeCUSTOM1: rType,
		// I-type: loads, immediate ALU, JALR, and SYSTEM (ecall/ebreak).
		opcodeLOAD:    iType,
		opcodeOPIMM:   iType,
		opcodeOPIMM32: iType,
		opcodeJALR:    iType,
		opcodeSYSTEM:  iType,
		// S-type: stores.
		opcodeSTORE: sType,
		// B-type: conditional branches.
		opcodeBRANCH: bType,
		// U-type: LUI / AUIPC.
		opcodeLUI:   uType,
		opcodeAUIPC: uType,
		// J-type: JAL.
		opcodeJAL: jType,
		// Misc-mem: FENCE / FENCE.I.
		opcodeMISCMEM: miscMemType,
		// Unknown opcodes fall through to the default arm.
		0b0000000: undefinedType,
		0b1111111: undefinedType,
	}

	for opcode, type_expected := range vectors {
		t.Run(fmt.Sprintf("opcode_%07b", opcode), func(t *testing.T) {
			if type_value := instructionTypeFromOpcode(opcode); type_value != type_expected {
				t.Fatalf("instructionTypeFromOpcode(%#09b) = %d, want %d", opcode, type_value, type_expected)
			}
		})
	}
}
