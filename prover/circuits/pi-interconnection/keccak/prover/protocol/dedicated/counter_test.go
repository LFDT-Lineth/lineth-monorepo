package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// CyclicCounterTestcase is an implementation of the [testtools.Testcase]
// interface and represents a wizard protocol using [CyclicCounter].
type CyclicCounterTestcase struct {
	// name is the name of the testcase
	name string
	// Period is the period to pass to the counter
	Period int
	// IsActive is the value of the isActive column/expression
	IsActive any
	// cc is the generated cyclic counter column. It is set by the
	// [Define] function.
	cc *CyclicCounter
}

// ListOfCyclicCounterTestcase lists the testcases for [CyclicCounter]
var ListOfCyclicCounterTestcase = []*CyclicCounterTestcase{
	{
		name:     "full-active/plain-column",
		Period:   20,
		IsActive: smartvectors.NewConstant(field.One(), 1<<8),
	},
	{
		name:     "full-active/verifier-column",
		Period:   20,
		IsActive: verifiercol.NewConstantCol(field.One(), 1<<8, ""),
	},
	{
		name:     "full-active/no-auto",
		Period:   3,
		IsActive: smartvectors.ForTest(1, 1, 1, 1, 1, 0, 0, 0),
	},
	{
		name:     "full-zero/plain-column",
		Period:   20,
		IsActive: smartvectors.NewConstant(field.Zero(), 1<<8),
	},
}

func (c *CyclicCounterTestcase) Define(comp *wizard.CompiledIOP) {

	var isActive any

	switch act := c.IsActive.(type) {
	case smartvectors.SmartVector:
		isActive = comp.InsertCommit(0, ifaces.ColID(c.name)+"_ACTIVE", act.Len())
	case verifiercol.ConstCol:
		isActive = act
	default:
		panic("unexpected type")
	}

	c.cc = NewCyclicCounter(comp, 0, c.Period, isActive)
}

func (c *CyclicCounterTestcase) Assign(run *wizard.ProverRuntime) {

	if act, ok := c.IsActive.(smartvectors.SmartVector); ok {
		run.AssignColumn(ifaces.ColID(c.name)+"_ACTIVE", act)
	}

	c.cc.Assign(run)
}

func (hbtc *CyclicCounterTestcase) MustFail() bool {
	return false
}

func (hbtc *CyclicCounterTestcase) Name() string {
	return hbtc.name
}

func TestCyclicCounter(t *testing.T) {
	for _, tc := range ListOfCyclicCounterTestcase {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestcase(
				t,
				tc,
				[]func(*wizard.CompiledIOP){dummy.Compile},
			)
		})
	}
}
