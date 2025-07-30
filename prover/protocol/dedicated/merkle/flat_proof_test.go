package merkle

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

const (
	merkleTestDepth  = 16
	merkleTestNumRow = 16
)

// testcases is a list of test-cases scenarios for the merkle tree
var testcases = [][]merkleTestCaseInstance{
	{
		{
			IsWrite: true,
			Pos:     0,
			Leaf:    types.Bytes32{1, 2, 3, 4},
		},
		{
			Pos: 1,
		},
	},
}

func TestMerkleTreeFlat(t *testing.T) {

	for i, testcase := range testcases {
		t.Run(fmt.Sprintf("testcase-%v", i), func(t *testing.T) {

			tree := newMerkleTestBuilder(merkleTestDepth)

			for _, instance := range testcase {
				if instance.IsWrite {
					tree.AddWrite(instance.Pos, instance.Leaf)
				} else {
					tree.AddRead(instance.Pos)
				}
			}

			var (
				tr   = &merkleTestRunnerFlat{}
				comp = wizard.Compile(tr.Define, dummy.CompileAtProverLvl())
				_    = wizard.Prove(comp, func(run *wizard.ProverRuntime) { tr.Assign(run, tree) })
			)

		})
	}
}

// merkleTestRunnerFlat is a runner for the merkle tree test-cases
type merkleTestRunnerFlat struct {
	ctx *FlatMerkleProofVerification
}

// Define implements the [wizard.DefineFunc] interface
func (ctx *merkleTestRunnerFlat) Define(b *wizard.Builder) {

	mpvInputs := FlatProofVerificationInputs{
		Name:     "test",
		Proof:    *NewProof(b.CompiledIOP, 0, "test", merkleTestDepth, merkleTestNumRow),
		Leaf:     b.RegisterCommit("LEAF", merkleTestNumRow),
		Roots:    b.RegisterCommit("ROOTS", merkleTestNumRow),
		Position: b.RegisterCommit("POS", merkleTestNumRow),
		IsActive: b.RegisterCommit("ACTIVE", merkleTestNumRow),
	}

	ctx.ctx = CheckFlatMerkleProofs(b.CompiledIOP, mpvInputs)
}

// Assign assigns the merkle tree test-cases at runtime
func (ctx *merkleTestRunnerFlat) Assign(run *wizard.ProverRuntime, data *merkleTestBuilder) {

	run.AssignColumn("LEAF", smartvectors.RightZeroPadded(data.leaves, merkleTestNumRow))
	run.AssignColumn("ROOTS", smartvectors.RightZeroPadded(data.roots, merkleTestNumRow))
	run.AssignColumn("POS", smartvectors.RightZeroPadded(data.pos, merkleTestNumRow))
	run.AssignColumn("ACTIVE", smartvectors.RightZeroPadded(data.isActive, merkleTestNumRow))

	ctx.ctx.Proof.Assign(run, data.proofs)
	ctx.ctx.Run(run)

	for i := 0; i < merkleTestNumRow; i++ {
		for l := 0; l < merkleTestDepth; l++ {

			var (
				left  = ctx.ctx.Lefts[l].Result.GetColAssignmentAt(run, i)
				right = ctx.ctx.Rights[l].Result.GetColAssignmentAt(run, i)
				node  = ctx.ctx.Nodes[l].Result().GetColAssignmentAt(run, i)
			)

			fmt.Printf("proof=%v level=%v left=%v right=%v node=%v\n", i, l, left.Text(16), right.Text(16), node.Text(16))
		}
	}
}

// merkleTestCaseInstance represents either a read or a write operation to add to
// the test-cases.
type merkleTestCaseInstance struct {
	IsWrite bool
	Pos     int
	// Leaf is only taken into consideration if Write is true
	Leaf types.Bytes32
}

// merkleTestBuilder is used to build the assignment of merkle proofs
// and is implemented like a writer.
type merkleTestBuilder struct {
	proofs             []smt.Proof
	pos                []field.Element
	roots              []field.Element
	leaves             []field.Element
	useNextMerkleProof []field.Element
	isActive           []field.Element
	counter            []field.Element
	tree               smt.Tree
}

// merkleTestBuilderRow is a pure-data structure specifying a row in the test builder
type merkleTestBuilderRow struct {
	proof              smt.Proof
	pos                int
	leaf               types.Bytes32
	root               types.Bytes32
	useNextMerkleProof bool
}

func newMerkleTestBuilder(depth int) *merkleTestBuilder {
	return &merkleTestBuilder{
		tree: *smt.BuildComplete(make([]types.Bytes32, 1<<depth), hashtypes.MiMC),
	}
}

func (mt *merkleTestBuilder) AddRead(pos int) {

	var (
		proof              = mt.tree.MustProve(pos)
		leaf, _            = mt.tree.GetLeaf(pos)
		root               = mt.tree.Root
		useNextMerkleProof = false
	)

	mt.pushRow(merkleTestBuilderRow{
		proof:              proof,
		pos:                pos,
		leaf:               leaf,
		root:               root,
		useNextMerkleProof: useNextMerkleProof,
	})
}

func (mt *merkleTestBuilder) AddWrite(pos int, newLeaf types.Bytes32) {

	proof := mt.tree.MustProve(pos)

	mt.pushRow(merkleTestBuilderRow{
		proof:              proof,
		pos:                pos,
		leaf:               mt.tree.MustGetLeaf(pos),
		root:               mt.tree.Root,
		useNextMerkleProof: true,
	})

	mt.tree.Update(pos, newLeaf)

	mt.pushRow(merkleTestBuilderRow{
		proof: proof,
		pos:   pos,
		leaf:  newLeaf,
		root:  mt.tree.Root,
	})
}

// pushRow adds a row to the builder
func (mt *merkleTestBuilder) pushRow(row merkleTestBuilderRow) {
	mt.counter = append(mt.counter, field.NewElement(uint64(len(mt.counter))))
	mt.proofs = append(mt.proofs, row.proof)
	mt.pos = append(mt.pos, field.NewElement(uint64(row.pos)))
	mt.leaves = append(mt.leaves, row.leaf.ToField())
	mt.roots = append(mt.roots, row.root.ToField())
	mt.useNextMerkleProof = append(mt.useNextMerkleProof, field.FromBool(row.useNextMerkleProof))
	mt.isActive = append(mt.isActive, field.One())
}
