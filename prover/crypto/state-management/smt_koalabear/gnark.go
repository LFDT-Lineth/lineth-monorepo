package smt_koalabear

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// GnarkProof mirrors [Proof] in a gnark circuit.
type GnarkProof struct {
	Path     frontend.Variable
	Siblings []poseidon2_koalabear.GnarkOctuplet
}

// KoalagnarkProof mirrors [Proof] but uses koalagnark-based octuplets.
type KoalagnarkProof struct {
	Path     frontend.Variable
	Siblings []poseidon2_koalabear.KoalagnarkOctuplet
}

// intoKoalagnarkOctuplet converts a GnarkOctuplet to a KoalagnarkOctuplet.
func intoKoalagnarkOctuplet(o poseidon2_koalabear.GnarkOctuplet) poseidon2_koalabear.KoalagnarkOctuplet {
	var res poseidon2_koalabear.KoalagnarkOctuplet
	for i := 0; i < 8; i++ {
		res[i] = koalagnark.WrapFrontendVariable(o[i])
	}
	return res
}

// intoGnarkOctuplet converts a KoalagnarkOctuplet to a GnarkOctuplet.
func intoGnarkOctuplet(o poseidon2_koalabear.KoalagnarkOctuplet) poseidon2_koalabear.GnarkOctuplet {
	return o.NativeArray()
}

// intoKoalagnarkProof converts a GnarkProof to a KoalagnarkProof.
func intoKoalagnarkProof(proof GnarkProof) KoalagnarkProof {
	siblings := make([]poseidon2_koalabear.KoalagnarkOctuplet, len(proof.Siblings))
	for i, s := range proof.Siblings {
		siblings[i] = intoKoalagnarkOctuplet(s)
	}
	return KoalagnarkProof{
		Path:     proof.Path,
		Siblings: siblings,
	}
}

// GnarkRecoverRoot computes the root from the proof and the leaf.
// Delegates to [KoalagnarkRecoverRoot].
func GnarkRecoverRoot(
	api frontend.API,
	proof GnarkProof,
	leaf poseidon2_koalabear.GnarkOctuplet,
) (poseidon2_koalabear.GnarkOctuplet, error) {
	koalaProof := intoKoalagnarkProof(proof)
	koalaLeaf := intoKoalagnarkOctuplet(leaf)
	koalaRoot := KoalagnarkRecoverRoot(api, koalaProof, koalaLeaf)
	return intoGnarkOctuplet(koalaRoot), nil
}

// GnarkVerifyMerkleProof asserts the validity of a [GnarkProof] against a root.
// Delegates to [KoalagnarkVerifyMerkleProof].
func GnarkVerifyMerkleProof(
	api frontend.API,
	proof GnarkProof,
	leaf poseidon2_koalabear.GnarkOctuplet,
	root poseidon2_koalabear.GnarkOctuplet,
) error {
	koalaProof := intoKoalagnarkProof(proof)
	koalaLeaf := intoKoalagnarkOctuplet(leaf)
	koalaRoot := intoKoalagnarkOctuplet(root)
	return KoalagnarkVerifyMerkleProof(api, koalaProof, koalaLeaf, koalaRoot)
}

// selectKoalagnarkOctuplet if b=1, returns l else return r
func selectKoalagnarkOctuplet(api *koalagnark.API, b frontend.Variable, l, r poseidon2_koalabear.KoalagnarkOctuplet) poseidon2_koalabear.KoalagnarkOctuplet {
	var res poseidon2_koalabear.KoalagnarkOctuplet
	for i := 0; i < 8; i++ {
		res[i] = api.Select(b, l[i], r[i])
	}
	return res
}

// KoalagnarkRecoverRoot computes the root from the proof and the leaf using koalagnark elements.
func KoalagnarkRecoverRoot(
	api frontend.API,
	proof KoalagnarkProof,
	leaf poseidon2_koalabear.KoalagnarkOctuplet,
) poseidon2_koalabear.KoalagnarkOctuplet {
	koalaAPI := koalagnark.NewAPI(api)
	h := poseidon2_koalabear.NewKoalagnarkMDHasher(koalaAPI)

	current := leaf
	nbBits := len(proof.Siblings)
	if nbBits == 0 {
		return current
	}
	b := api.ToBinary(proof.Path, nbBits)
	for i := 0; i < len(proof.Siblings); i++ {
		h.Reset()
		left := selectKoalagnarkOctuplet(koalaAPI, b[i], proof.Siblings[i], current)
		right := selectKoalagnarkOctuplet(koalaAPI, b[i], current, proof.Siblings[i])
		h.WriteOctuplet(left, right)
		current = h.Sum()
	}

	return current
}

// KoalagnarkVerifyMerkleProof asserts the validity of a [KoalagnarkProof] against a root.
func KoalagnarkVerifyMerkleProof(
	api frontend.API,
	proof KoalagnarkProof,
	leaf poseidon2_koalabear.KoalagnarkOctuplet,
	root poseidon2_koalabear.KoalagnarkOctuplet,
) error {
	koalaAPI := koalagnark.NewAPI(api)
	r := KoalagnarkRecoverRoot(api, proof, leaf)
	for i := 0; i < 8; i++ {
		koalaAPI.AssertIsEqual(r[i], root[i])
	}
	return nil
}

// MerkleProofKoalagnark defines the circuit for validating a Merkle proof
// using Poseidon2 over KoalaBear.
type MerkleProofKoalagnark struct {
	Proofs KoalagnarkProof
	Leaf   poseidon2_koalabear.KoalagnarkOctuplet
	Root   poseidon2_koalabear.KoalagnarkOctuplet
}
