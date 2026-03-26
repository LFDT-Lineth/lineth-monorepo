package testtools

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// MiMCTestcase represent a testcase for the MiMC query
type MiMCTestcase struct {
	NameStr   string
	OldStates []smartvectors.SmartVector
	Blocks    []smartvectors.SmartVector
	// NewState can optionally be left empty to let the test-case
	// generate the valid assignment corresponding to the old-state
	// and new-state. It can be explicity set to generate invalid
	// testcases.
	NewStates []smartvectors.SmartVector
}

// ListOfMiMCTestcase represents a list of MiMC testcases
var ListOfMiMCTestcase = []*MiMCTestcase{

	{
		NameStr: "positive/random-full",
		OldStates: []smartvectors.SmartVector{
			RandomVec(8),
		},
		Blocks: []smartvectors.SmartVector{
			RandomVec(8),
		},
	},

	{
		NameStr: "positive/random-full-2-vectors",
		OldStates: []smartvectors.SmartVector{
			RandomVec(8),
			RandomVec(16),
		},
		Blocks: []smartvectors.SmartVector{
			RandomVec(8),
			RandomVec(16),
		},
	},

	{
		NameStr: "positive/random-padded",
		OldStates: []smartvectors.SmartVector{
			RandomVecPadded(5, 8),
		},
		Blocks: []smartvectors.SmartVector{
			RandomVecPadded(5, 8),
		},
	},

	{
		NameStr: "positive/random-padded-2-vectors",
		OldStates: []smartvectors.SmartVector{
			RandomVecPadded(5, 8),
			RandomVecPadded(2, 16),
		},
		Blocks: []smartvectors.SmartVector{
			RandomVecPadded(5, 8),
			RandomVecPadded(2, 16),
		},
	},

	{
		NameStr: "positive/constant-zero",
		OldStates: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
		},
		Blocks: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
		},
	},

	{
		NameStr: "positive/constant-zero-2-vectors",
		OldStates: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
			smartvectors.NewConstant(field.Zero(), 16),
		},
		Blocks: []smartvectors.SmartVector{
			smartvectors.NewConstant(field.Zero(), 8),
			smartvectors.NewConstant(field.Zero(), 16),
		},
	},
}

func (m *MiMCTestcase) Define(comp *wizard.CompiledIOP) {

	for i := range m.Blocks {

		size := m.Blocks[i].Len()

		blocks := comp.InsertCommit(
			0,
			ifaces.ColIDf("%v_BLOCKS_%v", m.NameStr, i),
			size,
		)

		oldStates := comp.InsertCommit(
			0,
			ifaces.ColIDf("%v_OLD_STATES_%v", m.NameStr, i),
			size,
		)

		newStates := comp.InsertCommit(
			0,
			ifaces.ColIDf("%v_NEW_STATES_%v", m.NameStr, i),
			size,
		)

		comp.InsertMiMC(
			0,
			ifaces.QueryIDf("%v_MIMC_%v", m.NameStr, i),
			blocks, oldStates, newStates, nil,
		)
	}
}

func (m *MiMCTestcase) Assign(run *wizard.ProverRuntime) {

	for i := range m.Blocks {

		run.AssignColumn(
			ifaces.ColIDf("%v_BLOCKS_%v", m.NameStr, i),
			m.Blocks[i],
		)

		run.AssignColumn(
			ifaces.ColIDf("%v_OLD_STATES_%v", m.NameStr, i),
			m.OldStates[i],
		)

		var newStates smartvectors.SmartVector

		if i < len(m.NewStates) {
			newStates = m.NewStates[i]
		}

		if newStates == nil {

			var (
				size                = m.Blocks[i].Len()
				blocksWindow        = smartvectors.Window(m.Blocks[i])
				blocksPadding, _    = smartvectors.PaddingVal(m.Blocks[i])
				oldStatesWindow     = smartvectors.Window(m.OldStates[i])
				oldStatesPadding, _ = smartvectors.PaddingVal(m.OldStates[i])
				newStatesWindow     = make([]field.Element, len(blocksWindow))
				newStatesPadding    = mimc.BlockCompression(oldStatesPadding, blocksPadding)
			)

			for k := range blocksWindow {
				newStatesWindow[k] = mimc.BlockCompression(
					oldStatesWindow[k],
					blocksWindow[k],
				)
			}

			newStates = smartvectors.RightPadded(newStatesWindow, newStatesPadding, size)
		}

		run.AssignColumn(
			ifaces.ColIDf("%v_NEW_STATES_%v", m.NameStr, i),
			newStates,
		)
	}
}

func (m *MiMCTestcase) MustFail() bool {
	return false
}

func (m *MiMCTestcase) Name() string {
	return m.NameStr
}
