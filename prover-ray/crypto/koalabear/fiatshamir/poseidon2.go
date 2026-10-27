// Package fiatshamir implements the Fiat-Shamir transcript hashing to build non-interactive protocol.
// Use by instantiating a transcripting, updating it with prover messages and sampling randomness.
package fiatshamir

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// https://blog.trailofbits.com/2022/04/18/the-frozen-heart-vulnerability-in-plonk/

// FiatShamir is the transcript over koalabear, its degree 6 extension and
// Poseidon2 digests of 8 elements. See [fiatshamir.Transcript].
type FiatShamir = fiatshamir.Transcript[field.Element, field.Ext, field.Octuplet]

// NewFiatShamir creates a new FiatShamir transcript accumulator.
func NewFiatShamir() *FiatShamir {
	return fiatshamir.NewTranscript[field.Element, field.Ext](poseidon2.Compressor())
}
