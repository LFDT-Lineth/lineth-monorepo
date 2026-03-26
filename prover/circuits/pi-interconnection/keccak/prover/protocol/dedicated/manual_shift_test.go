package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// ManuallyShiftedTestcase represents a test case for [ManuallyShifted].
type ManuallyShiftedTestcase struct {
	name   string
	Root   smartvectors.SmartVector
	Offset int
	m      *ManuallyShifted
}

var ListOfManuallyShiftedTestcase = []*ManuallyShiftedTestcase{
	{
		name:   "shift+1",
		Root:   smartvectors.ForTest(0, 1, 2, 3),
		Offset: 1,
	},
	{
		name:   "shift-1",
		Root:   smartvectors.ForTest(0, 1, 2, 3),
		Offset: -1,
	},
	{
		name:   "shift0",
		Root:   smartvectors.ForTest(0, 1, 2, 3),
		Offset: 0,
	},
}

func (m *ManuallyShiftedTestcase) Define(comp *wizard.CompiledIOP) {
	root := comp.InsertCommit(0, ifaces.ColID(m.name)+"_ROOT", m.Root.Len())
	m.m = ManuallyShift(comp, root, m.Offset, "")
}

func (m *ManuallyShiftedTestcase) Assign(run *wizard.ProverRuntime) {
	run.AssignColumn(ifaces.ColID(m.name)+"_ROOT", m.Root)
	m.m.Assign(run)
}

func (hbtc *ManuallyShiftedTestcase) MustFail() bool {
	return false
}

func (hbtc *ManuallyShiftedTestcase) Name() string {
	return hbtc.name
}

func TestManuallyShifted(t *testing.T) {
	for _, tc := range ListOfManuallyShiftedTestcase {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestcase(
				t,
				tc,
				[]func(*wizard.CompiledIOP){dummy.Compile},
			)
		})
	}
}
