package ecpair

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	common "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

func (ec *ECPair) csIsActiveActivation(comp *wizard.CompiledIOP) {
	// IsActive is binary and cannot transition from 0 to 1
	common.MustBeActivationColumns(comp, ec.IsActive)
}

func (ec *ECPair) csBinaryConstraints(comp *wizard.CompiledIOP) {
	common.MustBeBinary(comp, ec.UnalignedPairingData.IsFirstLineOfInstance)
	common.MustBeBinary(comp, ec.UnalignedPairingData.IsFirstLineOfPrevAccumulator)
	common.MustBeBinary(comp, ec.UnalignedPairingData.IsFirstLineOfCurrAccumulator)
}

func (ec *ECPair) csFlagConsistency(comp *wizard.CompiledIOP) {
	// flag consistency. That the assigned data is pulled from input or computed.
	common.MustBeMutuallyExclusiveBinaryFlags(comp, ec.UnalignedPairingData.IsActive, []ifaces.Column{
		ec.UnalignedPairingData.IsPulling,
		ec.UnalignedPairingData.IsComputed,
	})
	common.MustBeMutuallyExclusiveBinaryFlags(comp, ec.UnalignedG2MembershipData.ToG2MembershipCircuitMask, []ifaces.Column{
		ec.UnalignedG2MembershipData.IsPulling,
		ec.UnalignedG2MembershipData.IsComputed,
	})
}

func (ec *ECPair) csOffWhenInactive(comp *wizard.CompiledIOP) {
	cols := []ifaces.Column{
		ec.UnalignedPairingData.InstanceID,
		ec.UnalignedPairingData.PairID,
		ec.UnalignedPairingData.TotalPairs,
		ec.UnalignedPairingData.Index,
		ec.UnalignedPairingData.IsFirstLineOfInstance,
		ec.UnalignedPairingData.IsFirstLineOfPrevAccumulator,
		ec.UnalignedPairingData.IsFirstLineOfCurrAccumulator,
		ec.UnalignedG2MembershipData.SuccessBit,
	}
	cols = append(cols, ec.UnalignedPairingData.Limbs.ToRawUnsafe()...)
	cols = append(cols, ec.UnalignedG2MembershipData.Limbs.ToRawUnsafe()...)

	// nothing is set when inactive
	common.MustZeroWhenInactive(comp, ec.IsActive, cols...)
}

func (ec *ECPair) csProjections(comp *wizard.CompiledIOP) {
	// we project data from the arithmetization correctly to the unaligned part of the circuit
	comp.InsertProjection(ifaces.QueryIDf("%v_PROJECTION_PAIRING", nameECPair),
		query.ProjectionInput{
			ColumnA: append(
				ec.ECPairSource.Limbs.ToLittleEndianLimbs().ToRawUnsafe(),
				ec.ECPairSource.ID, ec.ECPairSource.IsEcPairingResult,
			),
			ColumnB: append(
				ec.UnalignedPairingData.Limbs.ToLittleEndianLimbs().ToRawUnsafe(),
				ec.UnalignedPairingData.InstanceID, ec.UnalignedPairingData.IsResultOfInstance,
			),
			FilterA: ec.ECPairSource.CsEcpairing,
			FilterB: ec.UnalignedPairingData.IsPulling,
		})

	comp.InsertProjection(
		ifaces.QueryIDf("%v_PROJECTION_MEMBERSHIP", nameECPair),
		query.ProjectionInput{
			ColumnA: append(
				ec.ECPairSource.Limbs.ToLittleEndianLimbs().ToRawUnsafe(),
				ec.ECPairSource.SuccessBit),
			ColumnB: append(
				ec.UnalignedG2MembershipData.Limbs.ToLittleEndianLimbs().ToRawUnsafe(),
				ec.UnalignedG2MembershipData.SuccessBit),
			FilterA: ec.ECPairSource.CsG2Membership,
			FilterB: ec.UnalignedG2MembershipData.IsPulling,
		})
}

func (ec *ECPair) csMembershipComputedResult(comp *wizard.CompiledIOP) {
	// when row is IS_COMPUTED, then this corresponds to the result of the G2
	// membership check. In the source it is in separate column and we have to
	// show that it corresponds to the column (but in previous row).

	successBitAsRow := limbs.KoalaAsLimb[limbs.LittleEndian](
		column.Shift(ec.UnalignedG2MembershipData.SuccessBit, -1),
		128,
	)

	// as the limbs divide into 8 columns, last 7 of them are 0.
	limbs.NewGlobal(
		comp,
		ifaces.QueryIDf("%v_MEMBERSHIP_CHECK_RESULT_EMPTY_LIMB", nameECPair),
		sym.Mul(
			ec.UnalignedG2MembershipData.IsComputed,
			sym.Sub(
				ec.UnalignedG2MembershipData.Limbs,
				successBitAsRow,
			),
		),
	)
}

func (ec *ECPair) csConstantWhenIsComputing(comp *wizard.CompiledIOP) {
	// we want to ensure that when we compute lines then PAIRING_ID and
	// TOTAL_PAIRINGS is consistent with the projected values

	// IF IS_COMPUTING AND IS_ACTIVE AND NOT FIRST_LINE => PAIRING_ID_{i} = PAIRING_ID_{i-1} AND TOTAL_PAIRINGS_{i} = TOTAL_PAIRINGS_{i-1}
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_COUNTERS_CONSISTENCY", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsComputed,
			sym.Sub(1, ec.UnalignedPairingData.IsFirstLineOfInstance),
			sym.Sub(ec.UnalignedPairingData.PairID, column.Shift(ec.UnalignedPairingData.PairID, -1)),
			sym.Sub(ec.UnalignedPairingData.TotalPairs, column.Shift(ec.UnalignedPairingData.TotalPairs, -1)),
		),
	)
}

func (ec *ECPair) csInstanceIDChangeWhenNewInstance(comp *wizard.CompiledIOP) {
	// when we are at the first line of the new instance then the instance ID
	// should change
	prevEqualCurrID, cptPrevEqualCurrID := dedicated.IsZero(
		comp,
		sym.Sub(
			ec.UnalignedPairingData.InstanceID,
			column.Shift(ec.UnalignedPairingData.InstanceID, -1),
		),
	).GetColumnAndProverAction()

	ec.CptPrevEqualCurrID = cptPrevEqualCurrID

	// IF IS_ACTIVE AND FIRST_LINE AND INSTANCE_ID != 0 => INSTANCE_ID_{i} = INSTANCE_ID_{i-1} + 1
	// And the constraint does not apply on the first row.
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_INSTANCE_ID_CHANGE", nameECPair),
		sym.Mul(
			column.Shift(verifiercol.NewConstantCol(field.One(), ec.UnalignedPairingData.IsActive.Size(), "ecpair-cancelling-first-row"), -1), // this "useless" line helps cancelling the constraint on the first row
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfInstance,
			ec.UnalignedPairingData.InstanceID,
			prevEqualCurrID,
		),
	)
}

func (ec *ECPair) csAccumulatorInit(comp *wizard.CompiledIOP) {

	// At the beginning of pairing, the accumulator value must be set to
	// Gt = 1, which we encode in the accumulator as (1, 0, 0, 0, 0, 0, 0, 0)

	rowForOne := limbs.RowFromInt[limbs.LittleEndian](1, 128)
	rowForZero := limbs.RowFromInt[limbs.LittleEndian](0, 128)

	enforceRow := func(offset int, row symbolic.Metadata) {
		limbs.NewGlobal(comp,
			ifaces.QueryIDf("%v_ACCUMULATOR_INIT_EMPTY_PART_%d", nameECPair, offset),
			sym.Mul(
				ec.UnalignedPairingData.IsActive,
				column.Shift(ec.UnalignedPairingData.IsFirstLineOfInstance, -offset),
				sym.Sub(row, ec.UnalignedPairingData.Limbs),
			),
		)
	}

	// first HI=0, LO=1
	enforceRow(0, rowForZero)
	enforceRow(1, rowForOne)

	// rest HI=0, LO=0
	for i := 2; i < nbGtLimbs; i++ {
		enforceRow(i, rowForZero)
	}
}

func (ec *ECPair) csAccumulatorConsistency(comp *wizard.CompiledIOP) {
	// that the accumulator between pairs is consistent
	comp.InsertProjection(
		ifaces.QueryIDf("%v_ACCUMULATOR_CONSISTENCY", nameECPair),
		query.ProjectionInput{
			ColumnA: ec.UnalignedPairingData.Limbs.ToRawUnsafe(),
			ColumnB: ec.UnalignedPairingData.Limbs.ToRawUnsafe(),
			FilterA: ec.UnalignedPairingData.IsAccumulatorCurr,
			FilterB: ec.UnalignedPairingData.IsAccumulatorPrev,
		},
	)
}

func (ec *ECPair) csTotalPairs(comp *wizard.CompiledIOP) {
	// total pairs corresponds to the number of pairs in the instance. for this
	// we check that when the limb corresponds to the result, then the PairID of
	// the shifted by two corresponds to the total pairs. the limb corresponds
	// to the result when its PairID is 0 (for input pairs the indexing starts
	// from 1).
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_TOTAL_PAIRS", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsResultOfInstance,
			sym.Sub(ec.UnalignedPairingData.TotalPairs, column.Shift(ec.UnalignedPairingData.PairID, -2)), // we have two limbs for the result, shift by two gets to the input
		),
	)
}

func (ec *ECPair) csLastPairToFinalExp(comp *wizard.CompiledIOP) {
	// if the last pair then the final exp circuit should be active

	// IF IS_ACTIVE AND (PAIR_ID == TOTAL_PAIRS OR PAIR_ID == 0) => TO_FINAL_EXP_CIRCUIT_MASK = 1
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_LAST_PAIR_TO_FINAL_EXP", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			sym.Sub(ec.UnalignedPairingData.PairID, ec.UnalignedPairingData.TotalPairs),
			ec.UnalignedPairingData.PairID,
			ec.UnalignedPairingData.ToFinalExpCircuitMask,
		),
	)
}

func (ec *ECPair) csIndexConsistency(comp *wizard.CompiledIOP) {
	// index switches to zero when the first line of new instance. Otherwise increases
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_INDEX_START", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfInstance,
			ec.UnalignedPairingData.Index,
		),
	)
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_INDEX_INCREMENT", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			sym.Sub(1, ec.UnalignedPairingData.IsFirstLineOfInstance),
			sym.Sub(ec.UnalignedPairingData.Index, column.Shift(ec.UnalignedPairingData.Index, -1), 1),
		),
	)
}

func (ec *ECPair) csAccumulatorMask(comp *wizard.CompiledIOP) {
	// accumulator sum is IS_COMPUTED
	common.MustBeMutuallyExclusiveBinaryFlags(comp, ec.UnalignedPairingData.IsComputed, []ifaces.Column{
		ec.UnalignedPairingData.IsAccumulatorCurr,
		ec.UnalignedPairingData.IsAccumulatorPrev,
		ec.UnalignedPairingData.IsAccumulatorInit,
	})

	// first prev accumulator is 1 when pairID*60 == index
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_FIRST_ACC_PREV", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfPrevAccumulator,
			sym.Sub(
				sym.Mul(nbG1Limbs+nbG2Limbs+2*nbGtLimbs, sym.Sub(ec.UnalignedPairingData.PairID, 1)),
				ec.UnalignedPairingData.Index,
			),
		),
	)

	// first curr accumulator is 1 when pairID*60-24 = index
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_FIRST_ACC_CURR", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfCurrAccumulator,
			sym.Sub(
				sym.Mul(nbG1Limbs+nbG2Limbs+2*nbGtLimbs, ec.UnalignedPairingData.PairID),
				ec.UnalignedPairingData.Index,
				24,
			),
		),
	)

	sumMask := func(col ifaces.Column) *sym.Expression {
		r := sym.NewConstant(0)
		for i := 0; i < nbGtLimbs; i++ {
			r = sym.Add(r, column.Shift(col, i))
		}
		return r
	}
	// init accumulator mask is 1 when at the start of the instance
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_INIT_ACC_MASK", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfInstance,
			sym.Sub(nbGtLimbs, sumMask(ec.UnalignedPairingData.IsAccumulatorInit)),
		),
	)

	// curr accumulator mask is 1 when at the end of the pair
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_CURR_ACC_MASK", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfCurrAccumulator,
			sym.Sub(nbGtLimbs, sumMask(ec.UnalignedPairingData.IsAccumulatorCurr)),
		),
	)

	// prev accumulator mask is 1 when at the start of the pair
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_PREV_ACC_MASK", nameECPair),
		sym.Mul(
			ec.UnalignedPairingData.IsActive,
			ec.UnalignedPairingData.IsFirstLineOfPrevAccumulator,
			sym.Sub(nbGtLimbs, sumMask(ec.UnalignedPairingData.IsAccumulatorPrev)),
		),
	)
}

func (ec *ECPair) csPairingDataOrMembershipActive(comp *wizard.CompiledIOP) {
	// when module is active, then either pairing data or membership data is
	// active. Can also be both.
	comp.InsertGlobal(
		roundNr,
		ifaces.QueryIDf("%v_PAIRING_OR_MEMBERSHIP_ACTIVE", nameECPair),
		sym.Add(
			ec.IsActive,
			sym.Neg(ec.UnalignedPairingData.IsActive),
			sym.Neg(ec.UnalignedG2MembershipData.ToG2MembershipCircuitMask),
			sym.Mul(ec.UnalignedPairingData.IsActive, ec.UnalignedG2MembershipData.ToG2MembershipCircuitMask),
		),
	)
}

func (ec *ECPair) csExclusivePairingCircuitMasks(comp *wizard.CompiledIOP) {
	// the pairing circuit masks are mutually exclusive
	common.MustBeMutuallyExclusiveBinaryFlags(comp, ec.UnalignedPairingData.IsActive, []ifaces.Column{
		ec.UnalignedPairingData.ToMillerLoopCircuitMask,
		ec.UnalignedPairingData.ToFinalExpCircuitMask,
	})
}
