package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	commoncs "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

// csIsActiveActivation constraints that IsActive module to be only one for antichamber rounds.
func (ac *Antichamber) csIsActiveActivation(comp *wizard.CompiledIOP) {
	// IsActive must be binary and cannot transition from 0 to 1
	commoncs.MustBeActivationColumns(comp, ac.IsActive)
}

func (ac *Antichamber) csZeroWhenInactive(comp *wizard.CompiledIOP) {
	commoncs.MustZeroWhenInactive(comp, ac.IsActive, ac.cols(false)...)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive, ac.EcRecover.cols()...)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive, ac.Addresses.cols()...)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive, ac.TxSignature.cols()...)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive, ac.UnalignedGnarkData.cols()...)
}

func (ac *Antichamber) csConsistentPushingFetching(comp *wizard.CompiledIOP) {
	// pushing and fetching must be binary
	commoncs.MustBeBinary(comp, ac.IsPushing)
	commoncs.MustBeBinary(comp, ac.IsFetching)
	// pushing and fetching cannot be active at the same time
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_CONSISTENT_PUSHING_FETCHING", NAME_ANTICHAMBER),
		sym.Sub(sym.Add(ac.IsPushing, ac.IsFetching), ac.IsActive),
	)
}

func (ac *Antichamber) csIDSequential(comp *wizard.CompiledIOP) {
	idDiff := sym.Sub(ac.ID, column.Shift(ac.ID, -1))
	// ID must be sequential
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_ID_SEQUENTIAL", NAME_ANTICHAMBER),
		sym.Mul(ac.IsActive, sym.Mul(idDiff, sym.Sub(idDiff, 1))),
	)
}

func (ac *Antichamber) csSource(comp *wizard.CompiledIOP) {
	// source must be binary
	// Source=0 <> ECRecover, Source=1 <> TxSignature
	commoncs.MustBeBinary(comp, ac.Source)
}

func (ac *Antichamber) csTransitions(comp *wizard.CompiledIOP) {
	// stop fetching when have received ecrecover address limbs
	// TODO: store the condition as a variable to use later
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_ECRECOVER_FETCHING", NAME_ANTICHAMBER),
		sym.Mul(
			sym.Sub(1, ac.Source),          // Source[i] = EcRecover AND
			ac.IsFetching,                  // IsFetching[i] = 1 AND
			ac.EcRecover.EcRecoverIsRes,    // EcRecoverIsRes[i] = 1 AND
			ac.EcRecover.EcRecoverIndex,    // EcRecoverIndex[i] = 1 THEN
			column.Shift(ac.IsFetching, 1), // IsFetching[i+1] = 0 => IsFetching_shifted[i] = 0
		),
	)

	// stop fetching when recived txsignature
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_TXSIGNATURE_FETCHING", NAME_ANTICHAMBER),
		sym.Mul(
			ac.Source,                      // Source[i] = TxSignature AND
			ac.IsFetching,                  // IsFetching[i] = 1 THEN
			column.Shift(ac.IsFetching, 1), // IsFetching[i+1] = 0
		),
	)

	// turn on fetching
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_FETCHING_ON", NAME_ANTICHAMBER),
		sym.Mul(
			column.Shift(ac.IsActive, 1),    // IsActive[i+1] = 1 AND
			sym.Sub(1, ac.IsFetching),       // IsFetching[i] = 0 AND
			ac.UnalignedGnarkData.IsIndex13, // GnarkIndex[i] = 13 THEN
			sym.Add(
				sym.Sub(column.Shift(ac.IsFetching, 1), column.Shift(ac.IsActive, 1)), // IsFetching[i+1] = IsActive[i+1] AND
				sym.Mul(2, sym.Sub(column.Shift(ac.ID, 1), ac.ID, 1)),                 // ID[i+1] = ID[i] + 1
			),
		),
	)

	// X, Y are binary constrained
	// X AND Y == X * Y
	// X OR Y == 1-((1-X)*(1-Y))
	// NOT X == 1-X
	// X => Y == X*(1-Y)
	// NOT Y => NOT X == X => Y
	// X XOR Y = X+Y-2*X*Y

	/*
		If NOT(IsFetching[i] = 0 AND GnarkIndex[i] = 13):
		If IsFetching[i] == 1 OR IsGnarkIndex13[i] == 0
		IsActive[i+1] = IsActive[i] AND
		ID[i+1] = ID[i] AND
		Source[i+1] = Source[i] AND
	*/
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_CONSISTENCY", NAME_ANTICHAMBER),
		sym.Mul(
			sym.Sub(1, sym.Mul(
				sym.Sub(1, ac.IsFetching),
				ac.UnalignedGnarkData.IsIndex13,
			)),
			sym.Sub(ac.IsActive, column.Shift(ac.IsActive, 1)),
		),
	)
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_CONSISTENCY_2", NAME_ANTICHAMBER),
		sym.Mul(
			sym.Sub(1, sym.Mul(
				sym.Sub(1, ac.IsFetching),
				ac.UnalignedGnarkData.IsIndex13,
			)),
			sym.Sub(column.Shift(ac.ID, 1), ac.ID),
		),
	)
	comp.InsertGlobal(
		ROUND_NR,
		ifaces.QueryIDf("%v_TRANSITIONS_CONSISTENCY_3", NAME_ANTICHAMBER),
		sym.Mul(
			sym.Sub(1, sym.Mul(
				sym.Sub(1, ac.IsFetching),
				ac.UnalignedGnarkData.IsIndex13,
			)),
			sym.Sub(ac.Source, column.Shift(ac.Source, 1)),
		),
	)
}
