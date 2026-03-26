package keccak

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/circuits/internal/test_utils"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateCols(t *testing.T) {
	for i, c := range getTestCases(t) {

		in := make([][][32]frontend.Variable, len(c.in))
		hash := make([][2]frontend.Variable, len(c.hash))
		for j := range c.in {
			in[j] = make([][32]frontend.Variable, len(c.in[j])/32)
			for k := range in[j] {
				for l := 0; l < 32; l++ {
					in[j][k][l] = c.in[j][32*k+l]
				}
			}
			hash[j][0] = c.hash[j][:16]
			hash[j][1] = c.hash[j][16:]
		}

		circuit := testCreateColsCircuit{
			StaticInLength:    true,
			In:                make([][][32]frontend.Variable, len(c.in)),
			InLength:          make([]frontend.Variable, len(c.in)),
			Lanes:             make([]frontend.Variable, len(c.lanes)),
			IsFirstLaneOfHash: make([]frontend.Variable, len(c.isFirstLaneOfHash)),
			IsLaneActive:      make([]frontend.Variable, len(c.isLaneActive)),
			Hash:              make([][2]frontend.Variable, len(c.hash)),
		}

		for j := range circuit.In {
			circuit.In[j] = make([][32]frontend.Variable, len(in[j]))
		}

		assignment := testCreateColsCircuit{
			In:                in,
			InLength:          make([]frontend.Variable, len(c.in)),
			Lanes:             make([]frontend.Variable, len(c.lanes)),
			IsFirstLaneOfHash: utils.ToVariableSlice(c.isFirstLaneOfHash),
			IsLaneActive:      utils.ToVariableSlice(c.isLaneActive),
			Hash:              hash,
		}

		for j := range c.lanes {
			assignment.Lanes[j] = c.lanes[j][:]
		}
		for j := range assignment.InLength {
			assignment.InLength[j] = len(c.in[j]) / 32
		}

		// static inLen
		t.Run(fmt.Sprintf("%d/static-padding", i), func(t *testing.T) {
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})

		t.Run(fmt.Sprintf("%d/tight-padding", i), func(t *testing.T) {
			// dynamic, but "tight" inLen
			circuit.StaticInLength = false
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})

		var zeroLane [32]frontend.Variable
		for i := range zeroLane {
			zeroLane[i] = 0
		}

		loosen := func(slack int) (_circuit, _assignment frontend.Circuit) {
			c, a := circuit, assignment
			a.In = make([][][32]frontend.Variable, len(c.In))
			c.In = make([][][32]frontend.Variable, len(c.In))

			for i := range c.In {
				c.In[i] = make([][32]frontend.Variable, len(circuit.In[i])+slack)
				a.In[i] = make([][32]frontend.Variable, len(circuit.In[i])+slack)

				for j := range c.In[i] {
					a.In[i][j] = zeroLane
					if j < len(circuit.In[i]) {
						a.In[i][j] = assignment.In[i][j]
					}
				}
			}

			return &c, &a
		}

		t.Run(fmt.Sprintf("%d/loose-padding", i), func(t *testing.T) {
			// dynamic, "loose" inLen
			circuit, assignment := loosen(1)
			assert.NoError(t, test.IsSolved(circuit, assignment, ecc.BLS12_377.ScalarField()))
		})

		t.Run(fmt.Sprintf("%d/veryloose-padding", i), func(t *testing.T) {
			// dynamic, very loose inLen
			circuit, assignment := loosen(2)
			assert.NoError(t, test.IsSolved(circuit, assignment, ecc.BLS12_377.ScalarField()))
		})
	}
}

type testCreateColsCircuit struct {
	In       [][][32]frontend.Variable
	InLength []frontend.Variable

	Lanes                           []frontend.Variable
	IsFirstLaneOfHash, IsLaneActive []frontend.Variable
	Hash                            [][2]frontend.Variable
	StaticInLength                  bool
}

func (c *testCreateColsCircuit) Define(api frontend.API) error {
	hsh := Hasher{
		api:     api,
		nbLanes: len(c.Lanes),
	}

	radix := big.NewInt(256)
	for i := range c.In {
		var computedHash [32]frontend.Variable
		if c.StaticInLength {
			computedHash = hsh.Sum(nil, c.In[i]...)
		} else {
			computedHash = hsh.Sum(c.InLength[i], c.In[i]...)
		}
		hi := compress.ReadNum(api, computedHash[:16], radix)
		lo := compress.ReadNum(api, computedHash[16:], radix)

		api.AssertIsEqual(c.Hash[i][0], hi)
		api.AssertIsEqual(c.Hash[i][1], lo)
	}

	lanes, isLaneActive, isFirstLaneOfHash := hsh.createColumns()
	for i := range c.Lanes {
		api.AssertIsEqual(c.Lanes[i], lanes[i])
		api.AssertIsEqual(c.IsLaneActive[i], isLaneActive[i])
		api.AssertIsEqual(c.IsFirstLaneOfHash[i], isFirstLaneOfHash[i])
	}

	return nil
}

func TestE2E(t *testing.T) {

	wizardComponent := NewWizardVerifierSubCircuit(3, dummy.Compile) // increase maxNbKeccakF as needed when introducing longer test vectors

	for i, c := range getTestCases(t) {

		in := make([][][32]frontend.Variable, len(c.in))
		hash := make([][32]frontend.Variable, utils.NextPowerOfTwo(len(c.hash)))
		for j := range c.in {
			in[j] = make([][32]frontend.Variable, len(c.in[j])/32)
			for k := range in[j] {
				for l := 0; l < 32; l++ {
					in[j][k][l] = c.in[j][32*k+l]
				}
			}
			for k := range hash[j] {
				hash[j][k] = c.hash[j][k]
			}
		}

		wizardSubCircuit, err := wizardComponent.Compile()
		require.NoError(t, err)

		circuit := testE2ECircuit{
			In:             make([][][32]frontend.Variable, len(c.in)),
			InLength:       make([]frontend.Variable, len(c.in)),
			Hash:           make([][32]frontend.Variable, len(c.hash)),
			WizardVerifier: wizardSubCircuit,
		}

		for j := range circuit.In {
			circuit.In[j] = make([][32]frontend.Variable, len(in[j]))
		}

		assignment := testE2ECircuit{
			In:             in,
			InLength:       make([]frontend.Variable, len(c.in)),
			Hash:           hash,
			WizardVerifier: wizardComponent.Assign(c.in),
		}

		for j := range assignment.InLength {
			assignment.InLength[j] = len(c.in[j]) / 32
		}

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			assert.NoError(t, test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField()))
		})
	}

}

type testE2ECircuit struct {
	In       [][][32]frontend.Variable
	InLength []frontend.Variable

	Hash [][32]frontend.Variable

	WizardVerifier *wizard.VerifierCircuit
}

func (c *testE2ECircuit) Define(api frontend.API) error {
	hsh := Hasher{
		api:     api,
		nbLanes: len(c.WizardVerifier.GetColumn("Lane")),
	}
	for i := range c.In {
		// since TestCreateCols has already checked that different kinds of padding generate the same columns,
		// we can just apply static (or any other single) padding so as not to run the wizard verifier too many times.
		res := hsh.Sum(nil, c.In[i]...)
		internal.AssertSliceEquals(api, c.Hash[i][:], res[:])
	}

	return hsh.Finalize(c.WizardVerifier)
}

// createCols should fail if it runs out of lanes
func TestCreateColsBoundaryChecks(t *testing.T) {

	const blockNbBytes = lanesPerBlock * 8

	for i, c := range []struct {
		maxNbLanes int
		inLength   []int
	}{
		{1, []int{1, 1}},  // fail
		{34, []int{1}},    // pass
		{34, []int{1, 1}}, // pass
		{17, []int{2}},    // pass
		{17, []int{1, 2}}, // pass
		{34, []int{1, 5}}, // fail
	} {

		circuit := testCreateColsBoundaryChecks{
			maxNbLanes: c.maxNbLanes,
			InLength:   make([]frontend.Variable, len(c.inLength)),
		}

		nbNeededLanes := 0
		for _, l := range c.inLength {
			circuit.maxInLength = max(circuit.maxInLength, l)
			nbNeededLanes += (l*32 + blockNbBytes - 1) / blockNbBytes * lanesPerBlock
		}

		fail := nbNeededLanes > c.maxNbLanes

		t.Run(fmt.Sprintf("%d-%s", i, utils.Ite(fail, "fail", "pass")), func(t *testing.T) {

			assignment := testCreateColsBoundaryChecks{
				InLength: utils.ToVariableSlice(c.inLength),
			}

			err := test.IsSolved(&circuit, &assignment, ecc.BLS12_377.ScalarField())
			if fail { // fail
				assert.Error(t, err)
			} else { // pass
				assert.NoError(t, err)
			}
		})
	}
}

type testCreateColsBoundaryChecks struct {
	maxNbLanes  int
	InLength    []frontend.Variable
	maxInLength int
}

func (c *testCreateColsBoundaryChecks) Define(api frontend.API) error {
	hsh := Hasher{
		api:     api,
		nbLanes: c.maxNbLanes,
	}

	for _, l := range c.InLength {
		api.AssertIsLessOrEqual(l, c.maxInLength)
		inBlocks := make([][32]frontend.Variable, c.maxInLength)
		for j := range inBlocks {
			for k := range inBlocks[j] {
				inBlocks[j][k] = 0
			}
		}
		hsh.Sum(l, inBlocks...)
	}

	hsh.createColumns()
	return nil
}

func TestPadDirtyLanes(t *testing.T) {
	test_utils.SnarkFunctionTest(func(api frontend.API) []frontend.Variable {
		padded, _ := pad(api, []frontend.Variable{1, 0x123456}, 1)
		return padded
	}, 1, 0x100000000000000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x80)(t)
}
