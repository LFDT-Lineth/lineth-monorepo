package multisethashing

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/consensys/gnark/frontend"
)

// HashGnark is the in-circuit counterpart of [Hash]: it maps a Merkle root
// into a fresh accumulator, with KoalaBear arithmetic emulated in the circuit's
// field. The accumulator starts at the empty-set digest (all zeros), so the
// result is the concatenation of the chunkSize Poseidon2 digests that
// [MSetHash.Insert] would add, with no further arithmetic.
func HashGnark(api frontend.API, root circuit.Octuplet) [MSetHashSizeNumFieldElement]circuit.Element {
	// KoalaBear arithmetic, emulated over the circuit's native field (BN254 in our circuits).
	koalaAPI := circuit.NewAPI(api)
	var zeros [blockSize]circuit.Element
	for i := range zeros {
		zeros[i] = koalaAPI.Zero()
	}

	hsh := poseidon2.NewKoalagnarkMDHasher(api)
	hsh.WriteOctuplet(root)

	var res [MSetHashSizeNumFieldElement]circuit.Element
	for i := 0; i < chunkSize; i++ {
		chunk := hsh.Sum()
		copy(res[i*blockSize:(i+1)*blockSize], chunk[:])
		if i < chunkSize-1 {
			hsh.Write(zeros[:]...)
		}
	}
	return res
}
