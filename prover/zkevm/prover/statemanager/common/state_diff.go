package common

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// StateDiff is a collection of column that appears in several of the modules
// of the state-manager of Linea.
//
// In state summary, we have a unique tuple of (hKey, initialHVal, finalHVal,
// initialRoot, finalRoot) for each of the state operations (e.g. INSERT,
// DELETE, UPDATE, READZERO, and READNONZERO). We want to check that this
// unique tuple is the same for the state summary and the accumulator module.
// In the accumulator module, we have six rows for INSERT and DELETE, and two
// rows for UPDATE, READZERO and READNONZERO. The bridge we call the
// accumulatorSummary constructs the tuple (hKey, initialHVal, finalHVal,
// initialRoot, finalRoot) from various rows of the accumulator module.
// (To exemplify further, depending on the type of state operation on the sparse
// Merkle tree, hkey will appear on a different row in the segment corresponding
// to the state operation—and similarly with the other components in the constructed tuple.)
type StateDiff struct {
	// HKey stores the initial and final accumulator's key hashes.
	HKey [common.NbElemPerHash]ifaces.Column

	// InitialHVal and FinalHVal store the initial and final accumulator's
	// hash of values.
	InitialHVal, FinalHVal [common.NbElemPerHash]ifaces.Column

	// InitialRoot and FinalRoot store the accumulator's initial and final
	// root hashes.
	InitialRoot, FinalRoot [common.NbElemPerHash]ifaces.Column
}

// NewStateDiff declares all the columns adding up to a [StateDiff] and returns
// the corresponding object.
func NewStateDiff(comp *wizard.CompiledIOP, size int, moduleName, name string) StateDiff {

	createCol := func(moduleName, subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("%v_%v_%v", moduleName, name, subName),
			size,
			true,
		)
	}

	res := StateDiff{}

	for i := range common.NbElemPerHash {
		res.HKey[i] = createCol(moduleName, fmt.Sprintf("HKEY_%d", i))
		res.InitialHVal[i] = createCol(moduleName, fmt.Sprintf("INITIAL_HVAL_%d", i))
		res.FinalHVal[i] = createCol(moduleName, fmt.Sprintf("FINAL_HVAL_%d", i))
		res.InitialRoot[i] = createCol(moduleName, fmt.Sprintf("INITIAL_ROOT_%d", i))
		res.FinalRoot[i] = createCol(moduleName, fmt.Sprintf("FINAL_ROOT_%d", i))
	}

	return res
}

// StateDiffAssignmentBuilder is a convenience structure storing the column
// builders relating to an AccumulatorSummary.
type StateDiffAssignmentBuilder struct {
	HKey                   [common.NbElemPerHash]*common.VectorBuilder
	InitialHVal, FinalHVal [common.NbElemPerHash]*common.VectorBuilder
	InitialRoot, FinalRoot [common.NbElemPerHash]*common.VectorBuilder
}

// NewStateDiffAssignmentBuilder initializes a fresh
// [StateDiffAssignmentBuilder]
func NewStateDiffAssignmentBuilder(as StateDiff) StateDiffAssignmentBuilder {
	res := StateDiffAssignmentBuilder{}

	for i := range common.NbElemPerHash {
		res.HKey[i] = common.NewVectorBuilder(as.HKey[i])
		res.InitialHVal[i] = common.NewVectorBuilder(as.InitialHVal[i])
		res.FinalHVal[i] = common.NewVectorBuilder(as.FinalHVal[i])
		res.InitialRoot[i] = common.NewVectorBuilder(as.InitialRoot[i])
		res.FinalRoot[i] = common.NewVectorBuilder(as.FinalRoot[i])
	}

	return res
}

// PushReadZero pushes the relevant row when a ReadZero occurs on the
// accumulator side.
func (as *StateDiffAssignmentBuilder) PushReadZero(root, hkey types.KoalaOctuplet) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PushField(hkey[i])
		as.InitialHVal[i].PushZero()
		as.FinalHVal[i].PushZero()
		as.InitialRoot[i].PushField(root[i])
		as.FinalRoot[i].PushField(root[i])
	}
}

// PushReadNonZero pushes a row onto `as` for a read-non-zero operation.
func (as *StateDiffAssignmentBuilder) PushReadNonZero(root, hKey, hVal types.KoalaOctuplet) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PushField(hKey[i])
		as.InitialHVal[i].PushField(hVal[i])
		as.FinalHVal[i].PushField(hVal[i])
		as.InitialRoot[i].PushField(root[i])
		as.FinalRoot[i].PushField(root[i])
	}
}

// PushInsert pushes a row representing an insertion onto `as`.
func (as *StateDiffAssignmentBuilder) PushInsert(oldRoot, newRoot, hKey, newHVal types.KoalaOctuplet) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PushField(hKey[i])
		as.InitialHVal[i].PushZero()
		as.FinalHVal[i].PushField(newHVal[i])
		as.InitialRoot[i].PushField(oldRoot[i])
		as.FinalRoot[i].PushField(newRoot[i])
	}
}

// PushUpdate pushes a row representing an update onto `as`.
func (as *StateDiffAssignmentBuilder) PushUpdate(oldRoot, newRoot, hKey, oldHVal, newHVal types.KoalaOctuplet) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PushField(hKey[i])
		as.InitialHVal[i].PushField(oldHVal[i])
		as.FinalHVal[i].PushField(newHVal[i])
		as.InitialRoot[i].PushField(oldRoot[i])
		as.FinalRoot[i].PushField(newRoot[i])
	}
}

// PushDelete pushes a row representing a deletion onto `as`.
func (as *StateDiffAssignmentBuilder) PushDelete(oldRoot, newRoot, hKey, oldHVal types.KoalaOctuplet) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PushField(hKey[i])
		as.InitialHVal[i].PushField(oldHVal[i])
		as.FinalHVal[i].PushZero()
		as.InitialRoot[i].PushField(oldRoot[i])
		as.FinalRoot[i].PushField(newRoot[i])
	}
}

// PadAndAssign pads all the column in `as` and assign them into `run`
func (as *StateDiffAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	for i := range common.NbElemPerHash {
		as.HKey[i].PadAndAssign(run)
		as.InitialHVal[i].PadAndAssign(run)
		as.FinalHVal[i].PadAndAssign(run)
		as.InitialRoot[i].PadAndAssign(run)
		as.FinalRoot[i].PadAndAssign(run)
	}
}

// addRows add rows to the builder that is used to construct an AccumulatorSummary
func (builder *StateDiffAssignmentBuilder) AddRows(numRowsAccSegment int, hKey, initialHVal, finalHVal, initialRoot, finalRoot [common.NbElemPerHash]field.Element) {
	for i := 1; i <= numRowsAccSegment; i++ {
		for j := range common.NbElemPerHash {
			builder.HKey[j].PushField(hKey[j])
			builder.InitialHVal[j].PushField(initialHVal[j])
			builder.FinalHVal[j].PushField(finalHVal[j])
			builder.InitialRoot[j].PushField(initialRoot[j])
			builder.FinalRoot[j].PushField(finalRoot[j])
		}
	}
}
