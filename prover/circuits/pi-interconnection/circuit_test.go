package pi_interconnection_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/consensys/gnark-crypto/ecc"
	fr377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/backend/aggregation"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
)

func TestMerkle(t *testing.T) {
	const maxNbKeccakF = 500
	for cI, c := range []struct {
		toHashLen int
		toHash    []any // toHash[:toHashLen] are the actual values used
		nbLeaves  int   // the number of leaves in the merkle trees
	}{
		{2, []any{1, 2}, 32},
	} {

		t.Run(fmt.Sprintf("%d", cI), func(t *testing.T) {

			toHashHex := make([]string, len(c.toHash))
			toHashBytes := make([][32]byte, len(c.toHash))
			toHashSnark := make([][32]frontend.Variable, (len(c.toHash)+31)/32*32)
			// construct expected merkle trees
			for i := range toHashHex {
				var x fr377.Element
				_, err := x.SetInterface(c.toHash[i])
				assert.NoError(t, err)
				toHashBytes[i] = x.Bytes()
				toHashHex[i] = utils.HexEncodeToString(toHashBytes[i][:])
				assert.Equal(t, 32, utils.Copy(toHashSnark[i][:], toHashBytes[i][:]))
			}
			for i := len(toHashHex); i < len(toHashSnark); i++ { // pad with zeros
				for j := range toHashSnark[i] {
					toHashSnark[i][j] = 0
				}
			}
			rootsHex := aggregation.PackInMiniTrees(toHashHex)

			hsh := sha3.NewLegacyKeccak256()
			for i := range rootsHex {
				root := pi_interconnection.MerkleRoot(hsh, c.nbLeaves, toHashBytes[i*c.nbLeaves:min(len(toHashBytes), (i+1)*c.nbLeaves)])
				rootHex := utils.HexEncodeToString(root[:])
				assert.Equal(t, rootsHex[i], rootHex)
			}

			roots := make([][32]frontend.Variable, len(rootsHex))
			for i := range roots {
				assert.NoError(t, internal.CopyHexEncodedBytes(roots[i][:], rootsHex[i]))
			}

			circuit := testMerkleCircuit{
				ToHash:       make([][32]frontend.Variable, len(toHashSnark)),
				Roots:        make([][32]frontend.Variable, len(roots)),
				maxNbKeccakF: maxNbKeccakF,
			}

			assignment := testMerkleCircuit{
				ToHash: toHashSnark,
				Roots:  roots,
			}

			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})
	}
}

type testMerkleCircuit struct {
	ToHash [][32]frontend.Variable
	Roots  [][32]frontend.Variable

	maxNbKeccakF int
}

func (c *testMerkleCircuit) Define(api frontend.API) error {
	hshK := keccak.NewHasher(api, c.maxNbKeccakF)
	nbLeaves := len(c.ToHash) / len(c.Roots)
	if nbLeaves*len(c.Roots) != len(c.ToHash) {
		return errors.New("partial tree; pad the toHash")
	}

	for i := range c.Roots {
		root := pi_interconnection.MerkleRootSnark(hshK, c.ToHash[i*nbLeaves:(i+1)*nbLeaves])
		internal.AssertSliceEquals(api, c.Roots[i][:], root[:])
	}

	return nil
}

func TestMaxNbCircuitsSum(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 3
	parameters.Rng.Seed(0x123456789abcdef0)

	properties := gopter.NewProperties(parameters)

	properties.Property("provides the correct number of public inputs", prop.ForAll(
		func(maxNbDecompression, maxNbExecution int) bool {
			cfg := config.PublicInput{
				MaxNbDecompression: maxNbDecompression,
				MaxNbExecution:     maxNbExecution,
				MaxNbCircuits:      20,
				ExecutionMaxNbMsg:  2,
				L2MsgMerkleDepth:   5,
				L2MsgMaxNbMerkle:   2,
				MockKeccakWizard:   true,
			}

			c, err := pi_interconnection.Compile(cfg)
			assert.NoError(t, err)
			cs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, c.Circuit)
			assert.NoError(t, err)
			return cfg.MaxNbDecompression+cfg.MaxNbExecution == pi_interconnection.GetMaxNbCircuitsSum(cs)
		}, gen.IntRange(1, 10), gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}
