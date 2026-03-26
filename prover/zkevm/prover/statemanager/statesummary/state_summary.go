package statesummary

import (
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// initEmptyKeccak initialises emptyKeccak variable from emptyKeccakString.
//
// Returns a representation of empty keccak value in limbs with size defined
// by common.LimbBytes.
func initEmptyKeccak(emptyKeccakString string) (res [common.NbLimbU128]field.Element) {
	var emptyKeccakBig big.Int
	_, isErr := emptyKeccakBig.SetString(emptyKeccakString, 16)
	if !isErr {
		panic("empty keccak string is not correct")
	}
	emptyKeccakByteLimbs := common.SplitBytes(emptyKeccakBig.Bytes())
	if len(emptyKeccakByteLimbs) != common.NbLimbU128 {
		panic("empty keccak byte limbs length is not correct")
	}
	for i, limbByte := range emptyKeccakByteLimbs {
		res[i] = *new(field.Element).SetBytes(limbByte)
	}

	return res
}

const (
	EMPTYKECCAKCODEHASH_HI_STR = "c5d2460186f7233c927e7db2dcc703c0"
	EMPTYKECCAKCODEHASH_LO_STR = "e500b653ca82273b7bfad8045d85a470"
)

var (
	EMPTYKECCAKCODEHASH_HI = initEmptyKeccak(EMPTYKECCAKCODEHASH_HI_STR)
	EMPTYKECCAKCODEHASH_LO = initEmptyKeccak(EMPTYKECCAKCODEHASH_LO_STR)
)

// Module represents the state-summary module. It defines all the columns
// constraints and assigment methods for this module. The state-summary module
// is tasked with:
//
//   - “commanding” the accumulator module and providing a sequential
//     view of all the shomei operations.
//   - Hashing the accounts, account-addresses, storage-keys and storage-slots
//   - Ensuring the sequentiality of the root hashes
//   - There is only a single trace-segment for each block and account.
//   - The segments consists in a succession of
//
// The state-summary module **is not** tasked with:
//
//   - Ensuring the correctness of the state-access patterns (the accumulator's
//     properties are that any defect of the traces is either irrelevant or
//     detected). This is only true for what goes beside the above-required
//     points.
//
// The main heart-beat of the state-summary module is 1 row == 1 accumulator
// trace issuance.
type Module struct {

	// IsActive is a module-wide flag indicating whether the current row
	// corresponds to a state operation or a padding row in which nothing is
	// happening. Most of the constraints are cancelled for the padding rows.
	IsActive ifaces.Column

	// IsEndOfAccountSegment is a binary columns indicating the end of an
	// account segment.
	IsEndOfAccountSegment, IsBeginningOfAccountSegment ifaces.Column

	// IsInitialDeployment and IsFinalDeployment are columns assigned to boolean
	// values whose row indicates whether the current segment relates to the
	// initial or the final deployment of an account. This is needed essentially
	// to support contract redeployment. For "normal" account segments, this is
	// always assigned to 1.
	IsInitialDeployment, IsFinalDeployment ifaces.Column

	// IsStorage indicates that the current row is peaking at a storage slot
	// of the account. The negative case means we are looking at the account
	// in the world-state trie and are terminating a segment.
	IsStorage ifaces.Column

	// IsDeleteSegment is an ad-hoc column indicating whether the current
	// account sub-segment is an account deletion. It is constant on the whole
	// sub-segment and is used to un-bind the storage values from the accumulator
	// as Shomei disregards them.
	IsDeleteSegment ifaces.Column

	// BatchNumber represents the index of a block as part of the conflation.
	// It contrasts with the block-number in the sense that it always starts from
	// 0 for the first block of the conflation and then increases.
	BatchNumber [common.NbLimbU64]ifaces.Column

	// WorldStateRoot stores the state-root hashes.
	WorldStateRoot [common.NbElemPerHash]ifaces.Column

	// Account.Initial and Account.Final represents the values stored in the
	// account at the beginning and the end of the trace segment.
	Account AccountPeek

	// Storage and FinalStorage represent the initial and final value
	// of a storage slot currently being peeked at.
	Storage StoragePeek

	// AccumulatorStatement contains the statement values associated with the
	// accumulator.
	AccumulatorStatement AccumulatorStatement

	// ArithmetizationLink is an optional parameter (non-optional in production)
	// storing the collection of columns from the Hub module that are used by
	// the constraints declared by [StateSummary.WithHubConnection] method. It
	// also stores a few endemic columns
	ArithmetizationLink *arithmetizationLink
}

func NewModule(comp *wizard.CompiledIOP, size int) Module {

	if !utils.IsPowerOfTwo(size) {
		utils.Panic("size must be power of two, got %v", size)
	}

	// createCol is function to quickly create a column
	createCol := func(name string) ifaces.Column {
		return comp.InsertCommit(0, ifaces.ColIDf("STATE_SUMMARY_%v", name), size, true)
	}

	res := Module{
		IsActive:                    createCol("IS_ACTIVE"),
		IsEndOfAccountSegment:       createCol("IS_END_OF_ACCOUNT_SEGMENT"),
		IsBeginningOfAccountSegment: createCol("IS_BEGINNING_OF_ACCOUNT_SEGMENT"),
		IsInitialDeployment:         createCol("IS_INITIAL_DEPLOYMENT"),
		IsFinalDeployment:           createCol("IS_FINAL_DEPLOYMENT"),
		IsDeleteSegment:             createCol("IS_DELETE_SEGMENT"),
		IsStorage:                   createCol("IS_STORAGE"),
		Account:                     newAccountPeek(comp, size),
		Storage:                     newStoragePeek(comp, size, "STORAGE_PEEK"),
		AccumulatorStatement:        newAccumulatorStatement(comp, size, "ACCUMULATOR_STATEMENT"),
	}

	for i := range common.NbLimbU64 {
		res.BatchNumber[i] = createCol(fmt.Sprintf("BATCH_NUMBER_%v", i))
	}

	for i := range common.NbElemPerHash {
		res.WorldStateRoot[i] = createCol(fmt.Sprintf("WORLD_STATE_ROOT_%v", i))
	}

	res.csAccountAddress(comp)
	res.csAccountOld(comp)
	res.csAccountNew(comp)
	res.csAccumulatorRoots(comp)
	res.csAccumulatorStatementFlags(comp)
	res.csAccumulatorStatementHValKey(comp)
	res.csBatchNumber(comp)
	res.csInitialFinalDeployment(comp)
	res.csIsActive(comp)
	res.csIsBeginningOfAccountSegment(comp)
	res.csIsEndOfAccountSegment(comp)
	res.csIsStorage(comp)
	res.csStoragePeek(comp)
	res.csWorldStateRoot(comp)
	res.csIsDeletionSegment(comp)
	res.constrainExpectedHubCodeHash(comp)
	return res

}

// csAccountAddress adds all the constraints related to the account address.
func (ss *Module) csAccountAddress(comp *wizard.CompiledIOP) {

	for i := range common.NbLimbEthAddress {
		isZeroWhenInactive(comp, ss.Account.Address[i], ss.IsActive)
	}

	// Constraint for each limb: when batch number limb[i] didn't change by exactly 1,
	// addresses can only increase or stay same
	// Big-endian format: last limb (i=3) holds the value
	// For the other limbs, batch number should match with its shifted version
	for i := range common.NbLimbU64 - 1 {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_BATCH_NUMBER_HIGHER_LIMBS_ARE_ZERO_%d", i),
			sym.Mul(
				ss.IsActive,
				sym.Sub(ss.BatchNumber[i], column.Shift(ss.BatchNumber[i], -1))),
		)

	}
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_ADDRESS_CAN_ONLY_INCREASE_IN_BLOCK_RANGE"),
		sym.Mul(
			ss.IsActive,
			sym.Sub(
				ss.BatchNumber[common.NbLimbU64-1],
				column.Shift(ss.BatchNumber[common.NbLimbU64-1], -1),
				1,
			),
			sym.Add(
				ss.Account.HasGreaterAddressAsPrev,
				ss.Account.HasSameAddressAsPrev,
				-1,
			),
		),
	)
}

// csIsActive constrains the [ss.IsActive] flag.
func (ss *Module) csIsActive(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, ss.IsActive)
	isZeroWhenInactive(comp, ss.IsActive, column.Shift(ss.IsActive, -1))

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_ACTIVE_NO_0_TO_1",
		sym.Sub(ss.IsActive, sym.Mul(column.Shift(ss.IsActive, -1), ss.IsActive)),
	)

	// Note: this constraints also makes it invalid to have batch range ending
	// with an account segment for the 0x0 address. Fortunately, this is
	// impossible in practice since at least one EOA account has to be mutated
	// for a transaction and therefore a block to occur.
	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_ACTIVE_TRANSITION_AFTER_ACCOUNT_SEGMENT",
		sym.Mul(
			ss.Account.HasSameAddressAsPrev,
			sym.Sub(ss.IsActive, column.Shift(ss.IsActive, -1)),
		),
	)
}

// IsBeginningOfAccountSegment is a binary indicator column indicating whether
// the current row is corresponding to the beginning of an account segment.
func (ss *Module) csIsBeginningOfAccountSegment(comp *wizard.CompiledIOP) {
	mustBeBinary(comp, ss.IsBeginningOfAccountSegment)
	isZeroWhenInactive(comp, ss.IsBeginningOfAccountSegment, ss.IsActive)

	// IsBeginningOfAccountSegment being one implies that either the batchNumber
	// bumped or that we jumped to another account and reciprocally.
	// We check each limb independently for consistency with BlockNumber representation.
	// We need to consider only the last limb (lsb as big endian) for the batch number bumping
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_IBOAS_IS_ONE_IFF_EITHER_NEW_BATCH_OR_NEW_ADDR"),
		sym.Add(
			sym.Mul(
				ss.IsActive,
				ss.IsBeginningOfAccountSegment,
				sym.Sub(ss.BatchNumber[common.NbLimbU64-1], column.Shift(ss.BatchNumber[common.NbLimbU64-1], -1), 1),
				ss.Account.HasSameAddressAsPrev,
			),
			sym.Mul(
				ss.IsActive,
				sym.Sub(1, ss.IsBeginningOfAccountSegment),
				sym.Add(
					sym.Sub(ss.BatchNumber[common.NbLimbU64-1], column.Shift(ss.BatchNumber[common.NbLimbU64-1], -1)),
					sym.Sub(1, ss.Account.HasSameAddressAsPrev),
				),
			),
		),
	)

	comp.InsertLocal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_IBOA_STARTS_AT_ONE"),
		sym.Sub(ss.IsActive, ss.IsBeginningOfAccountSegment),
	)
}

// IsEndOfAccountSegment is a binary indicator column indicating whether the
// current row corresponds to the end of an account segment.
func (ss *Module) csIsEndOfAccountSegment(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, ss.IsEndOfAccountSegment)
	isZeroWhenInactive(comp, ss.IsEndOfAccountSegment, ss.IsActive)

	// IsEndOfSegment == 1 <==>
	// 		IBOAS[i=1] == 1 || (isActive[i+1] + 1 == isActive[i])
	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_END_OF_ACCOUNT_SEGMENT_PROPERLY_SET",
		sym.Add(
			sym.Mul(
				ss.IsEndOfAccountSegment,
				sym.Sub(1, column.Shift(ss.IsBeginningOfAccountSegment, 1)),
				sym.Sub(ss.IsActive, column.Shift(ss.IsActive, 1), 1),
			),
			sym.Mul(
				sym.Sub(1, ss.IsEndOfAccountSegment),
				sym.Add(
					column.Shift(ss.IsBeginningOfAccountSegment, 1),
					// the constraints on isActive ensures this is non-negative
					sym.Sub(ss.IsActive, column.Shift(ss.IsActive, 1)),
				),
			),
		),
	)

	comp.InsertLocal(
		0,
		"STATE_SUMMARY_IS_END_OF_ACCOUNT_SEGMENT_BOUNDARY_CONDITION",
		sym.Sub(
			column.Shift(ss.IsActive, -1),
			column.Shift(ss.IsEndOfAccountSegment, -1),
		),
	)
}

// csInitialFinalDeployment ensures that the columns ss.IsInitialDeployment and
// ss.IsFinalDeployment are both binary columns such that at least one of the
// two columns is set to 1.
func (ss *Module) csInitialFinalDeployment(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, ss.IsInitialDeployment)
	mustBeBinary(comp, ss.IsFinalDeployment)
	isZeroWhenInactive(comp, ss.IsInitialDeployment, ss.IsActive)
	isZeroWhenInactive(comp, ss.IsFinalDeployment, ss.IsActive)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_INITIAL_FINAL_DEPLOYMENT_ONE_FLAG_AT_LEAST_IS_SET",
		sym.Mul(
			sym.Add(ss.IsInitialDeployment, ss.IsFinalDeployment, sym.Neg(ss.IsActive)),
			sym.Add(ss.IsInitialDeployment, ss.IsFinalDeployment, sym.Neg(ss.IsActive), -1),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_INITIAL_DEPLOYMENT_AT_THE_BEGINNING_OF_AN_ACCOUNT_SEGMENT",
		sym.Mul(
			ss.IsBeginningOfAccountSegment,
			sym.Sub(1, ss.IsInitialDeployment),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_FINAL_DEPLOYMENT_AT_THE_END_OF_AN_ACCOUNT_SEGMENT",
		sym.Mul(
			ss.IsEndOfAccountSegment,
			sym.Sub(1, ss.IsFinalDeployment),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_INITIAL/FINAL_DEPLOYMENT_SUM_IS_CONSTANT",
		sym.Mul(
			// this also remove the constraints when inactive, since
			// isActive = 0 => isBOAS = 0
			sym.Sub(ss.IsBeginningOfAccountSegment, ss.IsActive),
			sym.Sub(
				sym.Add(ss.IsInitialDeployment, ss.IsFinalDeployment),
				sym.Add(
					column.Shift(ss.IsInitialDeployment, -1),
					column.Shift(ss.IsFinalDeployment, -1),
				),
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_INITIAL_FINAL_DEPLOYMENT_STAY_CONSTANT_UNLESS_IS_STORAGE_WAS_FALSE",
		sym.Mul(
			ss.IsActive,
			column.Shift(ss.IsStorage, -1),
			sym.Sub(
				sym.Sub(ss.IsFinalDeployment, ss.IsInitialDeployment),
				sym.Sub(
					column.Shift(ss.IsFinalDeployment, -1),
					column.Shift(ss.IsInitialDeployment, -1),
				),
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_INITIAL/FINAL_DEPLOYMENT_MUST_FLIP_ON_NEW_SUBSEGMENT",
		sym.Mul(
			ss.IsActive,
			sym.Sub(1, column.Shift(ss.IsStorage, -1)),
			sym.Sub(1, ss.IsBeginningOfAccountSegment),
			sym.Add(
				column.Shift(ss.IsInitialDeployment, -1),
				sym.Sub(1, column.Shift(ss.IsFinalDeployment, -1)),
				sym.Sub(1, ss.IsInitialDeployment),
				ss.IsFinalDeployment,
				-4,
			),
		),
	)
}

// csIsDeletionSegment constraints the IsDeletionSegmentColumn to be correctly
// constructed
func (ss *Module) csIsDeletionSegment(comp *wizard.CompiledIOP) {

	isZeroWhenInactive(comp, ss.IsDeleteSegment, ss.IsActive)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_DELETION_SEGMENT_VS_IS_DELETE",
		sym.Sub(
			ss.IsDeleteSegment,
			ternary(
				ss.IsStorage,
				column.Shift(ss.IsDeleteSegment, 1),
				ss.AccumulatorStatement.IsDelete,
			),
		),
	)

}

// csIsStorageSequentiality adds the relevant constraints to ensure that the
// IsStorageFlag is properly set.
func (ss *Module) csIsStorage(comp *wizard.CompiledIOP) {

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_STORAGE_IS_BOOLEAN",
		sym.Mul(
			ss.IsStorage,
			sym.Sub(ss.IsStorage, 1),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_IS_STORAGE_IS_ONE_WHEN_END_OF_ACCOUNT_SEGMENT",
		sym.Sub(
			ss.IsEndOfAccountSegment,
			sym.Mul(
				ss.IsEndOfAccountSegment,
				sym.Sub(1, ss.IsStorage),
			),
		),
	)
}

// csBatchNumber ensures that the BatchNumber column can only
// increment by one. And it can only do so if the
func (ss *Module) csBatchNumber(comp *wizard.CompiledIOP) {

	for i := range common.NbLimbU64 {
		// Big-endian format: last limb (i=3) holds the value, starts from 1
		// Higher limbs (i=0,1,2) are the MSB padding, start from 0
		if i == common.NbLimbU64-1 {
			comp.InsertLocal(
				0,
				ifaces.QueryIDf("STATE_SUMMARY_BATCH_NUMBER_START_FROM_ONE_%d", i),
				sym.Sub(
					ss.BatchNumber[i],
					1,
				),
			)
		} else {
			comp.InsertLocal(
				0,
				ifaces.QueryIDf("STATE_SUMMARY_BATCH_NUMBER_START_FROM_ZERO_%d", i),
				ifaces.ColumnAsVariable(ss.BatchNumber[i]),
			)
		}

		isZeroWhenInactive(comp, ss.BatchNumber[i], ss.IsActive)

		// Each limb can only stay the same (diff=0) or increment by 1 (diff=1)
		// For higher limbs, they will always have diff=0
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_BATCH_NUMBER_CAN_ONLY_INCREMENT_%d", i),
			sym.Mul(
				ss.IsActive,
				sym.Add(
					sym.Mul(
						ss.IsBeginningOfAccountSegment,
						sym.Sub(ss.BatchNumber[i], column.Shift(ss.BatchNumber[i], -1)),
						sym.Sub(ss.BatchNumber[i], column.Shift(ss.BatchNumber[i], -1), 1),
					),
					sym.Mul(
						sym.Sub(1, ss.IsBeginningOfAccountSegment),
						sym.Sub(ss.BatchNumber[i], column.Shift(ss.BatchNumber[i], -1)),
					),
				),
			),
		)
	}
}

// csWorldStateRootSequentiality ensures that the WorldStateRoot column is
// properly set w.r.t. the accumulator statement.
func (ss *Module) csWorldStateRoot(comp *wizard.CompiledIOP) {
	for i := range common.NbElemPerHash {
		isZeroWhenInactive(comp, ss.WorldStateRoot[i], ss.IsActive)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_WORLD_STATE_ROOT_SEQUENTIALITY_WHEN_NOT_WS_ACCESS_%d", i),
			sym.Mul(
				ss.IsStorage,
				sym.Sub(
					column.Shift(ss.WorldStateRoot[i], -1),
					ss.WorldStateRoot[i],
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_WORLD_STATE_ROOT_SEQUENTIALITY_WHEN_WS_ACCESS_OLD_ROOT_%d", i),
			sym.Mul(
				ss.IsActive,
				sym.Sub(1, ss.IsStorage),
				sym.Sub(
					column.Shift(ss.WorldStateRoot[i], -1),
					ss.AccumulatorStatement.StateDiff.InitialRoot[i],
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_WORLD_STATE_ROOT_SEQUENTIALITY_WHEN_WS_ACCESS_NEW_ROOT_%d", i),
			sym.Mul(
				ss.IsActive,
				sym.Sub(1, ss.IsStorage),
				sym.Sub(
					ss.WorldStateRoot[i],
					ss.AccumulatorStatement.StateDiff.FinalRoot[i],
				),
			),
		)
	}
}

// csAccountNew ensures that the account new is updated consistently with the
// account segment structure.
func (ss *Module) csAccountNew(comp *wizard.CompiledIOP) {

	isZeroWhenInactive(comp, ss.Account.Final.Exists, ss.IsActive)

	// mustBeConstantOnSubsegment defines a template for generating the
	// constraints ensuring that the initial account value remains unchanged on
	// an account sub-segment.
	mustBeConstantOnSubsegment := func(col ifaces.Column) {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("%v_IS_CONSTANT_DURING_SUB_SEGMENT", col.GetColID()),
			sym.Mul(
				sym.Add(
					sym.Sub(1, ss.AccumulatorStatement.IsDelete),
					ss.IsStorage,
				),
				column.Shift(ss.IsStorage, -1),
				sym.Sub(col, column.Shift(col, -1)),
			),
		)
	}

	mustBeConstantOnSubsegment(ss.Account.Final.Exists)

	// mustHaveDefaultWhenNotExists defines a template constraint to ensure that
	// `col` uses a default value when Exists = 0
	mustHaveDefaultWhenNotExists := func(col ifaces.Column, def any) {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("%v_HAS_DEFAULT_VALUE", col.GetColID()),
			sym.Mul(
				sym.Sub(1, ss.Account.Final.Exists),
				sym.Sub(col, sym.Mul(ss.IsActive, def)),
			),
		)
	}

	for i := range common.NbLimbU64 {
		mustBeConstantOnSubsegment(ss.Account.Final.CodeSize[i])
		mustBeConstantOnSubsegment(ss.Account.Final.Nonce[i])

		mustHaveDefaultWhenNotExists(ss.Account.Final.CodeSize[i], 0)
		mustHaveDefaultWhenNotExists(ss.Account.Final.Nonce[i], 0)
	}

	for i := range common.NbLimbU128 {
		mustHaveDefaultWhenNotExists(ss.Account.Final.KeccakCodeHash.Hi[i], 0)
		mustHaveDefaultWhenNotExists(ss.Account.Final.KeccakCodeHash.Lo[i], 0)

		mustBeConstantOnSubsegment(ss.Account.Final.KeccakCodeHash.Hi[i])
		mustBeConstantOnSubsegment(ss.Account.Final.KeccakCodeHash.Lo[i])
	}

	for i := range common.NbLimbU256 {
		mustHaveDefaultWhenNotExists(ss.Account.Final.Balance[i], 0)
		mustBeConstantOnSubsegment(ss.Account.Final.Balance[i])
	}

	for i := range poseidon2.BlockSize {
		mustBeConstantOnSubsegment(ss.Account.Final.LineaCodeHash[i])
		mustHaveDefaultWhenNotExists(ss.Account.Final.LineaCodeHash[i], 0)
	}

	for i := range poseidon2.BlockSize {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_STORAGE_ROOT_IS_EMPTY_%d", i),
			sym.Mul(
				sym.Sub(1, ss.Account.Final.Exists),
				sym.Sub(1, ss.IsStorage),
				ss.Account.Final.StorageRoot[i],
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_NEW_STORAGE_ROOT_CONSTANT_ON_SUB_SEGMENT_%d", i),
			sym.Mul(
				ss.IsActive,
				sym.Sub(1, ss.IsStorage),
				column.Shift(ss.IsStorage, -1),
				sym.Sub(
					1,
					ss.AccumulatorStatement.IsReadZero,
					ss.AccumulatorStatement.IsDelete,
				),
				sym.Sub(
					ss.Account.Final.StorageRoot[i],
					column.Shift(ss.Account.Final.StorageRoot[i], -1),
				),
			),
		)
	}
}

// csAccountOld ensures that the account new is updated consistently with the
// account segment structure. Namely, the initial account value may only be the
// one of the
func (ss *Module) csAccountOld(comp *wizard.CompiledIOP) {

	isZeroWhenInactive(comp, ss.Account.Initial.Exists, ss.IsActive)

	// mustBeConstantOnSubsegment defines a template for generating the
	// constraints ensuring that the initial account value remains unchanged on
	// an account sub-segment.
	//
	// limbNum is an argument that adds a number of constrained column limb to the query name.
	// If limbNum is negative, the query name remains unchanged.
	mustBeConstantOnSubsegment := func(col ifaces.Column, limbNum int) {
		qName := ifaces.QueryIDf("%v_IS_CONSTANT_DURING_SUB_SEGMENT", col.GetColID())
		if limbNum >= 0 {
			qName = ifaces.QueryIDf("%v_IS_CONSTANT_DURING_SUB_SEGMENT_%d", col.GetColID(), limbNum)
		}

		comp.InsertGlobal(
			0,
			qName,
			sym.Mul(
				column.Shift(ss.IsStorage, -1),
				sym.Sub(col, column.Shift(col, -1)),
			),
		)
	}

	mustBeConstantOnSubsegment(ss.Account.Initial.Exists, -1)

	// mustHaveDefaultWhenNotExists defines a template constraint to ensure that
	// `col` uses a default value when Exists = 0
	//
	// limbNum is an argument that adds a number of constrained column limb to the query name.
	// If limbNum is negative, the query name remains unchanged.
	mustHaveDefaultWhenNotExists := func(col ifaces.Column, def any, limbNum int) {
		qName := ifaces.QueryIDf("%v_HAS_DEFAULT_VALUE", col.GetColID())
		if limbNum >= 0 {
			qName = ifaces.QueryIDf("%v_HAS_DEFAULT_VALUE_%d", col.GetColID(), limbNum)
		}

		comp.InsertGlobal(
			0,
			qName,
			sym.Mul(
				sym.Sub(1, ss.Account.Initial.Exists),
				sym.Sub(col, sym.Mul(ss.IsActive, def)),
			),
		)
	}

	for i := range common.NbLimbU64 {
		mustBeConstantOnSubsegment(ss.Account.Initial.CodeSize[i], -1)
		mustBeConstantOnSubsegment(ss.Account.Initial.Nonce[i], -1)

		mustHaveDefaultWhenNotExists(ss.Account.Initial.CodeSize[i], 0, -1)
		mustHaveDefaultWhenNotExists(ss.Account.Initial.Nonce[i], 0, -1)
	}

	for i := range common.NbLimbU128 {
		mustHaveDefaultWhenNotExists(ss.Account.Initial.KeccakCodeHash.Hi[i], 0, -1)
		mustHaveDefaultWhenNotExists(ss.Account.Initial.KeccakCodeHash.Lo[i], 0, -1)

		mustBeConstantOnSubsegment(ss.Account.Initial.KeccakCodeHash.Hi[i], -1)
		mustBeConstantOnSubsegment(ss.Account.Initial.KeccakCodeHash.Lo[i], -1)
	}

	for i := range common.NbLimbU256 {
		mustHaveDefaultWhenNotExists(ss.Account.Initial.Balance[i], 0, -1)
		mustBeConstantOnSubsegment(ss.Account.Initial.Balance[i], -1)

	}

	for i := range poseidon2.BlockSize {
		mustBeConstantOnSubsegment(ss.Account.Initial.LineaCodeHash[i], -1)
		mustHaveDefaultWhenNotExists(ss.Account.Initial.LineaCodeHash[i], 0, -1)
	}

	for i := range poseidon2.BlockSize {
		mustBeConstantOnSubsegment(ss.Account.Initial.StorageRoot[i], i)
		mustHaveDefaultWhenNotExists(ss.Account.Initial.StorageRoot[i], 0, i)
	}

}

// csStoragePeek adds all the constraints related to the storage peek of the
// state summary module.
func (ss *Module) csStoragePeek(comp *wizard.CompiledIOP) {

	mustBeZeroWhenNotStorage := func(col ifaces.Column) {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("%v_IS_ZERO_WHEN_NOT_STORAGE", col.GetColID()),
			sym.Sub(col, sym.Mul(ss.IsStorage, col)),
		)
	}

	for i := range common.NbLimbU128 {
		mustBeZeroWhenNotStorage(ss.Storage.Key.Hi[i])
		mustBeZeroWhenNotStorage(ss.Storage.Key.Lo[i])
		mustBeZeroWhenNotStorage(ss.Storage.OldValue.Hi[i])
		mustBeZeroWhenNotStorage(ss.Storage.OldValue.Lo[i])
		mustBeZeroWhenNotStorage(ss.Storage.NewValue.Hi[i])
		mustBeZeroWhenNotStorage(ss.Storage.NewValue.Lo[i])
	}

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_STORAGE_KEY_INCREASES"),
		sym.Mul(
			ss.IsStorage,
			column.Shift(ss.IsStorage, -1),
			ss.AccumulatorStatement.SameTypeAsBefore, // Note: we observed that shomei was in fact sorting by type and then by storage key
			sym.Sub(1, ss.Storage.KeyIncreased),
		),
	)

	diffRoW := sym.Sub(
		sym.Add(
			ss.AccumulatorStatement.IsInsert,
			ss.AccumulatorStatement.IsUpdate,
			ss.AccumulatorStatement.IsDelete,
		),
		sym.Add(
			column.Shift(ss.AccumulatorStatement.IsInsert, -1),
			column.Shift(ss.AccumulatorStatement.IsUpdate, -1),
			column.Shift(ss.AccumulatorStatement.IsDelete, -1),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("STATE_SUMMARY_STORAGE_READS_THEN_WRITE"),
		sym.Mul(
			column.Shift(ss.IsStorage, -1),
			ss.IsStorage,
			sym.Sub(
				sym.Mul(diffRoW, diffRoW),
				diffRoW,
			),
		),
	)
}

// csAccumulatorStatementFlags constraints the accumulator statement's flags.
func (ss *Module) csAccumulatorStatementFlags(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, ss.AccumulatorStatement.IsReadNonZero)
	mustBeBinary(comp, ss.AccumulatorStatement.IsReadZero)
	mustBeBinary(comp, ss.AccumulatorStatement.IsInsert)
	mustBeBinary(comp, ss.AccumulatorStatement.IsUpdate)
	mustBeBinary(comp, ss.AccumulatorStatement.IsDelete)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_ACC_STATEMENT_FLAGS_MUTUALLY_EXCLUSIVE",
		sym.Add(
			ss.AccumulatorStatement.IsReadZero,
			ss.AccumulatorStatement.IsReadNonZero,
			ss.AccumulatorStatement.IsInsert,
			ss.AccumulatorStatement.IsUpdate,
			ss.AccumulatorStatement.IsDelete,
			sym.Neg(ss.IsActive),
		),
	)

	oldValueLimbExpressions := make([]any, 0, poseidon2.BlockSize)
	for i := range poseidon2.BlockSize {
		oldValueLimbExpressions = append(oldValueLimbExpressions, sym.Sub(1, ss.Storage.OldValueIsZero[i]))
	}

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_OLD_STORAGE_ZEROIZATION",
		sym.Mul(
			ss.IsStorage,
			sym.Add(
				ss.AccumulatorStatement.IsReadZero,
				ss.AccumulatorStatement.IsInsert,
				sym.Neg(sym.Sub(1, sym.Mul(oldValueLimbExpressions...))),
			),
		),
	)

	zeroizationLibsExpressions := make([]any, 0, poseidon2.BlockSize)
	for i := range poseidon2.BlockSize {
		zeroizationLibsExpressions = append(zeroizationLibsExpressions, sym.Sub(1, ss.AccumulatorStatement.FinalHValIsZero[i]))
	}

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_NEW_STORAGE_ZEROIZATION",
		sym.Mul(
			ss.IsStorage,
			sym.Add(
				ss.AccumulatorStatement.IsReadZero, ss.AccumulatorStatement.IsDelete,
				sym.Neg(sym.Sub(1, sym.Mul(zeroizationLibsExpressions...))),
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_OLD_ACCOUNT_ZEROIZATION",
		sym.Mul(
			ss.IsActive,
			sym.Sub(1, ss.IsStorage),
			sym.Add(
				ss.AccumulatorStatement.IsReadZero,
				ss.AccumulatorStatement.IsInsert,
				sym.Sub(ss.Account.Initial.Exists, 1),
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_NEW_ACCOUNT_ZEROIZATION",
		sym.Mul(
			ss.IsActive,
			sym.Sub(1, ss.IsStorage),
			sym.Add(
				ss.AccumulatorStatement.IsReadZero,
				ss.AccumulatorStatement.IsDelete,
				sym.Sub(ss.Account.Final.Exists, 1),
			),
		),
	)

	oldNewStorageEqualLimbs := make([]any, 0, poseidon2.BlockSize)
	for i := range poseidon2.BlockSize {
		oldNewStorageEqualLimbs = append(oldNewStorageEqualLimbs, ss.AccumulatorStatement.InitialAndFinalHValAreEqual[i])
	}

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_OLD_NEW_STORAGE_EQUAL",
		sym.Mul(
			ss.IsStorage,
			sym.Add(
				ss.AccumulatorStatement.IsReadNonZero, ss.AccumulatorStatement.IsReadZero,
				sym.Neg(sym.Mul(oldNewStorageEqualLimbs...)),
			),
		),
	)

	sameBitLimbExpressions := make([]any, 0, poseidon2.BlockSize)
	for i := range poseidon2.BlockSize {
		sameBitLimbExpressions = append(sameBitLimbExpressions, sym.Sub(1, ss.Account.InitialAndFinalAreSame[i]))
	}

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_OLD_NEW_ACCOUNT_EQUAL",
		sym.Mul(
			ss.IsActive,
			sym.Sub(1, ss.IsStorage),
			sym.Add(
				ss.AccumulatorStatement.IsReadNonZero,
				ss.AccumulatorStatement.IsReadZero,
				sym.Neg(sym.Sub(1, sym.Mul(sameBitLimbExpressions...))),
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_READ_ZERO_IS_A_SINGLE_ROW",
		sym.Mul(
			ss.AccumulatorStatement.IsReadZero,
			sym.Sub(1, ss.IsStorage),
			sym.Add(
				ss.IsBeginningOfAccountSegment,
				ss.IsEndOfAccountSegment,
				-2,
			),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_FIRST_SUBSEGMENT_IS_ALWAYS_DELETION",
		sym.Mul(
			sym.Sub(1, ss.IsStorage),
			ss.IsInitialDeployment,
			sym.Sub(1, ss.IsFinalDeployment),
			sym.Sub(1, ss.AccumulatorStatement.IsDelete),
		),
	)

	comp.InsertGlobal(
		0,
		"STATE_SUMMARY_SECOND_SUBSEGMENT_IS_ALWAYS_DELETION",
		sym.Mul(
			sym.Sub(1, ss.IsStorage),
			sym.Sub(1, ss.IsInitialDeployment),
			ss.IsFinalDeployment,
			sym.Sub(1, ss.AccumulatorStatement.IsInsert),
		),
	)
}

// csAccumulatorStatementHValKey ensures that the hkeys and kvals provided to the
// accumulator statement are picked from the correct origin. Either from the
// StorageHashing or from the AccountHashing. This is done using binary
// selectors.
func (ss *Module) csAccumulatorStatementHValKey(comp *wizard.CompiledIOP) {
	for i := range poseidon2.BlockSize {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_ACC_STATEMENT_HKEY_%d", i),
			sym.Sub(
				ss.AccumulatorStatement.StateDiff.HKey[i],
				sym.Mul(
					ss.IsActive,
					ternary(ss.IsStorage, ss.Storage.KeyHash[i], ss.Account.AddressHash[i]),
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_ACC_STATEMENT_INITIAL_HVAL_%d", i),
			sym.Sub(
				ss.AccumulatorStatement.StateDiff.InitialHVal[i],
				sym.Mul(
					sym.Sub(ss.IsActive, ss.AccumulatorStatement.IsReadZero, ss.AccumulatorStatement.IsInsert),
					ternary(ss.IsStorage, ss.Storage.OldValueHash[i], ss.Account.HashInitial[i]),
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_ACC_STATEMENT_FINAL_HVAL_%d", i),
			sym.Sub(
				ss.AccumulatorStatement.StateDiff.FinalHVal[i],
				ternary(ss.IsStorage,
					ternary(
						ss.IsDeleteSegment,
						ss.AccumulatorStatement.StateDiff.InitialHVal[i],
						sym.Mul(
							ss.Storage.NewValueHash[i],
							sym.Sub(
								ss.IsActive,
								ss.AccumulatorStatement.IsReadZero,
								ss.AccumulatorStatement.IsDelete,
							),
						),
					),
					sym.Mul(
						ss.Account.HashFinal[i],
						sym.Sub(
							ss.IsActive,
							ss.AccumulatorStatement.IsReadZero,
							ss.AccumulatorStatement.IsDelete,
						),
					),
				),
			),
		)
	}
}

// csAccumulatorRoots constrains the "roots" provided to the accumulator
// statement.
func (ss *Module) csAccumulatorRoots(comp *wizard.CompiledIOP) {

	// IsBeginningOfAccountSegment && IsStorage
	for i := range common.NbElemPerHash {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_STORAGE_ROOT_HASH_SEQUENTIALITY_BEGINNING_OF_SUBSEGMENT_%d", i),
			sym.Mul(
				ss.IsStorage,
				sym.Sub(1, column.Shift(ss.IsStorage, -1)),
				sym.Add(
					sym.Mul(
						ss.Account.Initial.Exists,
						sym.Sub(ss.Account.Initial.StorageRoot[i], ss.AccumulatorStatement.StateDiff.InitialRoot[i]),
					),
					sym.Mul(
						sym.Sub(1, ss.Account.Initial.Exists),
						sym.Sub(emptyStorageRoot[i], ss.AccumulatorStatement.StateDiff.InitialRoot[i]),
					),
				),
			),
		)

		comp.InsertLocal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_STORAGE_ROOT_HASH_SEQUENTIALITY_FIRST_ROW_%d", i),
			sym.Mul(
				ss.IsStorage,
				sym.Add(
					sym.Mul(
						ss.Account.Initial.Exists,
						sym.Sub(ss.Account.Initial.StorageRoot[i], ss.AccumulatorStatement.StateDiff.InitialRoot[i]),
					),
					sym.Mul(
						sym.Sub(1, ss.Account.Initial.Exists),
						sym.Sub(emptyStorageRoot[i], ss.AccumulatorStatement.StateDiff.InitialRoot[i]),
					),
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_ACCOUNT_ROOT_HASH_SEQUENTIALITY_MIDDLE_OF_SUBSEGMENT_%d", i),
			sym.Mul(
				ss.IsStorage,
				column.Shift(ss.IsStorage, -1),
				sym.Sub(
					column.Shift(ss.Account.Final.StorageRoot[i], -1),
					ss.AccumulatorStatement.StateDiff.InitialRoot[i],
				),
			),
		)

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("STATE_SUMMARY_STORAGE_ROOT_HASH_SEQUENTIALITY_MIDDLE_OF_SEGMENT_%d", i),
			sym.Mul(
				ss.IsStorage,
				sym.Sub(
					ss.Account.Final.StorageRoot[i],
					ss.AccumulatorStatement.StateDiff.FinalRoot[i],
				),
			),
		)
	}
}

// constrainExpectedHubCodeHash constrains the ExpectedHubCodeHash columns
// using the KeccakCodeHash information from the state summary
func (ss *Module) constrainExpectedHubCodeHash(comp *wizard.CompiledIOP) {
	for i := range common.NbLimbU128 {
		// if account exists we have the same Keccak code hash
		// if account does not exist we have the empty code hash in what is expected
		// from the HUB
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_INITIAL_CASE_EXISTENT_HI_%d", i),
			sym.Mul(
				ss.Account.Initial.Exists,
				sym.Sub(
					ss.Account.Initial.KeccakCodeHash.Hi[i],
					ss.Account.Initial.ExpectedHubCodeHash.Hi[i],
				),
			),
		)

		// initial case Lo, existent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_INITIAL_CASE_EXISTENT_LO_%d", i),
			sym.Mul(
				ss.Account.Initial.Exists,
				sym.Sub(
					ss.Account.Initial.KeccakCodeHash.Lo[i],
					ss.Account.Initial.ExpectedHubCodeHash.Lo[i],
				),
			),
		)

		// initial case Hi, nonexistent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_INITIAL_CASE_NON_EXISTENT_HI_%d", i),
			sym.Mul(
				ss.IsActive, // only on the active part of the module
				sym.Sub(
					1,
					ss.Account.Initial.Exists,
				),
				sym.Sub(
					ss.Account.Initial.ExpectedHubCodeHash.Hi[i],
					EMPTYKECCAKCODEHASH_HI[i],
				),
			),
		)

		// initial case Lo, nonexistent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_INITIAL_CASE_NON_EXISTENT_LO_%d", i),
			sym.Mul(
				ss.IsActive, // only on the active part of the module
				sym.Sub(
					1,
					ss.Account.Initial.Exists,
				),
				sym.Sub(
					ss.Account.Initial.ExpectedHubCodeHash.Lo[i],
					EMPTYKECCAKCODEHASH_LO[i],
				),
			),
		)

		// final checks
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_FINALL_CASE_EXISTENT_HI_%d", i),
			sym.Mul(
				ss.Account.Final.Exists,
				sym.Sub(
					ss.Account.Final.KeccakCodeHash.Hi[i],
					ss.Account.Final.ExpectedHubCodeHash.Hi[i],
				),
			),
		)

		// final case Lo, existent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_FINAL_CASE_EXISTENT_LO_%d", i),
			sym.Mul(
				ss.Account.Final.Exists,
				sym.Sub(
					ss.Account.Final.KeccakCodeHash.Lo[i],
					ss.Account.Final.ExpectedHubCodeHash.Lo[i],
				),
			),
		)

		// final case Hi, nonexistent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_FINAL_CASE_NON_EXISTENT_HI_%d", i),
			sym.Mul(
				ss.IsActive, // only on the active part of the module
				sym.Sub(
					1,
					ss.Account.Final.Exists,
				),
				sym.Sub(
					ss.Account.Final.ExpectedHubCodeHash.Hi[i],
					EMPTYKECCAKCODEHASH_HI[i],
				),
			),
		)

		// final case Lo, nonexistent accounts
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("GLOBAL_CONSTRAINT_EXPECTED_HUB_CODEHASH_FINAL_CASE_NON_EXISTENT_LO_%d", i),
			sym.Mul(
				ss.IsActive, // only on the active part of the module
				sym.Sub(
					1,
					ss.Account.Final.Exists,
				),
				sym.Sub(
					ss.Account.Final.ExpectedHubCodeHash.Lo[i],
					EMPTYKECCAKCODEHASH_LO[i],
				),
			),
		)
	}
}

// ternary is a small utility to construct ternaries is constraints
func ternary(cond, if1, if0 any) *sym.Expression {
	return sym.Add(
		sym.Mul(sym.Sub(1, cond), if0),
		sym.Mul(cond, if1),
	)
}

// isZeroWhenInactive constraints the column to cancel when inactive.
func isZeroWhenInactive(comp *wizard.CompiledIOP, c, isActive ifaces.Column) {
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_IS_ZERO_WHEN_INACTIVE", c.GetColID()),
		sym.Sub(c, sym.Mul(c, isActive)),
	)
}

// mustBeBinary constrains the current column to be binary.
func mustBeBinary(comp *wizard.CompiledIOP, c ifaces.Column) {
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_MUST_BE_BINARY", c.GetColID()),
		sym.Mul(c, sym.Sub(c, 1)),
	)
}
