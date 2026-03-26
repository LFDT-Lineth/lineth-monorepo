package statesummary

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	smCommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/common"
)

// initEmptyStorageRoot initialises emptyStorageRoot variable from emptyStorageRootString.
//
// Returns a representation of storage root value in limbs with size defined
// by common.LimbBytes.
func initEmptyStorageRoot() (res [common.NbElemPerHash]field.Element) {
	var emptyStorageRootBig big.Int
	_, isErr := emptyStorageRootBig.SetString(emptyStorageRootString, 10)
	if !isErr {
		panic("empty storage root string is not correct")
	}

	emptyStorageRootByteLimbs := common.SplitBytes(emptyStorageRootBig.Bytes(), field.Bytes)
	if len(emptyStorageRootByteLimbs) != common.NbElemPerHash {
		panic("empty storage root byte limbs length is not correct")
	}
	for i, limbByte := range emptyStorageRootByteLimbs {
		res[i] = *new(field.Element).SetBytes(limbByte)
	}

	return res
}

var (
	// emptyStorageRootString is the TopRoot of an empty accumulator (Poseidon2 hash).
	// This corresponds to the hex value 0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7
	// from TestEmptyAccumulatorPoseidon2 test.
	emptyStorageRootString = "21541760304644240659228284866961727357508514476423129995327043182333673184759"
	// emptyStorageRoot is the root of the empty tree represented in limbs with size common.LimbBytes each.
	emptyStorageRoot = initEmptyStorageRoot()
)

// AccumulatorStatement represents the statement sent to the accumulator.
type AccumulatorStatement struct {
	// IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete are binary
	// flags indicating the type of traces being deferred to the accumulator
	// module.
	IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete ifaces.Column
	// SameTypeAsBefore is an indicator column indicating whether the current
	// row has the same type of shomei operation as the previous one.
	SameTypeAsBefore    ifaces.Column
	CptSameTypeAsBefore wizard.ProverAction

	// StateDiff contains the relevant accumulator values
	smCommon.StateDiff

	// InitialAndFinalHValAreEqual is a constrained limb columns that will contain 1s on the positions where InitialHVal = FinalHVal
	InitialAndFinalHValAreEqual     [poseidon2.BlockSize]ifaces.Column
	ComputeInitialAndFinalHValEqual [poseidon2.BlockSize]wizard.ProverAction

	// FinalHValIsZero is a constrained limb columns that will contain 1s on the positions where FinalHValIsZero = 0
	FinalHValIsZero        [poseidon2.BlockSize]ifaces.Column
	ComputeFinalHValIsZero [poseidon2.BlockSize]wizard.ProverAction
}

// newAccumulatorStatement returns a new AccumulatorStatement with initialized
// columns that are not constrained.
func newAccumulatorStatement(comp *wizard.CompiledIOP, size int, name string) AccumulatorStatement {

	createCol := func(subName string) ifaces.Column {
		return comp.InsertCommit(
			0,
			ifaces.ColIDf("STATE_SUMMARY_%v_%v", name, subName),
			size,
			true,
		)
	}

	res := AccumulatorStatement{
		IsReadZero:    createCol("IS_READ_ZERO"),
		IsReadNonZero: createCol("IS_READ_NON_ZERO"),
		IsInsert:      createCol("IS_INSERT"),
		IsUpdate:      createCol("IS_UPDATE"),
		IsDelete:      createCol("IS_DELETE"),
		StateDiff:     smCommon.NewStateDiff(comp, size, "STATE_SUMMARY", "ACC_STATEMENT"),
	}

	for i := range poseidon2.BlockSize {
		res.InitialAndFinalHValAreEqual[i], res.ComputeInitialAndFinalHValEqual[i] = dedicated.IsZero(
			comp,
			sym.Sub(res.StateDiff.InitialHVal[i], res.StateDiff.FinalHVal[i]),
		).GetColumnAndProverAction()

		res.FinalHValIsZero[i], res.ComputeFinalHValIsZero[i] = dedicated.IsZero(
			comp,
			sym.Sub(res.StateDiff.FinalHVal[i], field.Zero()),
		).GetColumnAndProverAction()
	}

	res.SameTypeAsBefore, res.CptSameTypeAsBefore = dedicated.IsZero(
		comp,
		sym.Sub(
			sym.Add(
				res.IsInsert,
				res.IsUpdate,
				res.IsDelete,
			),
			sym.Add(
				column.Shift(res.IsInsert, -1),
				column.Shift(res.IsUpdate, -1),
				column.Shift(res.IsDelete, -1),
			),
		),
	).GetColumnAndProverAction()

	return res
}

// AccumulatorStatementAssignmentBuilder is a convenience structure storing
// the column builders relating to an AccumulatorStatement.
type AccumulatorStatementAssignmentBuilder struct {
	IsReadZero, IsReadNonZero, IsInsert, IsUpdate, IsDelete *common.VectorBuilder
	SummaryBuilder                                          smCommon.StateDiffAssignmentBuilder
}

// newAccumulatorStatementAssignmentBuilder initializes a fresh
// [AccumulatorStatementAssignmentBuilder]
func newAccumulatorStatementAssignmentBuilder(as *AccumulatorStatement) AccumulatorStatementAssignmentBuilder {
	return AccumulatorStatementAssignmentBuilder{
		IsReadZero:     common.NewVectorBuilder(as.IsReadZero),
		IsReadNonZero:  common.NewVectorBuilder(as.IsReadNonZero),
		IsInsert:       common.NewVectorBuilder(as.IsInsert),
		IsUpdate:       common.NewVectorBuilder(as.IsUpdate),
		IsDelete:       common.NewVectorBuilder(as.IsDelete),
		SummaryBuilder: smCommon.NewStateDiffAssignmentBuilder(as.StateDiff),
	}
}

// PushReadZero pushes the relevant row when a ReadZero occurs on the
// accumulator side.
func (as *AccumulatorStatementAssignmentBuilder) PushReadZero(root, hkey types.KoalaOctuplet) {
	as.IsReadZero.PushOne()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushReadZero(root, hkey)
}

// PushReadNonZero pushes a row onto `as` for a read-non-zero operation.
func (as *AccumulatorStatementAssignmentBuilder) PushReadNonZero(root, hKey, hVal types.KoalaOctuplet) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushOne()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushReadNonZero(root, hKey, hVal)
}

// PushInsert pushes a row representing an insertion onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushInsert(oldRoot, newRoot, hKey, newHVal types.KoalaOctuplet) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushOne()
	as.IsUpdate.PushZero()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushInsert(oldRoot, newRoot, hKey, newHVal)
}

// PushUpdate pushes a row representing an update onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushUpdate(oldRoot, newRoot, hKey, oldHVal, newHVal types.KoalaOctuplet) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushOne()
	as.IsDelete.PushZero()
	as.SummaryBuilder.PushUpdate(oldRoot, newRoot, hKey, oldHVal, newHVal)
}

// PushDelete pushes a row representing a deletion onto `as`.
func (as *AccumulatorStatementAssignmentBuilder) PushDelete(oldRoot, newRoot, hKey, oldHVal types.KoalaOctuplet) {
	as.IsReadZero.PushZero()
	as.IsReadNonZero.PushZero()
	as.IsInsert.PushZero()
	as.IsUpdate.PushZero()
	as.IsDelete.PushOne()
	as.SummaryBuilder.PushDelete(oldRoot, newRoot, hKey, oldHVal)
}

// PadAndAssign pads all the column in `as` and assign them into `run`
func (as *AccumulatorStatementAssignmentBuilder) PadAndAssign(run *wizard.ProverRuntime) {
	as.IsReadZero.PadAndAssign(run)
	as.IsReadNonZero.PadAndAssign(run)
	as.IsInsert.PadAndAssign(run)
	as.IsUpdate.PadAndAssign(run)
	as.IsDelete.PadAndAssign(run)
	as.SummaryBuilder.PadAndAssign(run)
}
