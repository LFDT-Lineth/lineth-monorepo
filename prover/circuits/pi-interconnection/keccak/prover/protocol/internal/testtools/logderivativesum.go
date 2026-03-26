package testtools

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
)

// LogDerivativeSumTestcase specifies a protocol with a log-derivative
// sum relationship defined in it and the assignment of its columns
// (which may be valid or not).
type LogDerivativeSumTestcase struct {
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
	Q query.LogDerivativeSum
}

// ListOfLogDerivativeSumTestcasePositive is a list of valid grand-product relations
// and assignmeents.
var ListOfLogDerivativeSumTestcasePositive = []*LogDerivativeSumTestcase{

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
		Value: new(field.Element).SetInt64(8),
	},

	{
		NameStr: "positive/x-divided-by-x",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
		},
		Denominators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
		},
		Value: new(field.Element).SetInt64(8),
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
	},

	{
		NameStr: "positive/one-size-cancel-the-other",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
			smartvectors.LinComb([]int{-1}, []smartvectors.SmartVector{RandomFromSeed(8, 2)}),
		},
		Denominators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 1),
		},
		Value: &field.Element{},
	},

	{
		NameStr: "positive/random-sum-multi-size",
		Numerators: []smartvectors.SmartVector{
			RandomFromSeed(8, 1),
			RandomFromSeed(8, 2),
			RandomFromSeed(8, 3),
			RandomFromSeed(8, 4),
			RandomFromSeed(8, 5),
			RandomFromSeed(8, 6),
			RandomFromSeed(8, 7),
			RandomFromSeed(16, 1),
			RandomFromSeed(16, 2),
			RandomFromSeed(16, 3),
			RandomFromSeed(16, 4),
			RandomFromSeed(16, 5),
			RandomFromSeed(16, 6),
			RandomFromSeed(16, 7),
		},
		Denominators: []smartvectors.SmartVector{
			RandomFromSeed(8, 21),
			RandomFromSeed(8, 22),
			RandomFromSeed(8, 23),
			RandomFromSeed(8, 24),
			RandomFromSeed(8, 25),
			RandomFromSeed(8, 26),
			RandomFromSeed(8, 27),
			RandomFromSeed(16, 21),
			RandomFromSeed(16, 22),
			RandomFromSeed(16, 23),
			RandomFromSeed(16, 24),
			RandomFromSeed(16, 25),
			RandomFromSeed(16, 26),
			RandomFromSeed(16, 27),
		},
	},
}

var ListOfLogDerivativeSumTestcaseNegative = []*LogDerivativeSumTestcase{

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

func (t *LogDerivativeSumTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		numerators   = make([]ifaces.Column, len(t.Numerators))
		denominators = make([]ifaces.Column, len(t.Denominators))
		queryInputs  = make([]query.LogDerivativeSumPart, 0)
	)

	for i := range numerators {

		numerators[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("LogDerivative", t.NameStr, "Numerator", i),
			t.Numerators[i].Len(),
		)

		denominators[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("LogDerivative", t.NameStr, "Denominator", i),
			t.Denominators[i].Len(),
		)

		size := numerators[i].Size()

		queryInputs = append(queryInputs, query.LogDerivativeSumPart{
			Size: size,
			Name: formatName[string]("LogDerivative", t.NameStr, "Part", i),
			Num:  symbolic.NewVariable(numerators[i]),
			Den:  symbolic.NewVariable(denominators[i]),
		})
	}

	t.Q = comp.InsertLogDerivativeSum(
		0,
		formatName[ifaces.QueryID]("LogDerivative", t.NameStr),
		query.LogDerivativeSumInput{
			Parts: queryInputs,
		},
	)
}

func (t *LogDerivativeSumTestcase) Assign(run *wizard.ProverRuntime) {

	for i := range t.Numerators {

		run.AssignColumn(
			formatName[ifaces.ColID]("LogDerivative", t.NameStr, "Numerator", i),
			t.Numerators[i],
		)

		run.AssignColumn(
			formatName[ifaces.ColID]("LogDerivative", t.NameStr, "Denominator", i),
			t.Denominators[i],
		)
	}

	if t.Value == nil {
		correctValue, err := t.Q.Compute(run)
		if err != nil {
			panic(err)
		}
		t.Value = &correctValue
	}

	run.AssignLogDerivSum(t.Q.ID, *t.Value)
}

func (t *LogDerivativeSumTestcase) MustFail() bool {
	return t.MustFailFlag
}

func (t *LogDerivativeSumTestcase) Name() string {
	return t.NameStr
}
