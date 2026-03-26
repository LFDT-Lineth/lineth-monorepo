package merkle

import (
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// Module summarizing the Merkle proof claims
type ResultMod struct {

	// the compiled IOP
	comp *wizard.CompiledIOP

	// Number of rows in the module
	NumRows int
	// Number of proofs
	NumProofs int
	// Name if the name of parent context joined with a specifier.
	Name string
	// Round of the module
	Round int
	// Depth of the proof
	Depth int

	// Leaf contains the alleged leaves
	Leaf [blockSize]ifaces.Column
	// Roots contains the Merkle roots
	Roots [blockSize]ifaces.Column
	// Pos contains the positions of the alleged leaves
	Pos ifaces.Column
	// Use for looking up and selecting only the
	// the columns containing the root in the ComputeMod
	IsActive ifaces.Column
	// Column used to verify reuse of Merkle proofs
	UseNextMerkleProof ifaces.Column
	// Column to verify the sequentiality of Merkle proofs
	Counter ifaces.Column
	// variable denoting whether we want to reuse the Merkle proofs
	withOptProofReuseCheck bool
}

// Registers all the columns also assumes that Leaf, Roots and Pos have been
// passed already.
func (rm *ResultMod) Define(comp *wizard.CompiledIOP, round int, name string, numProofs int, depth int, useNextMerkleProof ifaces.Column, isActive ifaces.Column, counter ifaces.Column) {

	// Sanity check that the columns have been passed
	for i := 0; i < blockSize; i++ {
		if rm.Leaf[i] == nil || rm.Roots[i] == nil {
			panic("please set all the required columns before calling define")
		}
	}
	if rm.Pos == nil {
		panic("please set all the required columns before calling define")
	}

	// Sanity check that the depth is consistent
	if rm.Depth != depth {
		panic("there is an inconsitency in the assignment of the depth of the Merkle proof")
	}

	// Sanity check that they all have the same size
	for i := 0; i < blockSize; i++ {
		if rm.Leaf[i].Size() != rm.Roots[i].Size() || rm.Leaf[i].Size() != rm.Pos.Size() {
			utils.Panic("the sizes of the passed columns should be consistent %v, %v, and %v", rm.Leaf[i].Size(), rm.Roots[i].Size(), rm.Pos.Size())
		}
	}

	// Sanity-check the value of NumRows
	numRows := rm.Roots[0].Size()
	if numRows != utils.NextPowerOfTwo(numProofs) {
		utils.Panic("numRows %v but numProofs %v", numRows, numProofs)
	}

	// Assigns the depth and the number of proofs
	rm.comp = comp
	rm.NumRows = numRows
	rm.NumProofs = numProofs
	rm.Round = round
	rm.Name = strings.Join([]string{"MERKLE", "RESULTMOD", name}, "_")

	// Defines the zero-th column
	if !rm.withOptProofReuseCheck {
		rm.IsActive = rm.comp.InsertPrecomputed(
			rm.colname("IS_ACTIVE_PRECOMP"),
			smartvectors.RightZeroPadded(
				vector.Repeat(field.One(), rm.NumProofs),
				rm.NumRows,
			),
		)
	}

	// Columns registered/redefined to verify reuse of Merkle proof in the Accumulator
	if rm.withOptProofReuseCheck {
		rm.UseNextMerkleProof = useNextMerkleProof
		rm.IsActive = isActive
		rm.Counter = counter
	}

}

func (rm *ResultMod) colname(name string, args ...any) ifaces.ColID {
	return ifaces.ColIDf("MERKLE_%v_%v", rm.Name, rm.comp.SelfRecursionCount) + "_" + ifaces.ColIDf(name, args...)
}
