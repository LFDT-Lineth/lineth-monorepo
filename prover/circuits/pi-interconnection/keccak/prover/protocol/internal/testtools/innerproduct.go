package testtools

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// InnerProductTestcase specifies a protocol with an inner-product
// relationship and the assignment to provide to it.
type InnerProductTestcase struct {
	// Name is the name of the testcase.
	Name string
	// A is the assignment to provide to the A column of the smartvector
	A smartvectors.SmartVector
	// Bs is the assignment to provide the Bs columns of the smartvector
	Bs []smartvectors.SmartVector
	// Values are the values to assign to the query. nil tells the assign
	// function to compute it itself. And thus, having the correct value.
	Values []field.Element
	// Q is the query to define.
	Q query.InnerProduct
	// MustFailFlag is true if the testcase must fail.
	MustFailFlag bool
}

var ListOfInnerProductTestcasePositive = []InnerProductTestcase{

	{
		Name: "positive/query-1",
		A:    smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
		},
		Values: []field.Element{
			field.NewElement(5),
		},
	},

	{
		Name: "positive/query-2",
		A:    smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
			smartvectors.ForTest(1, 0, 0, 2),
		},
		Values: []field.Element{field.NewElement(5), field.NewElement(3)},
	},

	{
		Name: "positive/query-3",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0),
			smartvectors.ForTest(1, 0, 0, 2, 1, 0, 0, 0),
		},
		Values: []field.Element{field.NewElement(7), field.NewElement(5)},
	},

	{
		Name: "positive/query-4",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values: []field.Element{field.NewElement(15)},
	},

	{
		Name: "positive/query-5",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values: []field.Element{field.NewElement(30)},
	},

	{
		Name: "positive/random-full",
		A:    RandomFromSeed(8, 1),
		Bs: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
		},
	},
}

var ListOfInnerProductTestcaseNegative = []*InnerProductTestcase{

	{
		Name: "negative/query-1/bad-result",
		A:    smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
		},
		Values: []field.Element{
			field.NewElement(17),
		},
		MustFailFlag: true,
	},

	{
		Name: "negative/query-2",
		A:    smartvectors.ForTest(1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2),
			smartvectors.ForTest(1, 0, 0, 2),
		},
		Values:       []field.Element{field.NewElement(22), field.NewElement(3)},
		MustFailFlag: true,
	},

	{
		Name: "negative/query-3",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0),
			smartvectors.ForTest(1, 0, 0, 2, 1, 0, 0, 0),
		},
		Values:       []field.Element{field.NewElement(77), field.NewElement(5)},
		MustFailFlag: true,
	},

	{
		Name: "negative/query-4",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values:       []field.Element{field.NewElement(14)},
		MustFailFlag: true,
	},

	{
		Name: "negative/query-5",
		A:    smartvectors.ForTest(1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 0, 2, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		Bs: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 3, 0, 2, 1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1),
		},
		Values:       []field.Element{field.NewElement(31)},
		MustFailFlag: true,
	},

	{
		Name: "negative/random-full",
		A:    RandomFromSeed(8, 1),
		Bs: []smartvectors.SmartVector{
			RandomFromSeed(8, 2),
		},
		Values:       []field.Element{field.NewElement(30)},
		MustFailFlag: true,
	},
}

func (ip *InnerProductTestcase) Define(comp *wizard.CompiledIOP) {

	bs := make([]ifaces.Column, len(ip.Bs))

	a := comp.InsertCommit(
		0,
		formatName[ifaces.ColID]("InnerProduct", ip.Name, "A"),
		ip.A.Len(),
	)

	for i := range ip.Bs {
		bs[i] = comp.InsertCommit(
			0,
			formatName[ifaces.ColID]("InnerProduct", ip.Name, "B", i),
			ip.Bs[i].Len(),
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

	run.AssignColumn(
		formatName[ifaces.ColID]("InnerProduct", ip.Name, "A"),
		ip.A,
	)

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
