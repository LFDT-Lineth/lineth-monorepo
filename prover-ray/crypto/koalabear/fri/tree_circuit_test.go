package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/stretchr/testify/require"
)

// This file checks the in-circuit Merkle capping primitives of tree_circuit.go
// against the native ones in tree.go. The circuit mirrors are only meaningful
// if they accept exactly what the native verifier accepts, so every case here
// runs both sides on the same data.
//
// The negative cases matter as much as the positive ones. Capping is the first
// place in this verifier where a circuit *variable* selects among
// prover-supplied values (the frontier multiplexer), so an under-constrained
// mirror would still accept every honest proof while authenticating nothing.

// capCircuit constrains a capped branch against a cap and a trusted root,
// exactly as the running-tree path of VerifyGnark does.
type capCircuit struct {
	Cap      GnarkMerkleCap
	Branch   GnarkBranch
	Root     poseidon2.KoalagnarkOctuplet
	IdxBits  []frontend.Variable
	capDepth int // structural, not a witness value
}

func (c *capCircuit) Define(api frontend.API) error {
	f := circuit.NewAPI(api)
	frontier := authenticateCapGnark(f, c.Cap, c.capDepth, c.Root)
	authenticateToCapGnark(f, c.Branch, c.IdxBits, frontier)
	return nil
}

// capWitness builds a tree, opens leaf idx to a depth-capDepth frontier, and
// returns the native pieces plus an assigned circuit.
type capWitness struct {
	tree   *Tree
	cap    MerkleCap
	branch Branch
	idx    int
}

func newCapWitness(t *testing.T, prng *rand.Rand, numLeaves, capDepth, idx int) capWitness {
	t.Helper()
	// One level per tree height, only the bottom one populated: that is the
	// aux-free shape the running FRI trees have.
	levels := make([][]field.Octuplet, utils.Log2Ceil(numLeaves)+1)
	levels[len(levels)-1] = pseudoRandOctuplets(prng, numLeaves)
	tree := NewTree(levels)
	w := capWitness{
		tree:   tree,
		cap:    tree.OpenCap(capDepth),
		branch: tree.OpenBranchToDepth(idx, capDepth),
		idx:    idx,
	}

	// The native side must accept, or the fixture is wrong rather than the
	// circuit.
	require.NoError(t, w.cap.Authenticate(capDepth, tree.Root()), "native cap must authenticate")
	frontier := w.frontier()
	require.NoError(t, w.branch.AuthenticateToCap(idx, frontier), "native branch must authenticate")
	return w
}

// frontier mirrors the verifier's choice: a depth-zero cap stands for the root.
func (w capWitness) frontier() []field.Octuplet {
	if len(w.cap.Nodes) == 0 {
		return []field.Octuplet{w.tree.Root()}
	}
	return w.cap.Nodes
}

// assign builds the circuit assignment. numBits must cover the fold levels
// plus the frontier selector bits.
func (w capWitness) assign(capDepth, numBits int) *capCircuit {
	idxBits := make([]frontend.Variable, numBits)
	for i := range idxBits {
		idxBits[i] = (w.idx >> i) & 1
	}
	return &capCircuit{
		Cap:      NewGnarkMerkleCap(w.cap),
		Branch:   GnarkBranch{Leaf: convertOctuplet(w.branch.Leaf, true), Siblings: convertOctuplets(w.branch.Siblings, true)},
		Root:     poseidon2.NewKoalagnarkOctuplet(w.tree.Root()),
		IdxBits:  idxBits,
		capDepth: capDepth,
	}
}

// template returns the unassigned circuit with the same shape.
func (w capWitness) template(capDepth, numBits int) *capCircuit {
	return &capCircuit{
		Cap:      AllocateGnarkMerkleCap(w.cap),
		Branch:   GnarkBranch{Siblings: make([]poseidon2.KoalagnarkOctuplet, len(w.branch.Siblings))},
		IdxBits:  make([]frontend.Variable, numBits),
		capDepth: capDepth,
	}
}

// solveCap compiles the circuit and reports whether the assignment satisfies it.
func solveCap(t *testing.T, template, assignment *capCircuit, emulated bool) error {
	t.Helper()

	// CompileU32 and Compile return different constraint-system types; both
	// answer IsSolved, which is all this helper needs.
	type solvable interface {
		IsSolved(witness.Witness, ...solver.Option) error
	}
	var (
		ccs     solvable
		modulus = koalabear.Modulus()
		err     error
	)
	if emulated {
		modulus = ecc.BLS12_377.ScalarField()
		ccs, err = frontend.Compile(modulus, scs.NewBuilder, template)
	} else {
		ccs, err = frontend.CompileU32(modulus, scs.NewBuilder, template)
	}
	require.NoError(t, err, "circuit must compile")

	w, err := frontend.NewWitness(assignment, modulus)
	require.NoError(t, err, "witness must build")
	return ccs.IsSolved(w)
}

// TestCapCircuitMatchesNative checks the honest path across the cap depths the
// verifier actually uses, in both native and emulated modes.
func TestCapCircuitMatchesNative(t *testing.T) {
	prng := rand.New(utils.NewRandSource(7))

	cases := []struct {
		name      string
		numLeaves int
		capDepth  int
		idx       int
	}{
		{"depth0-root-only", 8, 0, 5},
		{"depth1", 8, 1, 5},
		{"depth2", 8, 2, 3},
		{"depth2-wide", 32, 2, 21},
		{"depth3", 32, 3, 17},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			height := utils.Log2Ceil(tc.numLeaves)
			w := newCapWitness(t, prng, tc.numLeaves, tc.capDepth, tc.idx)
			require.Len(t, w.branch.Siblings, height-tc.capDepth,
				"capped branch must drop its top siblings")

			require.NoError(t, solveCap(t, w.template(tc.capDepth, height), w.assign(tc.capDepth, height), false),
				"honest capped opening must satisfy the circuit (native)")
			require.NoError(t, solveCap(t, w.template(tc.capDepth, height), w.assign(tc.capDepth, height), true),
				"honest capped opening must satisfy the circuit (emulated)")
		})
	}
}

// TestCapCircuitRejectsCorruptedFrontier is the soundness direction: a cap node
// the branch lands on must be constrained, not merely read. Natively this is
// caught by the cap failing to reconstruct the root.
func TestCapCircuitRejectsCorruptedFrontier(t *testing.T) {
	const (
		numLeaves = 32
		capDepth  = 2
		idx       = 21
	)
	prng := rand.New(utils.NewRandSource(11))
	height := utils.Log2Ceil(numLeaves)
	w := newCapWitness(t, prng, numLeaves, capDepth, idx)

	// Corrupting any frontier node breaks the cap's reconstruction of the root.
	for node := range w.cap.Nodes {
		tampered := w
		tampered.cap = MerkleCap{Nodes: append([]field.Octuplet(nil), w.cap.Nodes...), Aux: w.cap.Aux}
		tampered.cap.Nodes[node][0] = field.PseudoRandOctuplet(prng)[0]

		require.Error(t, tampered.cap.Authenticate(capDepth, w.tree.Root()),
			"native verifier must reject a corrupted frontier node %d", node)
		require.Error(t, solveCap(t, w.template(capDepth, height), tampered.assign(capDepth, height), false),
			"circuit must reject a corrupted frontier node %d", node)
	}
}

// TestCapCircuitRejectsWrongFrontierSlot checks the multiplexer itself: a branch
// authenticated against the wrong frontier node must fail. Flipping a selector
// bit keeps every hash in the fold valid and only moves the frontier lookup, so
// a circuit that muxes on the wrong bits — or skips the equality — still passes
// every honest case but fails here.
func TestCapCircuitRejectsWrongFrontierSlot(t *testing.T) {
	const (
		numLeaves = 32
		capDepth  = 2
		idx       = 21
	)
	prng := rand.New(utils.NewRandSource(13))
	height := utils.Log2Ceil(numLeaves)
	w := newCapWitness(t, prng, numLeaves, capDepth, idx)
	numSiblings := len(w.branch.Siblings)

	for bit := numSiblings; bit < height; bit++ {
		assignment := w.assign(capDepth, height)
		cur, ok := assignment.IdxBits[bit].(int)
		require.True(t, ok, "index bits are assigned as ints")
		assignment.IdxBits[bit] = 1 - cur

		// The native check is the same statement: the branch folds to a
		// different frontier slot than the one it belongs to.
		flipped := idx ^ (1 << bit)
		require.Error(t, w.branch.AuthenticateToCap(flipped, w.frontier()),
			"native verifier must reject frontier slot bit %d flipped", bit)
		require.Error(t, solveCap(t, w.template(capDepth, height), assignment, false),
			"circuit must reject frontier slot bit %d flipped", bit)
	}
}

// TestCapCircuitRejectsTamperedSibling covers the fold below the frontier,
// which capping shortened but did not change in kind.
func TestCapCircuitRejectsTamperedSibling(t *testing.T) {
	const (
		numLeaves = 32
		capDepth  = 2
		idx       = 21
	)
	prng := rand.New(utils.NewRandSource(17))
	height := utils.Log2Ceil(numLeaves)
	w := newCapWitness(t, prng, numLeaves, capDepth, idx)

	for sib := range w.branch.Siblings {
		tampered := w
		tampered.branch = Branch{
			Leaf:        w.branch.Leaf,
			Siblings:    append([]field.Octuplet(nil), w.branch.Siblings...),
			AuxSiblings: w.branch.AuxSiblings,
		}
		tampered.branch.Siblings[sib] = field.PseudoRandOctuplet(prng)

		require.Error(t, tampered.branch.AuthenticateToCap(idx, w.frontier()),
			"native verifier must reject a tampered sibling %d", sib)
		require.Error(t, solveCap(t, w.template(capDepth, height), tampered.assign(capDepth, height), false),
			"circuit must reject a tampered sibling %d", sib)
	}
}

// TestMerkleCapDepthMatchesParams pins the depth schedule the circuit assumes
// structurally: the circuit reads cap sizes from the allocated witness, so it
// must agree with what the prover emits.
func TestMerkleCapDepthMatchesParams(t *testing.T) {
	for _, numQueries := range []uint{1, 2, 3, 4, 8, 16} {
		for height := 1; height <= 6; height++ {
			depth := merkleCapDepth(numQueries, height)
			require.GreaterOrEqual(t, depth, 0)
			require.LessOrEqual(t, depth, height-1, "cap must leave at least one branch step")
		}
	}
}
