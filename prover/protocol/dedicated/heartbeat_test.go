package dedicated

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// HeartBeatColumnTestcase is an implementation of the [testtools.Testcase]
// interface and represents a wizard protocol using [HeartBeatColumn].
type HeartBeatColumnTestcase struct {
	name     string
	Period   int
	Size     int
	Offset   int
	Activity int
	isActive ifaces.Column
	hb       *HeartBeatColumn
}

// ListOfHeartBeatTestcase lists all the relevant testcases for the heart
// beat columns.
var ListOfHeartBeatTestcase = []*HeartBeatColumnTestcase{
	{
		name:     "full-active/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   0,
		Activity: 0,
	},
	{
		name:     "empty/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   0,
		Activity: 0,
	},
	{
		name:     "less-then-period/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   0,
		Activity: 10,
	},
	{
		name:     "middle/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   0,
		Activity: 1 << 7,
	},
	{
		name:     "full-active/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   19,
		Activity: 0,
	},
	{
		name:     "empty/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   19,
		Activity: 0,
	},
	{
		name:     "less-then-period/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   19,
		Activity: 10,
	},
	{
		name:     "middle/no-auto",
		Period:   20,
		Size:     1 << 8,
		Offset:   19,
		Activity: 1 << 7,
	},
}

func (hbtc *HeartBeatColumnTestcase) Define(comp *wizard.CompiledIOP) {

	hbtc.isActive = comp.InsertCommit(
		0,
		ifaces.ColID(hbtc.name)+"/isactive",
		hbtc.Size,
	)

	hbtc.hb = CreateHeartBeat(comp, 0, hbtc.Period, hbtc.Offset, hbtc.isActive)
}

func (hbtc *HeartBeatColumnTestcase) Assign(run *wizard.ProverRuntime) {

	isActive := make([]field.Element, hbtc.Size)
	for i := 0; i < hbtc.Activity; i++ {
		isActive[i].SetOne()
	}

	run.AssignColumn(hbtc.isActive.GetColID(), smartvectors.NewRegular(isActive))
	hbtc.hb.Assign(run)

	_ = hbtc.hb.Natural.GetColAssignment(run)
}

func (hbtc *HeartBeatColumnTestcase) MustFail() bool {
	return false
}

func (hbtc *HeartBeatColumnTestcase) Name() string {
	return hbtc.name
}

func TestHeartBeat(t *testing.T) {

	for _, tc := range ListOfHeartBeatTestcase {
		t.Run(tc.Name(), func(t *testing.T) {
			testtools.RunTestcase(
				t,
				tc,
				[]func(*wizard.CompiledIOP){dummy.Compile},
			)
		})
	}
}
