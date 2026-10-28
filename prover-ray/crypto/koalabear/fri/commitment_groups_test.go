package fri

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// groupedWitness builds a batch whose sizes 0 and 1 are revealed above the
// depth-2 input cap and whose size 2 is opened on every query path, each split
// into two groups; the largest size, 3, is a single group.
func groupedWitness(prng *rand.Rand) Batch {
	return Batch{
		{
			Base:   [][]field.Element{field.VecPseudoRandBase(prng, 1), field.VecPseudoRandBase(prng, 1)},
			Groups: []GroupShape{{BaseWidth: 1, Ordinal: 0}, {BaseWidth: 1, Ordinal: 1}},
		},
		{
			Base:   [][]field.Element{field.VecPseudoRandBase(prng, 2)},
			Ext:    [][]field.Ext{field.VecPseudoRandExt(prng, 2)},
			Groups: []GroupShape{{BaseWidth: 1, Ordinal: 2}, {ExtWidth: 1, Ordinal: 3}},
		},
		{
			Base:   [][]field.Element{field.VecPseudoRandBase(prng, 4), field.VecPseudoRandBase(prng, 4)},
			Ext:    [][]field.Ext{field.VecPseudoRandExt(prng, 4)},
			Groups: []GroupShape{{BaseWidth: 1, ExtWidth: 1, Ordinal: 4}, {BaseWidth: 1, Ordinal: 5}},
		},
		{
			Ext:    [][]field.Ext{field.VecPseudoRandExt(prng, 8)},
			Groups: []GroupShape{{ExtWidth: 1, Ordinal: 6}},
		},
	}
}

// ungrouped returns a copy of shape without its groups.
func ungrouped(shape Shape) Shape {
	res := make(Shape, len(shape))
	for i, sized := range shape {
		res[i] = SizedShape{BaseWidth: sized.BaseWidth, ExtWidth: sized.ExtWidth}
	}
	return res
}

// newGroupedPCSFixture commits the grouped witness, opens it as numBatches
// batches sharing one input tree at four queries, and checks the native
// verifier accepts.
func newGroupedPCSFixture(t *testing.T, numBatches int) pcsOpenVerifyFixture {
	t.Helper()
	params, err := NewParams(4, 3, 4)
	require.NoError(t, err)
	params.MerkleCapHeight = 2
	pcs, err := NewPCS(params, makeEncoders(4, 2))
	require.NoError(t, err)
	witness := groupedWitness(rand.New(utils.NewRandSource(20261021)))
	committed := pcs.Commit(witness)

	shifts := BatchShifts{
		{Base: [][]int{{0}, {0}}},
		{Base: [][]int{{0}}, Ext: [][]int{{0}}},
		{Base: [][]int{{0}, {1}}, Ext: [][]int{{0}}},
		{Ext: [][]int{{0}}},
	}
	challenges := Challenges{
		FoldAlphas: []field.Ext{
			field.UintsToExt(7, 1, 0, 0, 0, 0),
			field.UintsToExt(11, 0, 1, 0, 0, 0),
			field.UintsToExt(13, 0, 0, 1, 0, 0),
		},
		QueryPositions: []int{5, 9, 2, 14},
	}
	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)
	repeat := func(n int) []int { return make([]int, n) }
	proof, claimed := openForTest(t, pcs, openInputs{
		Witnesses:  utils.Map(func(int) Batch { return witness }, repeat(numBatches)),
		Committed:  utils.Map(func(int) CommitterState { return committed }, repeat(numBatches)),
		Shifts:     utils.Map(func(int) BatchShifts { return shifts }, repeat(numBatches)),
		Zeta:       zeta,
		Challenges: challenges,
	})
	shape := witness.Shape()
	fx := pcsOpenVerifyFixture{
		pcs:       pcs,
		committed: []CommitterState{committed},
		input: VerifyInputs{
			Roots:         utils.Map(func(int) field.Octuplet { return committed.Tree.Root() }, repeat(numBatches)),
			Shapes:        utils.Map(func(int) Shape { return shape }, repeat(numBatches)),
			Shifts:        utils.Map(func(int) BatchShifts { return shifts }, repeat(numBatches)),
			ClaimedValues: claimed,
			Zeta:          zeta,
			Challenges:    challenges,
		},
		proof: proof,
	}
	require.NoError(t, pcs.Verify(fx.input, fx.proof), "native verifier must accept the grouped fixture")
	return fx
}

// referenceGroupDigest is the leaf digest of rows (2j, 2j+1) of one group,
// written out from the leaf format: header, then both rows.
func referenceGroupDigest(group SizedTable, j int) field.Octuplet {
	h := poseidon2.NewMDHasher()
	absorbLeafHeader(h, len(group.Base), len(group.Ext))
	for _, row := range []int{2 * j, 2*j + 1} {
		for _, col := range group.Base {
			h.WriteElements(col[row])
		}
		for _, col := range group.Ext {
			limbs := extLimbs(col[row])
			h.WriteElements(limbs[:]...)
		}
	}
	return h.SumDigest()
}

// TestMerkleizeInjectsEachGroup checks the grouped tree against its definition:
// every group of a size is digested with its own header and injected into the
// size's level in group order, and the largest size, a single group here, is
// digested as row pairs into the leaves. The grouping must change the root.
func TestMerkleizeInjectsEachGroup(t *testing.T) {
	witness := groupedWitness(rand.New(utils.NewRandSource(5)))
	encoded := witness.Encode(makeEncoders(4, 2))
	tree := encoded.Merkleize()

	bottom := encoded[len(encoded)-1]
	leaves := make([]field.Octuplet, bottom.Size()/2)
	for j := range leaves {
		leaves[j] = referenceGroupDigest(bottom, j)
	}
	injections := make([][][]field.Octuplet, utils.Log2Ceil(len(leaves)))
	for i := range len(encoded) - 1 {
		level := utils.Log2Ceil(encoded[i].Size() / 2)
		var base, ext int
		for _, g := range encoded[i].Groups {
			group := SizedTable{Base: encoded[i].Base[base : base+g.BaseWidth], Ext: encoded[i].Ext[ext : ext+g.ExtWidth]}
			base += g.BaseWidth
			ext += g.ExtWidth
			digests := make([]field.Octuplet, encoded[i].Size()/2)
			for j := range digests {
				digests[j] = referenceGroupDigest(group, j)
			}
			injections[level] = append(injections[level], digests)
		}
	}
	want := allocTree(len(leaves))
	copy(want.Nodes[len(leaves)-1:], leaves)
	want.buildLevels(injections)
	require.Equal(t, want.Root(), tree.Root(), "grouped root must inject one digest per group")
	require.Equal(t, bottom.Size()/2, tree.NumLeaves(), "a grouped tree's leaves digest row pairs")

	flat := make(MultiSizeTable, len(encoded))
	for i := range encoded {
		flat[i] = SizedTable{Base: encoded[i].Base, Ext: encoded[i].Ext}
	}
	require.NotEqual(t, flat.Merkleize().Root(), tree.Root(), "the grouping must change the commitment")
}

// TestPCSVerifyGroupedCommitment opens a grouped commitment through both the
// revealed-table and the query-path authentication, and checks that the
// verifier rejects any other grouping, including no grouping, and batches that
// share the tree but disagree on its grouping.
func TestPCSVerifyGroupedCommitment(t *testing.T) {
	fx := newGroupedPCSFixture(t, 2)

	info, err := inputCapShapeInfo(fx.pcs.Params, fx.input.Shapes[0])
	require.NoError(t, err)
	require.Equal(t, []int{0, 1}, info.revealed, "sizes 0 and 1 must be revealed above the cap")
	require.True(t, info.queryRows[2], "size 2 must be opened on the query path")
	require.Len(t, info.levelGroups[2], 2, "the query-path size must hold several groups")

	withShapes := func(shapes ...Shape) VerifyInputs {
		in := fx.input
		in.Shapes = shapes
		return in
	}
	flat := ungrouped(fx.input.Shapes[0])
	require.Error(t, fx.pcs.Verify(withShapes(flat, flat), fx.proof), "an ungrouped shape must not verify")

	swapped := append(Shape(nil), fx.input.Shapes[0]...)
	swapped[2].Groups = []GroupShape{{BaseWidth: 1, Ordinal: 4}, {BaseWidth: 1, ExtWidth: 1, Ordinal: 5}}
	require.Error(t, fx.pcs.Verify(withShapes(swapped, swapped), fx.proof), "another grouping must not verify")

	require.ErrorContains(t, fx.pcs.Verify(withShapes(fx.input.Shapes[0], swapped), fx.proof),
		"groups its rows differently")
	require.ErrorContains(t, fx.pcs.Verify(withShapes(fx.input.Shapes[0], flat), fx.proof),
		"declares no groups", "a grouped and an ungrouped batch cannot be numbered together")
}

// TestPCSVerifyGnarkGroupedCommitment is the circuit counterpart: the circuit
// accepts the grouped proof and rejects it under no grouping. The circuit gives
// every batch its own input tree, so the fixture has a single batch.
func TestPCSVerifyGnarkGroupedCommitment(t *testing.T) {
	fx := newGroupedPCSFixture(t, 1)
	template, assignment := circuitFor(fx, fx.proof)
	require.NoError(t, solvePCSCircuit(t, template, assignment), "grouped proof must satisfy the verifier circuit")

	fx.input.Shapes = []Shape{ungrouped(fx.input.Shapes[0])}
	template, assignment = circuitFor(fx, fx.proof)
	require.Error(t, solvePCSCircuit(t, template, assignment), "an ungrouped shape must not satisfy the circuit")
}

// TestGroupsAreValidated checks that groups must split their size exactly into
// nonempty groups, at every size including the largest, at commit and at
// verification alike.
func TestGroupsAreValidated(t *testing.T) {
	params, err := NewParams(4, 3, 4)
	require.NoError(t, err)
	encoders := makeEncoders(4, 2)

	invalid := map[string]func(Batch){
		"rows left out": func(b Batch) { b[2].Groups = []GroupShape{{BaseWidth: 1, ExtWidth: 1}} },
		"empty group":   func(b Batch) { b[2].Groups = append(b[2].Groups, GroupShape{}) },
		"too many rows": func(b Batch) { b[1].Groups = []GroupShape{{BaseWidth: 2}, {ExtWidth: 1}} },
		"largest size, rows left out": func(b Batch) {
			b[3].Ext = append(b[3].Ext, b[3].Ext[0])
			b[3].Groups = []GroupShape{{ExtWidth: 1}}
		},
	}
	for name, corrupt := range invalid {
		witness := groupedWitness(rand.New(utils.NewRandSource(6)))
		corrupt(witness)
		require.Panics(t, func() { Commit(encoders, witness) }, name)
		_, err := inputCapShapeInfo(params, witness.Shape())
		require.Error(t, err, name)
	}
}

// TestPCSVerifyRejectsMalformedRowPair checks that an opened pair whose rows do
// not have the widths of their groups is rejected with an error, not a panic,
// on the query path, which authenticates the pair before checking its shape.
func TestPCSVerifyRejectsMalformedRowPair(t *testing.T) {
	fx := newGroupedPCSFixture(t, 1)
	level := 2 // the query-path size, split into two groups

	malformed := map[string]func(*RowPair){
		"short self row":    func(p *RowPair) { p[0].Base = p[0].Base[:1] },
		"short sibling row": func(p *RowPair) { p[1].Ext = nil },
		"long sibling row":  func(p *RowPair) { p[1].Base = append(p[1].Base, p[1].Base[0]) },
	}
	for name, corrupt := range malformed {
		proof := fx.proof
		proof.InputQueries = append([]InputQuery(nil), fx.proof.InputQueries...)
		proof.InputQueries[0] = append(InputQuery(nil), fx.proof.InputQueries[0]...)
		opening := proof.InputQueries[0][0]
		opening.Leaves = append([]*RowPair(nil), opening.Leaves...)
		pair := *opening.Leaves[level]
		pair[0].Base = append([]field.Element(nil), pair[0].Base...)
		pair[1].Base = append([]field.Element(nil), pair[1].Base...)
		corrupt(&pair)
		opening.Leaves[level] = &pair
		proof.InputQueries[0][0] = opening

		require.NotPanics(t, func() {
			require.Error(t, fx.pcs.Verify(fx.input, proof), name)
		}, name)
	}
}
