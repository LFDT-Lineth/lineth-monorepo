package fri

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// MerkleHash selects the hash function of the Merkle trees: the commitment
// trees, the FRI layer trees and their openings. Digests are octuplets either
// way, so it changes no type.
//
// The Fiat-Shamir transcript is not affected: it keeps KoalaBear Poseidon2,
// and absorbs the roots as octuplets in both modes.
type MerkleHash uint8

const (
	// MerkleHashKoalaBear is KoalaBear Poseidon2 (width 16): MDHasher for the
	// leaves, Compress for the nodes. It is the default and the only mode with
	// the batched (SIMD) hashing paths.
	MerkleHashKoalaBear MerkleHash = iota
	// MerkleHashBNPacked hashes with BN254 Poseidon2 on KoalaBear elements
	// packed 8 per BN254 element: [poseidon2.BNOctupletMDHasher] for the
	// leaves and [poseidon2.BNCompressOctuplet] for the nodes, digests encoded
	// as octuplets by [poseidon2.BNDigestToOctuplet]. It is meant for the last
	// recursion layer, whose proof is verified in a BN254 gnark circuit, where
	// it is much cheaper than emulating KoalaBear Poseidon2. Natively it is
	// slower and has no batched path.
	MerkleHashBNPacked
)

// leafHasher is the leaf hashing interface shared by both modes.
type leafHasher interface {
	Reset()
	WriteElements(elmts ...field.Element)
	SumDigest() field.Octuplet
}

func (m MerkleHash) String() string {
	switch m {
	case MerkleHashKoalaBear:
		return "koalabear-poseidon2"
	case MerkleHashBNPacked:
		return "bn-packed-poseidon2"
	default:
		return fmt.Sprintf("MerkleHash(%d)", uint8(m))
	}
}

func (m MerkleHash) validate() error {
	if m > MerkleHashBNPacked {
		return fmt.Errorf("fri: unknown Merkle hash %v", m)
	}
	return nil
}

// batched reports whether the mode has the batched (SIMD) hashing paths.
func (m MerkleHash) batched() bool { return m == MerkleHashKoalaBear }

func (m MerkleHash) newLeafHasher() leafHasher {
	if m == MerkleHashBNPacked {
		return poseidon2.NewBNOctupletMDHasher()
	}
	return poseidon2.NewMDHasher()
}

// compress is the two-to-one node compression C.
func (m MerkleHash) compress(left, right field.Octuplet) field.Octuplet {
	if m == MerkleHashBNPacked {
		return poseidon2.BNCompressOctuplet(left, right)
	}
	return poseidon2.Compress(left, right)
}
