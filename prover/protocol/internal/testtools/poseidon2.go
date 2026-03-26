package testtools

import (
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// Poseidon2Testcase represent a testcase for the Poseidon2 query
type Poseidon2Testcase struct {
	NameStr   string
	OldStates [][8]smartvectors.SmartVector
	Blocks    [][8]smartvectors.SmartVector
	// NewState can optionally be left empty to let the test-case
	// generate the valid assignment corresponding to the old-state
	// and new-state. It can be explicity set to generate invalid
	// testcases.
	NewStates [][8]smartvectors.SmartVector
}

// ListOfPoseidon2Testcase represents a list of Poseidon2 testcases
var ListOfPoseidon2Testcase = []*Poseidon2Testcase{
	{
		NameStr: "positive/random-one-row",
		OldStates: [][8]smartvectors.SmartVector{
			RandomOctupletVec(1),
		},
		Blocks: [][8]smartvectors.SmartVector{
			RandomOctupletVec(1),
		},
	},
	{
		NameStr: "positive/random-full",
		OldStates: [][8]smartvectors.SmartVector{
			RandomOctupletVec(4),
		},
		Blocks: [][8]smartvectors.SmartVector{
			RandomOctupletVec(4),
		},
	},

	{
		NameStr: "positive/random-full-2-vectors",
		OldStates: [][8]smartvectors.SmartVector{
			RandomOctupletVec(8),
			RandomOctupletVec(16),
		},
		Blocks: [][8]smartvectors.SmartVector{
			RandomOctupletVec(8),
			RandomOctupletVec(16),
		},
	},

	{
		NameStr: "positive/random-padded",
		OldStates: [][8]smartvectors.SmartVector{
			RandomOctupletVecPadded(5, 8),
		},
		Blocks: [][8]smartvectors.SmartVector{
			RandomOctupletVecPadded(5, 8),
		},
	},

	{
		NameStr: "positive/random-padded-2-vectors",
		OldStates: [][8]smartvectors.SmartVector{
			RandomOctupletVecPadded(5, 8),
			RandomOctupletVecPadded(2, 16),
		},
		Blocks: [][8]smartvectors.SmartVector{
			RandomOctupletVecPadded(5, 8),
			RandomOctupletVecPadded(2, 16),
		},
	},

	{
		NameStr: "positive/constant-zero",
		OldStates: [][8]smartvectors.SmartVector{
			ZeroOctupletVec(8),
		},
		Blocks: [][8]smartvectors.SmartVector{
			ZeroOctupletVec(8),
		},
	},

	{
		NameStr: "positive/constant-zero-2-vectors",
		OldStates: [][8]smartvectors.SmartVector{
			ZeroOctupletVec(8),
			ZeroOctupletVec(16),
		},
		Blocks: [][8]smartvectors.SmartVector{
			ZeroOctupletVec(8),
			ZeroOctupletVec(16),
		},
	},
}

func (m *Poseidon2Testcase) Define(comp *wizard.CompiledIOP) {

	for i := range m.Blocks {

		var blocks, oldStates, newStates [8]ifaces.Column
		for j := 0; j < 8; j++ {

			size := m.Blocks[i][j].Len()

			blocks[j] = comp.InsertCommit(
				0,
				ifaces.ColIDf("%v_BLOCKS_%v_%v", m.NameStr, i, j),
				size,
				true,
			)

			oldStates[j] = comp.InsertCommit(
				0,
				ifaces.ColIDf("%v_OLD_STATES_%v_%v", m.NameStr, i, j),
				size,
				true,
			)

			newStates[j] = comp.InsertCommit(
				0,
				ifaces.ColIDf("%v_NEW_STATES_%v_%v", m.NameStr, i, j),
				size,
				true,
			)

		}
		comp.InsertPoseidon2(
			0,
			ifaces.QueryIDf("%v_Poseidon2_%v", m.NameStr, i),
			blocks, oldStates, newStates, nil,
		)
	}
}

func (m *Poseidon2Testcase) Assign(run *wizard.ProverRuntime) {

	for i := range m.Blocks {

		for j := 0; j < 8; j++ {

			run.AssignColumn(
				ifaces.ColIDf("%v_BLOCKS_%v_%v", m.NameStr, i, j),
				m.Blocks[i][j],
			)

			run.AssignColumn(
				ifaces.ColIDf("%v_OLD_STATES_%v_%v", m.NameStr, i, j),
				m.OldStates[i][j],
			)
		}
		var newStates [8]smartvectors.SmartVector

		// if a pre-computed NewStates value is available
		if i < len(m.NewStates) {
			newStates = m.NewStates[i]
		}

		// if no pre-computed value exists. This block performs the actual hash computation:
		if newStates[0] == nil {

			size := m.Blocks[i][0].Len()
			var blocksPadding, oldStatesPadding field.Octuplet
			var blocksWindow, oldStatesWindow [][8]field.Element

			var rotatedBlocksWindow, rotatedOldStatesWindow [8][]field.Element

			for j := 0; j < 8; j++ {
				rotatedBlocksWindow[j] = smartvectors.Window(m.Blocks[i][j])
				blocksPadding[j], _ = smartvectors.PaddingVal(m.Blocks[i][j])

				rotatedOldStatesWindow[j] = smartvectors.Window(m.OldStates[i][j])
				oldStatesPadding[j], _ = smartvectors.PaddingVal(m.OldStates[i][j])
			}

			blocksWindow = make([][8]field.Element, len(rotatedBlocksWindow[0]))
			oldStatesWindow = make([][8]field.Element, len(rotatedOldStatesWindow[0]))

			for s := range blocksWindow {
				for t := range blocksWindow[s] {
					blocksWindow[s][t] = rotatedBlocksWindow[t][s]
				}
			}

			for s := range oldStatesWindow {
				for t := range oldStatesWindow[s] {
					oldStatesWindow[s][t] = rotatedOldStatesWindow[t][s]
				}
			}

			newStatesWindow := make([][8]field.Element, len(blocksWindow))
			var rotatedNewStatesWindow [8][]field.Element

			newStatesPadding := vortex.CompressPoseidon2(oldStatesPadding, blocksPadding)

			for k := range blocksWindow {
				newStatesWindow[k] = vortex.CompressPoseidon2(
					oldStatesWindow[k],
					blocksWindow[k],
				)
			}

			for s := range rotatedNewStatesWindow {
				rotatedNewStatesWindow[s] = make([]field.Element, len(newStatesWindow))
				for t := range rotatedNewStatesWindow[s] {
					rotatedNewStatesWindow[s][t] = newStatesWindow[t][s]
				}
			}

			for j := 0; j < 8; j++ {
				newStates[j] = smartvectors.RightPadded(rotatedNewStatesWindow[j], newStatesPadding[j], size)
			}
		}

		for j := 0; j < 8; j++ {
			run.AssignColumn(
				ifaces.ColIDf("%v_NEW_STATES_%v_%v", m.NameStr, i, j),
				newStates[j],
			)
		}
	}
}

func (m *Poseidon2Testcase) MustFail() bool {
	return false
}

func (m *Poseidon2Testcase) Name() string {
	return m.NameStr
}
