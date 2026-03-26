package p256verify

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
)

const (
	input_filler_key = "p256-verify-input-filler"
)

func init() {
	plonk.RegisterInputFiller(input_filler_key, inputFiller)
}

func inputFiller(circuitInstance, inputIndex int) field.Element {
	datas := []string{
		// h
		"0", "0", "0", "0", "0", "0", "0", "0",
		"0", "0", "0", "0", "0", "0", "0", "0",
		// r
		"0", "0", "0", "0", "0", "0", "0", "0",
		"1", "0", "0", "0", "0", "0", "0", "0",
		// s
		"0", "0", "0", "0", "0", "0", "0", "0",
		"1", "0", "0", "0", "0", "0", "0", "0",
		// qx
		"0x40f2", "0x63a4", "0xe6e5", "0xf8bc", "0x4247", "0xe12c", "0xd1f2", "0x6b17",
		"0xc296", "0xd898", "0x3945", "0xf4a1", "0x33a0", "0x2deb", "0x7d81", "0x7703",
		// qy
		"0x9e16", "0x7c0f", "0xeb4a", "0x8ee7", "0x7f9b", "0xfe1a", "0x42e2", "0x4fe3",
		"0x51f5", "0x37bf", "0x4068", "0xcbb6", "0x5ece", "0x6b31", "0x3357", "0x2bce",
		// expected result
		"0", "0", "0", "0", "0", "0", "0", "0",
		"0", "0", "0", "0", "0", "0", "0", "0",
	}
	return field.NewFromString(datas[inputIndex%nbRows])
}
