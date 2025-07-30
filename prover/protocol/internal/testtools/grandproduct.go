package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

// GrandProductTestcase specifies a protocol with a log-derivative
// sum relationship defined in it and the assignment of its columns
// (which may be valid or not).
type GrandProductTestcase struct {
	// NameStr is the the name of the test-case
	NameStr string
	// Numerators stores the list of the numerators for every pair
	Numerators []smartvectors.SmartVector
	// Denominators stores the list of the denominators for every pair
	Denominators []smartvectors.SmartVector
	// When the value is 'nil', the assigner will compute itself the
	// correct value.
	Value *field.Element
	// MustFailFlag indicates that the present test-case is expected to
	// produce an invalid assignment.
	MustFailFlag bool
	// Q is log-derivative query
	Q query.GrandProduct
}

// ListOfGrandProductTestcasePositive is a list of valid grand-product relations
// and assignmeents.
var ListOfGrandProductTestcasePositive = []*GrandProductTestcase{

	{
		NameStr: "positive/zeroes",
		Numerators: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
		},
		Denominators: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 8),
		},
		Value: &field.Element{},
	},

	{
		NameStr: "positive/ones",
		Numerators: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 8),
		},
		Denominators: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 8),
		},
		Value: new(field.Element).SetOne(),
	},

	{
		NameStr: "positive/randoms",
		Numerators: []smartvectors.SmartVector{
			RandomVec(8),
		},
		Denominators: []smartvectors.SmartVector{
			RandomVec(8),
		},
	},

	{
		NameStr: "positive/has-one-zero",
		Numerators: []smartvectors.SmartVector{
			smartvectors.ForTest(1, 0, 2, 3),
		},
		Denominators: []smartvectors.SmartVector{
			RandomVec(4),
		},
		Value: &field.Element{},
	},

	{
		NameStr: "positive/one-size-cancel-the-other",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 2),
		},
		Denominators: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
			RandomFromSeed(8, 1),
		},
		Value: new(field.Element).SetOne(),
	},
}

// ListOfGrandProductTestcaseNegative lists differents grand-product relations
// and assignmeents that are expected to be rejected by the relation.
var ListOfGrandProductTestcaseNegative = []*GrandProductTestcase{

	{
		NameStr: "negative/zeroes-in-denominator",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 2),
		},
		Denominators: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
			smartvectors.NewConstant(field.Zero(), 8),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/zeroes-in-denominator-swapped",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 2),
		},
		Denominators: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
			RandomFromSeed(8, 2),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/zeroes-in-denominator-only-one-pos",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 2),
		},
		Denominators: []smartvectors.SmartVector{
			smartvectors.ForTest(1, 1, 0, 1, 1, 1, 1, 1),
			RandomFromSeed(8, 2),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/random-result",
		Numerators: []smartvectors.SmartVector{
			RandomVec(8),
		},
		Denominators: []smartvectors.SmartVector{
			RandomVec(8),
		},
		Value: func() *field.Element {
			x := field.PseudoRand(rng)
			return &x
		}(),
		MustFailFlag: true,
	},
}

func (t *GrandProductTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		numerators   = make([]ifaces.Column, len(t.Numerators))
		denominators = make([]ifaces.Column, len(t.Denominators))
		queryInputs  = make(map[int]*query.GrandProductInput)
	)

	for i := range numerators {

		numerators[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("GrandProduct", t.NameStr, "Numerator", i),
			t.Numerators[i].Len(),
		)

		denominators[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("GrandProduct", t.NameStr, "Denominator", i),
			t.Denominators[i].Len(),
		)

		size := numerators[i].Size()

		if _, ok := queryInputs[size]; !ok {
			queryInputs[size] = &query.GrandProductInput{}
		}

		queryInput := queryInputs[size]

		queryInput.Numerators = append(queryInput.Numerators, symbolic.NewVariable(numerators[i]))
		queryInput.Denominators = append(queryInput.Denominators, symbolic.NewVariable(denominators[i]))
	}

	t.Q = comp.InsertGrandProduct(
		0,
		formatName[ifaces.QueryID]("GrandProduct", t.NameStr),
		queryInputs,
	)
}

func (t *GrandProductTestcase) Assign(run *wizard.ProverRuntime) {

	for i := range t.Numerators {

		run.AssignColumn(
			formatName[ifaces.ColID]("GrandProduct", t.NameStr, "Numerator", i),
			t.Numerators[i],
		)

		run.AssignColumn(
			formatName[ifaces.ColID]("GrandProduct", t.NameStr, "Denominator", i),
			t.Denominators[i],
		)
	}

	if t.Value == nil {
		correctValue := t.Q.Compute(run)
		t.Value = &correctValue
	}

	run.AssignGrandProduct(t.Q.ID, *t.Value)
}

func (t *GrandProductTestcase) MustFail() bool {
	return t.MustFailFlag
}

func (t *GrandProductTestcase) Name() string {
	return t.NameStr
}
