//go:build purego || !amd64

package poseidon2

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"

func compressChain16(state, matrix []field.Element, out []field.Octuplet) {
	compressChain16Generic(state, matrix, out)
}
