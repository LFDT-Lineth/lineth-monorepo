package types

import (
	"github.com/consensys/linea-monorepo/prover/maths/bls12377/field"
)

// KoalaFr is a wrapper for gnark's koalabear elements that is suitable
// for serialization.
type KoalaFr field.Element
