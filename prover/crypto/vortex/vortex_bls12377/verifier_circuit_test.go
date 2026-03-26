package vortex_bls12377

import (
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/stretchr/testify/assert"
)

type VerifierCircuit struct {
	Proof        vortex.GnarkProof
	Vi           vortex.GnarkVerifierInput
	MerkleProofs [][]smt_bls12377.GnarkProof
	Roots        []frontend.Variable
	params       vortex.Params
}

func (c *VerifierCircuit) Define(api frontend.API) error {
	var fs fiatshamir.GnarkFS
	if api.Compiler().Field().Cmp(field.Modulus()) == 0 {
		fs = fiatshamir.NewGnarkFSKoalabear(api)
	} else {
		fs = fiatshamir.NewGnarkFSBLS12377(api)
	}
	err := vortex.GnarkVerify(api, fs, c.params, c.Proof, c.Vi)
	if err != nil {
		return err
	}
	return GnarkCheckColumnInclusionNoSis(api, c.Proof.Columns, c.MerkleProofs, c.Roots)
}

func TestGnarkVerifier(t *testing.T) {

	nCommitments := 4
	nbPolys := 15
	polySize := 1 << 10
	rate := 2
	WithSis := make([]bool, nCommitments)

	params, proof, vi, roots, merkleProofs := getProofVortexNCommitmentsWithMerkle(t, nCommitments, nbPolys, polySize, rate, WithSis)

	var circuit, witness VerifierCircuit
	circuit.params = params.Params
	circuit.Proof.Columns = make([][][]koalagnark.Element, len(proof.Columns))
	witness.Proof.Columns = make([][][]koalagnark.Element, len(proof.Columns))
	for i := 0; i < len(proof.Columns); i++ {
		circuit.Proof.Columns[i] = make([][]koalagnark.Element, len(proof.Columns[i]))
		witness.Proof.Columns[i] = make([][]koalagnark.Element, len(proof.Columns[i]))
		for j := 0; j < len(proof.Columns[i]); j++ {
			circuit.Proof.Columns[i][j] = make([]koalagnark.Element, len(proof.Columns[i][j]))
			witness.Proof.Columns[i][j] = make([]koalagnark.Element, len(proof.Columns[i][j]))
			for k := 0; k < len(proof.Columns[i][j]); k++ {
				witness.Proof.Columns[i][j][k] = koalagnark.NewElementFromKoala(proof.Columns[i][j][k])
			}
		}
	}
	circuit.Proof.LinearCombination = make([]koalagnark.Ext, proof.LinearCombination.Len())
	witness.Proof.LinearCombination = make([]koalagnark.Ext, proof.LinearCombination.Len())
	for i := 0; i < proof.LinearCombination.Len(); i++ {
		witness.Proof.LinearCombination[i] = koalagnark.NewExt(proof.LinearCombination.GetExt(i))
	}

	witness.Vi.Alpha = koalagnark.NewExt(vi.Alpha)
	witness.Vi.X = koalagnark.NewExt(vi.X)

	circuit.Vi.EntryList = make([]frontend.Variable, len(vi.EntryList))
	witness.Vi.EntryList = make([]frontend.Variable, len(vi.EntryList))
	for i := 0; i < len(vi.EntryList); i++ {
		witness.Vi.EntryList[i] = vi.EntryList[i]
	}

	circuit.Vi.Ys = make([][]koalagnark.Ext, len(vi.Ys))
	witness.Vi.Ys = make([][]koalagnark.Ext, len(vi.Ys))
	for i := 0; i < len(vi.Ys); i++ {
		circuit.Vi.Ys[i] = make([]koalagnark.Ext, len(vi.Ys[i]))
		witness.Vi.Ys[i] = make([]koalagnark.Ext, len(vi.Ys[i]))
		for j := 0; j < len(vi.Ys[i]); j++ {
			witness.Vi.Ys[i][j] = koalagnark.NewExt(vi.Ys[i][j])
		}
	}

	circuit.Roots = make([]frontend.Variable, len(roots))
	witness.Roots = make([]frontend.Variable, len(roots))
	for i := 0; i < len(witness.Roots); i++ {
		witness.Roots[i] = roots[i].String()
	}
	circuit.MerkleProofs = make([][]smt_bls12377.GnarkProof, len(merkleProofs))
	witness.MerkleProofs = make([][]smt_bls12377.GnarkProof, len(merkleProofs))
	for i := 0; i < len(witness.MerkleProofs); i++ {
		circuit.MerkleProofs[i] = make([]smt_bls12377.GnarkProof, len(merkleProofs[i]))
		witness.MerkleProofs[i] = make([]smt_bls12377.GnarkProof, len(merkleProofs[i]))
		for j := 0; j < len(merkleProofs[i]); j++ {
			circuit.MerkleProofs[i][j].Siblings = make([]frontend.Variable, len(merkleProofs[i][j].Siblings))
			witness.MerkleProofs[i][j].Siblings = make([]frontend.Variable, len(merkleProofs[i][j].Siblings))
			witness.MerkleProofs[i][j].Path = merkleProofs[i][j].Path
			for k := 0; k < len(merkleProofs[i][j].Siblings); k++ {
				witness.MerkleProofs[i][j].Siblings[k] = merkleProofs[i][j].Siblings[k].String()
			}
		}
	}

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)

}
