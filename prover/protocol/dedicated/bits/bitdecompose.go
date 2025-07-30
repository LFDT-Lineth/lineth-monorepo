package bits

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// BitDecomposed represents the output of a bit decomposition of
// a column. The struct implements the [wizard.ProverAction] interface
// to self-assign itself.
type BitDecomposed struct {
	// Packed is the input of the bit-decomposition
	Packed ifaces.Column
	// Bits lists the decomposed bits of the "packed" column in LSbit
	// order.
	Bits []ifaces.Column
}

// BitDecompose generates a bit decomposition of a column and returns
// a struct that implements the [wizard.ProverAction] interface to
// self-assign itself.
func BitDecompose(comp *wizard.CompiledIOP, packed ifaces.Column, numBits int) *BitDecomposed {

	var (
		round   = packed.Round()
		bitExpr = []*symbolic.Expression{}
		bd      = &BitDecomposed{
			Packed: packed,
			Bits:   make([]ifaces.Column, numBits),
		}
	)

	for i := 0; i < numBits; i++ {
		bd.Bits[i] = comp.InsertCommit(round, ifaces.ColIDf("%v_BIT_%v", packed.GetColID(), i), packed.Size())
		MustBeBoolean(comp, bd.Bits[i])
		bitExpr = append(bitExpr, symbolic.NewVariable(bd.Bits[i]))
	}

	// This constraint ensures that the recombined bits are equal to the
	// original column.
	comp.InsertGlobal(
		round,
		ifaces.QueryID(packed.GetColID())+"_BIT_RECOMBINATION",
		symbolic.Sub(
			packed,
			symbolic.NewPolyEval(symbolic.NewConstant(2), bitExpr),
		),
	)

	return bd
}

// Run implements the [wizard.ProverAction] interface and assigns the bits
// columns
func (bd *BitDecomposed) Run(run *wizard.ProverRuntime) {

	v := bd.Packed.GetColAssignment(run)
	bits := make([][]field.Element, len(bd.Bits))

	for x := range v.IterateCompact() {

		if !x.IsUint64() {
			panic("can handle 64 bits at most")
		}

		x := x.Uint64()

		for i := range bd.Bits {
			if x>>i&1 == 1 {
				bits[i] = append(bits[i], field.One())
			} else {
				bits[i] = append(bits[i], field.Zero())
			}
		}
	}

	for i := range bd.Bits {
		run.AssignColumn(bd.Bits[i].GetColID(), smartvectors.FromCompactWithShape(v, bits[i]))
	}
}

// MustBeBoolean adds a constraint ensuring that the input is a boolean
// column. The constraint is named after the column.
func MustBeBoolean(comp *wizard.CompiledIOP, col ifaces.Column) {
	// This adds the constraint x^2 = x
	comp.InsertGlobal(
		col.Round(),
		ifaces.QueryID(col.GetColID())+"_IS_BOOLEAN",
		symbolic.Sub(col, symbolic.Mul(col, col)))
}
