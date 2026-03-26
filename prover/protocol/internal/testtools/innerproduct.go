package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// InnerProductTestcase specifies a protocol with an inner-product relationship
// and the assignment to provide to it.
type InnerProductTestcase struct {
	// Name is the name of the testcase.
	NameStr string
	// A is the assignment to provide to the A column of the smartvector
	A smartvectors.SmartVector
	// Bs is the assignment to provide the Bs columns of the smartvector
	Bs []smartvectors.SmartVector
	// Values are the values to assign to the query. nil tells the assign
	// function to compute it itself. And thus, having the correct value.
	Values []fext.Element
	// Q is the query to define.
	Q query.InnerProduct
	// MustFailFlag is true if the testcase must fail.
	MustFailFlag bool
	// AsIsConstCol is an optional flag to force the A column to be defined
	// as a verifiercol.Constant instead of a plain column.
	AsIsConstCol bool
}

var ListOfInnerProductTestcasePositive = []*InnerProductTestcase{

	{
		NameStr: "positive/counting",
		A:       smartvectors.NewConstant(field.One(), 16),
		Bs: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 16),
		},
		Values: []fext.Element{
			fext.Lift(field.NewElement(16)),
		},
	},

	{
		NameStr: "positive/counting-with-constcol",
		A:       smartvectors.NewConstant(field.One(), 16),
		Bs: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.One(), 16),
		},
		Values: []fext.Element{
			fext.Lift(field.NewElement(16)),
		},
		AsIsConstCol: true,
	},

	{
		NameStr: "positive/query-1",
		A:       smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
		},
		Values: []fext.Element{
			fext.Lift(field.NewElement(5)),
		},
	},

	{
		NameStr: "positive/query-2",
		A:       smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
			smartvectors.ForTest(1, 0, 0, 2),
		},
		Values: []fext.Element{fext.Lift(field.NewElement(5)), fext.Lift(field.NewElement(3))},
	},

	{
		NameStr: "positive/query-3",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0),
			smartvectors.ForTest(1, 0, 0, 2, 1, 0, 0, 0),
		},
		Values: []fext.Element{fext.Lift(field.NewElement(7)), fext.Lift(field.NewElement(5))},
	},

	{
		NameStr: "positive/query-4",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values: []fext.Element{fext.Lift(field.NewElement(15))},
	},

	{
		NameStr: "positive/query-5",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values: []fext.Element{fext.Lift(field.NewElement(30))},
	},

	{
		NameStr: "positive/random-full",
		A:       RandomFromSeed(8, 1),
		Bs: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
		},
	},
}

var ListOfInnerProductTestcaseNegative = []*InnerProductTestcase{

	{
		NameStr: "negative/query-1/bad-result",
		A:       smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
		},
		Values: []fext.Element{
			fext.Lift(field.NewElement(17)),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/query-2",
		A:       smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
			smartvectors.ForTest(1, 0, 0, 2),
		},
		Values:       []fext.Element{fext.Lift(field.NewElement(22)), fext.Lift(field.NewElement(3))},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/query-3",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0),
			smartvectors.ForTest(1, 0, 0, 2, 1, 0, 0, 0),
		},
		Values:       []fext.Element{fext.Lift(field.NewElement(77)), fext.Lift(field.NewElement(5))},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/query-4",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values:       []fext.Element{fext.Lift(field.NewElement(14))},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/query-5",
		A:       smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values:       []fext.Element{fext.Lift(field.NewElement(31))},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/random-full",
		A:       RandomFromSeed(8, 1),
		Bs: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
		},
		Values:       []fext.Element{fext.Lift(field.NewElement(30))},
		MustFailFlag: true,
	},
}

func (ip *InnerProductTestcase) Define(comp *wizard.CompiledIOP) {

	bs := make([]ifaces.Column, len(ip.Bs))

	var a ifaces.Column

	if ip.AsIsConstCol {
		a = verifiercol.NewConstantCol(ip.A.Get(0), ip.A.Len(), "")
	} else {
		a = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("InnerProduct", ip.Name, "A"),
			ip.A.Len(),
			smartvectors.IsBase(ip.A),
		)
	}

	for i := range ip.Bs {
		bs[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("InnerProduct", ip.Name, "B", i),
			ip.Bs[i].Len(),
			smartvectors.IsBase(ip.Bs[i]),
		)
	}

	ip.Q = comp.InsertInnerProduct(
		0,
		formatName[ifaces.QueryID]("InnerProduct", ip.Name, "Query"),
		a,
		bs,
	)
}

func (ip *InnerProductTestcase) Assign(run *wizard.ProverRuntime) {

	if !ip.AsIsConstCol {
		run.AssignColumn(
			formatName[ifaces.ColID]("InnerProduct", ip.Name, "A"),
			ip.A,
		)
	}

	for i := range ip.Bs {
		run.AssignColumn(
			formatName[ifaces.ColID]("InnerProduct", ip.Name, "B", i),
			ip.Bs[i],
		)
	}

	value := ip.Values

	if len(value) == 0 {
		value = ip.Q.Compute(run)
	}

	run.AssignInnerProduct(ip.Q.ID, value...)
}

func (ip *InnerProductTestcase) MustFail() bool {
	return ip.MustFailFlag
}

func (ip *InnerProductTestcase) Name() string {
	return ip.NameStr
}
