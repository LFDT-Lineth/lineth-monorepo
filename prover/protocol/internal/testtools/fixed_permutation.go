package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// FixedPermutationTestcase represents a testcase for a fixed permutation
// query and its assignment.
type FixedPermutationTestcase struct {
	NameStr      string
	A            []smartvectors.SmartVector
	B            []smartvectors.SmartVector
	S            []smartvectors.SmartVector
	AIsProof     []bool
	MustFailFlag bool
}

// ListOfFixedPermutationTestcasePositive lists standard fixed permutation
// testcases that are supposed to pass.
var ListOfFixedPermutationTestcasePositive = []*FixedPermutationTestcase{
	{
		NameStr: "positive/single-column",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(14, 13, 12, 11),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(3, 2, 1, 0),
		},
	},
	{
		NameStr: "positive/multi-column-a",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18),
			smartvectors.ForTest(21, 22, 23, 24, 25, 26, 27, 28),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
		},
	},
	{
		NameStr: "positive/multi-column-b",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18),
			smartvectors.ForTest(21, 22, 23, 24, 25, 26, 27, 28),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7),
			smartvectors.ForTest(8, 9, 10, 11, 12, 13, 14, 15),
		},
	},

	{
		NameStr: "positive/single-column-a-is-proof",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(14, 13, 12, 11),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(3, 2, 1, 0),
		},
	},
	{
		NameStr: "positive/multi-column-a-is-proof",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18),
			smartvectors.ForTest(21, 22, 23, 24, 25, 26, 27, 28),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
		},
	},
	{
		NameStr: "positive/multi-column-b-is-proof",
		A: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18, 21, 22, 23, 24, 25, 26, 27, 28),
		},
		B: []smartvectors.SmartVector{
			smartvectors.ForTest(11, 12, 13, 14, 15, 16, 17, 18),
			smartvectors.ForTest(21, 22, 23, 24, 25, 26, 27, 28),
		},
		S: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 1, 2, 3, 4, 5, 6, 7),
			smartvectors.ForTest(8, 9, 10, 11, 12, 13, 14, 15),
		},
	},
}

func (f *FixedPermutationTestcase) Name() string {
	return f.NameStr
}

func (f *FixedPermutationTestcase) MustFail() bool {
	return f.MustFailFlag
}

func (f *FixedPermutationTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		a = make([]ifaces.Column, len(f.A))
		b = make([]ifaces.Column, len(f.B))
	)

	for i := range f.A {
		a[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("FixedPermutation", f.NameStr, "A", i),
			f.A[i].Len(),
		)

		if i < len(f.AIsProof) && f.AIsProof[i] {
			comp.Columns.SetStatus(a[i].GetColID(), column.Proof)
		}
	}

	for i := range f.B {
		b[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("FixedPermutation", f.NameStr, "B", i),
			f.B[i].Len(),
		)
	}

	comp.InsertFixedPermutation(
		0,
		formatName[ifaces.QueryID]("FixedPermutation", f.NameStr, "Query"),
		f.S,
		a,
		b,
	)
}

func (f *FixedPermutationTestcase) Assign(comp *wizard.ProverRuntime) {

	for i := range f.A {
		comp.AssignColumn(
			formatName[ifaces.ColID]("FixedPermutation", f.NameStr, "A", i),
			f.A[i],
		)
	}

	for i := range f.B {
		comp.AssignColumn(
			formatName[ifaces.ColID]("FixedPermutation", f.NameStr, "B", i),
			f.B[i],
		)
	}
}
