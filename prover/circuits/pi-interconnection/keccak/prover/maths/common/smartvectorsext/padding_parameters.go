package smartvectorsext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
)

func fieldPadding() field.Element {
	return field.Zero()
}

func fieldPaddingInt() uint64 {
	return 0
}
