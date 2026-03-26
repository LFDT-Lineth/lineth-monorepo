package merkle

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
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
			Leaf:    field.RandomOctuplet()},
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

	var (
		leaf, roots [blockSize]ifaces.Column
		position    [limbPerU64]ifaces.Column
	)

	proofs := *NewProof(b.CompiledIOP, 0, "test", merkleTestDepth, merkleTestNumRow)

	for i := 0; i < blockSize; i++ {
		leaf[i] = b.RegisterCommit(ifaces.ColIDf("LEAF_%v", i), merkleTestNumRow)
		roots[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), merkleTestNumRow)
	}
	for i := 0; i < limbPerU64; i++ {
		position[i] = b.RegisterCommit(ifaces.ColIDf("POS_LIMB_%v", i), merkleTestNumRow)
	}
	mpvInputs := FlatProofVerificationInputs{
		Name:     "test",
		Proof:    proofs,
		Leaf:     leaf,
		Roots:    roots,
		Position: position,
		IsActive: b.RegisterCommit("ACTIVE", merkleTestNumRow),
	}

	ctx.ctx = CheckFlatMerkleProofs(b.CompiledIOP, mpvInputs)
}

// Assign assigns the merkle tree test-cases at runtime
func (ctx *merkleTestRunnerFlat) Assign(run *wizard.ProverRuntime, data *merkleTestBuilder) {
	for i := 0; i < blockSize; i++ {
		run.AssignColumn(ifaces.ColIDf("LEAF_%v", i), smartvectors.RightZeroPadded(data.leaves[i], merkleTestNumRow))
		run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.RightZeroPadded(data.roots[i], merkleTestNumRow))
	}
	for i := 0; i < limbPerU64; i++ {
		run.AssignColumn(ifaces.ColIDf("POS_LIMB_%v", i), smartvectors.RightZeroPadded(data.pos[i], merkleTestNumRow))
	}
	run.AssignColumn("ACTIVE", smartvectors.RightZeroPadded(data.isActive, merkleTestNumRow))

	ctx.ctx.Proof.Assign(run, data.proofs)
	ctx.ctx.Run(run)

	for i := 0; i < merkleTestNumRow; i++ {
		for l := 0; l < merkleTestDepth; l++ {

			var left, right, node [blockSize]field.Element

			for k := 0; k < blockSize; k++ {
				left[k] = ctx.ctx.Lefts[l][k].Result.GetColAssignmentAt(run, i)
				right[k] = ctx.ctx.Rights[l][k].Result.GetColAssignmentAt(run, i)
				node[k] = ctx.ctx.Nodes[l].Result()[k].GetColAssignmentAt(run, i)
				fmt.Printf("proof=%v level=%v left=%v right=%v node=%v\n", i, l, left[k].Text(16), right[k].Text(16), node[k].Text(16))
			}

		}
	}
}

// merkleTestCaseInstance represents either a read or a write operation to add to
// the test-cases.
type merkleTestCaseInstance struct {
	IsWrite bool
	Pos     int
	// Leaf is only taken into consideration if Write is true
	Leaf field.Octuplet
}

// merkleTestBuilder is used to build the assignment of merkle proofs
// and is implemented like a writer.
type merkleTestBuilder struct {
	proofs             []smt_koalabear.Proof
	pos                [limbPerU64][]field.Element
	roots              [blockSize][]field.Element
	leaves             [blockSize][]field.Element
	useNextMerkleProof []field.Element
	isActive           []field.Element
	counter            []field.Element
	tree               smt_koalabear.Tree
}

// merkleTestBuilderRow is a pure-data structure specifying a row in the test builder
type merkleTestBuilderRow struct {
	proof              smt_koalabear.Proof
	pos                int
	leaf               field.Octuplet
	root               field.Octuplet
	useNextMerkleProof bool
}

func newMerkleTestBuilder(depth int) *merkleTestBuilder {
	return &merkleTestBuilder{
		tree: *smt_koalabear.NewTree(make([]field.Octuplet, 1<<depth)),
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

func (mt *merkleTestBuilder) AddWrite(pos int, newLeaf field.Octuplet) {

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
	leafOct := row.leaf
	rootOct := row.root
	for i := 0; i < blockSize; i++ {
		mt.leaves[i] = append(mt.leaves[i], leafOct[i])
		mt.roots[i] = append(mt.roots[i], rootOct[i])
	}
	// compute position limbs
	limbs := uint64To4BitLimbs(uint64(row.pos))
	for i := 0; i < limbPerU64; i++ {
		mt.pos[i] = append(mt.pos[i], field.NewElement(limbs[i]))

	}
	mt.useNextMerkleProof = append(mt.useNextMerkleProof, field.FromBool(row.useNextMerkleProof))
	mt.isActive = append(mt.isActive, field.One())
}

// uint64To4BitLimbs splits v into four 16-bit limbs (big-endian order):
// limbs[15] = lowest 16 bits, limbs[0] = highest 16 bits.
func uint64To4BitLimbs(v uint64) [16]uint64 {
	var limbs [16]uint64
	limbs[15] = uint64(v & 0xF)
	limbs[14] = uint64((v >> 4) & 0xF)
	limbs[13] = uint64((v >> 8) & 0xF)
	limbs[12] = uint64((v >> 12) & 0xF)
	limbs[11] = uint64((v >> 16) & 0xF)
	limbs[10] = uint64((v >> 20) & 0xF)
	limbs[9] = uint64((v >> 24) & 0xF)
	limbs[8] = uint64((v >> 28) & 0xF)
	limbs[7] = uint64((v >> 32) & 0xF)
	limbs[6] = uint64((v >> 36) & 0xF)
	limbs[5] = uint64((v >> 40) & 0xF)
	limbs[4] = uint64((v >> 44) & 0xF)
	limbs[3] = uint64((v >> 48) & 0xF)
	limbs[2] = uint64((v >> 52) & 0xF)
	limbs[1] = uint64((v >> 56) & 0xF)
	limbs[0] = uint64((v >> 60) & 0xF)
	return limbs
}
