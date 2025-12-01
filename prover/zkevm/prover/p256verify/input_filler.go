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
		"0", "0",
		// r
		"0", "1",
		// s
		"0", "1",
		// qx
		"0x6b17d1f2e12c4247f8bce6e563a440f2",
		"0x77037d812deb33a0f4a13945d898c296",
		// qy
		"0x4fe342e2fe1a7f9b8ee7eb4a7c0f9e16",
		"0x2bce33576b315ececbb6406837bf51f5",
		// expected result
		"0", "0",
	}
	return field.NewFromString(datas[inputIndex%nbRows])
}
