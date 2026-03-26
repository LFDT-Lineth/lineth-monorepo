package vortex_koalabear

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/stretchr/testify/assert"
)

type VerifierCircuit struct {
	Proof        vortex.GnarkProof
	Vi           vortex.GnarkVerifierInput
	MerkleProofs [][]smt_koalabear.GnarkProof
	Roots        []poseidon2_koalabear.GnarkOctuplet
	params       vortex.Params
}

func (c *VerifierCircuit) Define(api frontend.API) error {
	fs := fiatshamir.NewGnarkFSKoalabear(api)
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

	circuit.Roots = make([]poseidon2_koalabear.GnarkOctuplet, len(roots))
	witness.Roots = make([]poseidon2_koalabear.GnarkOctuplet, len(roots))
	for i := 0; i < len(witness.Roots); i++ {
		for j := 0; j < 8; j++ {
			witness.Roots[i][j] = roots[i][j].String()
		}
	}
	circuit.MerkleProofs = make([][]smt_koalabear.GnarkProof, len(merkleProofs))
	witness.MerkleProofs = make([][]smt_koalabear.GnarkProof, len(merkleProofs))
	for i := 0; i < len(witness.MerkleProofs); i++ {
		circuit.MerkleProofs[i] = make([]smt_koalabear.GnarkProof, len(merkleProofs[i]))
		witness.MerkleProofs[i] = make([]smt_koalabear.GnarkProof, len(merkleProofs[i]))
		for j := 0; j < len(merkleProofs[i]); j++ {
			circuit.MerkleProofs[i][j].Siblings = make([]poseidon2_koalabear.GnarkOctuplet, len(merkleProofs[i][j].Siblings))
			witness.MerkleProofs[i][j].Siblings = make([]poseidon2_koalabear.GnarkOctuplet, len(merkleProofs[i][j].Siblings))
			witness.MerkleProofs[i][j].Path = merkleProofs[i][j].Path
			for k := 0; k < len(merkleProofs[i][j].Siblings); k++ {
				for l := 0; l < 8; l++ {
					witness.MerkleProofs[i][j].Siblings[k][l] = merkleProofs[i][j].Siblings[k][l].String()
				}
			}
		}
	}

	ccs, err := frontend.CompileU32(koalabear.Modulus(), gnarkutil.NewMockBuilder(scs.NewBuilder), &circuit)
	assert.NoError(t, err)

	fullWitness, err := frontend.NewWitness(&witness, koalabear.Modulus())
	assert.NoError(t, err)
	err = ccs.IsSolved(fullWitness)
	assert.NoError(t, err)

}
