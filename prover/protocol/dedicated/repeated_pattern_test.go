package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// RepeatedPatternTestcase represents a test case for [RepeatedPattern].
type RepeatedPatternTestcase struct {
	name     string
	IsActive smartvectors.SmartVector
	Pattern  []field.Element
	rp       *RepeatedPattern
}

// ListOfRepeatedPatternTestcase is a list of [RepeatedPatternTestcase].
var ListOfRepeatedPatternTestcase = []*RepeatedPatternTestcase{
	{
		name:     "size-1-pattern",
		IsActive: smartvectors.RightZeroPadded([]field.Element{field.NewElement(1)}, 8),
		Pattern:  []field.Element{field.NewElement(1)},
	},
	{
		name:     "size-3-pattern-fully-active",
		IsActive: smartvectors.ForTest(1, 1, 1, 1, 1, 1, 1, 1),
		Pattern:  vector.ForTest(1, 2, 3),
	},
	{
		name:     "size-3-pattern-partly-active",
		IsActive: smartvectors.ForTest(1, 1, 1, 1, 1, 0, 0, 0),
		Pattern:  vector.ForTest(1, 2, 3),
	},
	{
		name:     "size-3-pattern-little-active",
		IsActive: smartvectors.ForTest(1, 1, 1, 0, 0, 0, 0, 0),
		Pattern:  vector.ForTest(1, 2, 3),
	},
	{
		name:     "size-3-pattern-no-activity",
		IsActive: smartvectors.ForTest(0, 0, 0, 0, 0, 0, 0, 0),
		Pattern:  vector.ForTest(1, 2, 3),
	},
}

func (rp *RepeatedPatternTestcase) Define(comp *wizard.CompiledIOP) {
	isActive := comp.InsertCommit(0, ifaces.ColID(rp.name)+"_ACTIVE", rp.IsActive.Len())
	rp.rp = NewRepeatedPattern(comp, 0, rp.Pattern, isActive)
}

func (rp *RepeatedPatternTestcase) Assign(run *wizard.ProverRuntime) {
	run.AssignColumn(ifaces.ColID(rp.name)+"_ACTIVE", rp.IsActive)
	rp.rp.Assign(run)
}

func (hbtc *RepeatedPatternTestcase) MustFail() bool {
	return false
}

func (hbtc *RepeatedPatternTestcase) Name() string {
	return hbtc.name
}

func TestRepeatedPattern(t *testing.T) {
	for _, tc := range ListOfRepeatedPatternTestcase {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestcase(
				t,
				tc,
				[]func(*wizard.CompiledIOP){dummy.Compile},
			)
		})
	}
}
