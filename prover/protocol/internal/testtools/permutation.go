package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// PermutationTestcase represents a permutation relationship and and its assignment.
// The instances are used to generate testcases.
type PermutationTestcase struct {
	NameStr      string
	A            [][]smartvectors.SmartVector
	B            [][]smartvectors.SmartVector
	AIsProof     []bool
	BIsProof     []bool
	MustFailFlag bool
	Q            query.Permutation
}

// ListOfPermutationTestcasePositive lists standard permutation testcases
// that are supposed to pass.
var ListOfPermutationTestcasePositive = []*PermutationTestcase{

	{
		NameStr: "positive/1234",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
			},
		},
	},

	{
		NameStr: "positive/1234-multi-column",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
				smartvectors.ForTest(5, 6, 7, 8),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
				smartvectors.ForTest(8, 6, 7, 5),
			},
		},
	},

	{
		NameStr: "positive/1234-split",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4, 9, 10, 11, 12),
			},
			{
				smartvectors.ForTest(5, 6, 7, 8, 13, 14, 15, 16),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1, 8, 6, 7, 5, 9, 10, 11, 12, 13, 14, 15, 16),
			},
		},
	},

	{
		NameStr: "positive/1234-with-proofs",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
			},
		},
		AIsProof: []bool{true},
	},

	{
		NameStr: "positive/1234-multi-column-with-proofs",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
				smartvectors.ForTest(5, 6, 7, 8),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
				smartvectors.ForTest(8, 6, 7, 5),
			},
		},
		AIsProof: []bool{true},
	},

	{
		NameStr: "positive/1234-split-with-proofs",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4, 9, 10, 11, 12),
			},
			{
				smartvectors.ForTest(5, 6, 7, 8, 13, 14, 15, 16),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1, 8, 6, 7, 5, 9, 10, 11, 12, 13, 14, 15, 16),
			},
		},
		AIsProof: []bool{true, true},
	},

	{
		NameStr: "positive/1234-with-proofs-b",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
			},
		},
		BIsProof: []bool{true},
	},

	{
		NameStr: "positive/1234-multi-column-with-proofs-b",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
				smartvectors.ForTest(5, 6, 7, 8),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
				smartvectors.ForTest(8, 6, 7, 5),
			},
		},
		BIsProof: []bool{true},
	},

	{
		NameStr: "positive/1234-split-with-proofs-b",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4, 9, 10, 11, 12),
			},
			{
				smartvectors.ForTest(5, 6, 7, 8, 13, 14, 15, 16),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1, 8, 6, 7, 5, 9, 10, 11, 12, 13, 14, 15, 16),
			},
		},
		BIsProof: []bool{true},
	},
}

// ListOfPermutationTestcasePositive lists standard permutation testcases
// that are supposed to pass.
var ListOfPermutationTestcaseNegative = []*PermutationTestcase{

	{
		NameStr: "negative/1234/missing-1",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(2, 2, 3, 4),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
			},
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/1234-multi-column/missing-first-row",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(2, 2, 3, 4),
				smartvectors.ForTest(6, 6, 7, 8),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(4, 2, 3, 1),
				smartvectors.ForTest(8, 6, 7, 5),
			},
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/1234-split/missing-first-pos",
		A: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(1, 2, 3, 4),
			},
			{
				smartvectors.ForTest(5, 6, 7, 8),
			},
		},
		B: [][]smartvectors.SmartVector{
			{
				smartvectors.ForTest(2, 2, 3, 1, 8, 6, 7, 5),
			},
		},
		MustFailFlag: true,
	},
}

func (p *PermutationTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		a = make([][]ifaces.Column, len(p.A))
		b = make([][]ifaces.Column, len(p.B))
	)

	for i := range p.A {
		a[i] = make([]ifaces.Column, len(p.A[i]))
		for j := range p.A[i] {
			a[i][j] = comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Permutation", p.NameStr, "A", i, j),
				p.A[i][j].Len(),
				smartvectors.IsBase(p.A[i][j]),
			)

			if i < len(p.AIsProof) && p.AIsProof[i] {
				comp.Columns.SetStatus(a[i][j].GetColID(), column.Proof)
			}
		}
	}

	for i := range p.B {
		b[i] = make([]ifaces.Column, len(p.B[i]))
		for j := range p.B[i] {
			b[i][j] = comp.InsertCommit(
				0,
				formatName[ifaces.ColID]("Permutation", p.NameStr, "B", i, j),
				p.B[i][j].Len(),
				smartvectors.IsBase(p.B[i][j]),
			)

			if i < len(p.BIsProof) && p.BIsProof[i] {
				comp.Columns.SetStatus(b[i][j].GetColID(), column.Proof)
			}
		}

	}

	p.Q = query.NewPermutation(formatName[ifaces.QueryID]("Permutation", p.NameStr), a, b)

	comp.QueriesNoParams.AddToRound(0, p.Q.ID, p.Q)
}

func (p *PermutationTestcase) Assign(run *wizard.ProverRuntime) {

	for i := range p.A {
		for j := range p.A[i] {
			run.AssignColumn(
				formatName[ifaces.ColID]("Permutation", p.NameStr, "A", i, j),
				p.A[i][j],
			)
		}
	}

	for i := range p.B {
		for j := range p.B[i] {
			run.AssignColumn(
				formatName[ifaces.ColID]("Permutation", p.NameStr, "B", i, j),
				p.B[i][j],
			)
		}
	}
}

func (p *PermutationTestcase) MustFail() bool { return p.MustFailFlag }

func (p *PermutationTestcase) Name() string { return p.NameStr }
