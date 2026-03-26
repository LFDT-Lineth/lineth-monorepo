package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

// TwoByTwoCombination combines every two columns into one column by combining every two consecutive bytes into one in big-endian order.
type TwoByTwoCombination struct {
	Cols            []ifaces.Column
	CombinationCols []ifaces.Column
}

// it combines two columns in one column by combining every two consecutive bytes into one in big-endian order.
func NewTwoByTwoCombination(comp *wizard.CompiledIOP, cols []ifaces.Column) *TwoByTwoCombination {
	if len(cols)%2 != 0 {
		panic("number of columns should be even")
	}

	result := make([]ifaces.Column, len(cols)/2)
	for i := 0; i < len(cols); i += 2 {
		result[i/2] = combineTwoColumnsToOne(comp, cols[i], cols[i+1])
	}
	return &TwoByTwoCombination{
		Cols:            cols,
		CombinationCols: result,
	}
}

// Run performs the combination of every two columns into one column.
func (t *TwoByTwoCombination) Run(run *wizard.ProverRuntime) {

	var (
		cols         = make([][]field.Element, len(t.Cols))
		combinedCols = make([]*VectorBuilder, len(t.CombinationCols))
		byteFr       = field.NewElement(256)
	)

	for i, col := range t.Cols {
		cols[i] = col.GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	for i := range t.CombinationCols {
		combinedCols[i] = NewVectorBuilder(t.CombinationCols[i])
	}

	// combine every two columns
	res := make([]field.Element, t.CombinationCols[0].Size())
	for i := 0; i < len(t.Cols); i += 2 {
		vector.ScalarMul(res, cols[i], byteFr)
		vector.Add(res, res, cols[i+1])
		combinedCols[i/2].PushSliceF(res)
		combinedCols[i/2].PadAndAssign(run)
	}

}

// combineTwoColumnsToOne combines two columns into one in big-endian order.
func combineTwoColumnsToOne(comp *wizard.CompiledIOP, col1, col2 ifaces.Column) ifaces.Column {
	if col1.Size() != col2.Size() {
		panic("columns should have the same size")
	}

	size := col1.Size()
	newCol := comp.InsertCommit(
		0,
		ifaces.ColIDf("COMBINED_%v_%v", col1.GetColID(), col2.GetColID()),
		size,
		true,
	)

	// check that the columns have been combined correctly
	// i.e., newCol[i] = col1[i] * 256 + col2[i] // big-endian
	comp.InsertGlobal(0,
		ifaces.QueryIDf("COMBINE_TWO_COLUMNS_%v_%v", col1.GetColID(), col2.GetColID()),
		sym.Sub(
			newCol,
			sym.Add(
				sym.Mul(col1, 256),
				col2,
			),
		),
	)

	return newCol
}
