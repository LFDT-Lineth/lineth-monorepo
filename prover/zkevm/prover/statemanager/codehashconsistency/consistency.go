// codehashconsistency implements the necessary constraints to enforce consistency
// between the mimccodehash module and the statesummary module. The constraints
// generated in this package essentially aim at ensuring that
package codehashconsistency

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/mimccodehash"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/statesummary"
)

// Module stores the column expressing the consistency between the
// [mimccodehash.Module] (computing the MiMCCodeHash from the code in the ROM
// module and exposing the corresponding codehash) and the [statesummary.Module]
// (recording the access to the state, thus holding the KeccakCodeHash and the
// MiMCCode hash of the registered contracts). The consistency check is achieved
// by ensuring that if a Keccak is present in both the MiMCCodeHash and the
// StateSummary, then the corresponding KeccakCodeHash must be equal. This
// ensures that code exposed in the ROM module is the same as what is in the
// state.
type Module struct {
	StateSummaryInput *statesummary.Module
	MimcCodeHashInput *mimccodehash.Module

	IsActive        ifaces.Column
	StateSumKeccak  common.HiLoColumns
	StateSumMiMC    ifaces.Column
	StateSumOngoing ifaces.Column
	RomKeccak       common.HiLoColumns
	RomMiMC         ifaces.Column
	RomOngoing      ifaces.Column

	StateSumKeccakLimbs byte32cmp.LimbColumns
	RomKeccakLimbs      byte32cmp.LimbColumns

	CptStateSumKeccakLimbsHi wizard.ProverAction
	CptStateSumKeccakLimbsLo wizard.ProverAction
	CptRomKeccakLimbsHi      wizard.ProverAction
	CptRomKeccakLimbsLo      wizard.ProverAction
	CmpStateSumLimbs         wizard.ProverAction
	CmpRomLimbs              wizard.ProverAction
	CmpRomVsStateSumLimbs    wizard.ProverAction

	StateSumIsGtRom, StateSumIsEqRom, StateSumIsLtRom ifaces.Column
	StateSumIsConst, StateSumIncreased                ifaces.Column
	RomIsConst, RomIncreased                          ifaces.Column
}

// NewModule returns a constrained [Module] connecting `ss` with `mch`. `name`
// is used as a prefix for the name of all the generated columns and constraints.
func NewModule(comp *wizard.CompiledIOP, name string, ss *statesummary.Module, mch *mimccodehash.Module) Module {

	name = name + "_CODEHASH_CONSISTENCY"
	size := 1 << 16

	ch := Module{
		StateSummaryInput: ss,
		MimcCodeHashInput: mch,
		IsActive:          comp.InsertCommit(0, ifaces.ColID(name+"_IS_ACTIVE"), size),
		StateSumKeccak:    common.NewHiLoColumns(comp, size, name+"_STATE_SUMMARY_KECCAK"),
		RomKeccak:         common.NewHiLoColumns(comp, size, name+"_ROM_KECCAK"),
		StateSumMiMC:      comp.InsertCommit(0, ifaces.ColID(name+"_STATE_SUMMARY_MIMC"), size),
		RomMiMC:           comp.InsertCommit(0, ifaces.ColID(name+"_ROM_MIMC"), size),
		RomOngoing:        comp.InsertCommit(0, ifaces.ColID(name+"_ROM_ONGOING"), size),
		StateSumOngoing:   comp.InsertCommit(0, ifaces.ColID(name+"_STATE_SUM_ONGOING"), size),
	}

	commonconstraints.MustBeActivationColumns(comp, ch.IsActive)
	commonconstraints.MustBeActivationColumns(comp, ch.RomOngoing)
	commonconstraints.MustBeActivationColumns(comp, ch.StateSumOngoing)

	commonconstraints.MustZeroWhenInactive(comp, ch.IsActive,
		ch.StateSumKeccak.Hi,
		ch.StateSumKeccak.Lo,
		ch.StateSumMiMC,
		ch.StateSumOngoing,
		ch.RomKeccak.Hi,
		ch.RomKeccak.Lo,
		ch.RomMiMC,
		ch.RomOngoing,
	)

	var (
		romDecreased                     ifaces.Column
		stateSumDecreased                ifaces.Column
		romLimbsHi, romLimbsLo           byte32cmp.LimbColumns
		stateSumLimbsHi, stateSumLimbsLo byte32cmp.LimbColumns
	)

	romLimbsHi, ch.CptRomKeccakLimbsHi = byte32cmp.Decompose(comp, ch.RomKeccak.Hi, 8, 16)
	romLimbsLo, ch.CptRomKeccakLimbsLo = byte32cmp.Decompose(comp, ch.RomKeccak.Lo, 8, 16)
	stateSumLimbsHi, ch.CptStateSumKeccakLimbsHi = byte32cmp.Decompose(comp, ch.StateSumKeccak.Hi, 8, 16)
	stateSumLimbsLo, ch.CptStateSumKeccakLimbsLo = byte32cmp.Decompose(comp, ch.StateSumKeccak.Lo, 8, 16)
	ch.RomKeccakLimbs = byte32cmp.FuseLimbs(romLimbsLo, romLimbsHi)
	ch.StateSumKeccakLimbs = byte32cmp.FuseLimbs(stateSumLimbsLo, stateSumLimbsHi)

	ch.RomIncreased, ch.RomIsConst, romDecreased, ch.CmpRomLimbs = byte32cmp.CmpMultiLimbs(
		comp,
		ch.RomKeccakLimbs,
		ch.RomKeccakLimbs.Shift(-1),
	)

	ch.StateSumIncreased, ch.StateSumIsConst, stateSumDecreased, ch.CmpStateSumLimbs = byte32cmp.CmpMultiLimbs(
		comp,
		ch.StateSumKeccakLimbs,
		ch.StateSumKeccakLimbs.Shift(-1),
	)

	ch.StateSumIsGtRom, ch.StateSumIsEqRom, ch.StateSumIsLtRom, ch.CmpRomVsStateSumLimbs = byte32cmp.CmpMultiLimbs(
		comp,
		ch.StateSumKeccakLimbs,
		ch.RomKeccakLimbs,
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_ROM_IS_SORTED"),
		sym.Mul(ch.RomOngoing, romDecreased),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_STATE_SUMMARY_IS_SORTED"),
		sym.Mul(ch.StateSumOngoing, stateSumDecreased),
	)

	// This constraint ensures that the state summary cursor. Is correctly
	// updated. It follows the following rules:
	//
	// NB: Since we have sorting constraints. We know that ROM and SS may
	// only increase or stay constant. Therefore, enforcing the constant
	//
	// 	switch {
	// 	case IsSSOngoing == 0:
	// 		// No constraints applied
	// 	case IsRomOnGoing == 0:
	// 		assert ssMustBeConstant == 0
	// 	case ss > rom:
	// 		assert ssMustBeConstant == 1
	// 	else:
	// 		assert ssMustBeConstant == 0
	// 	}
	//
	// The reciproqual constraint is enforced over the ROM module.
	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_STATE_SUM_STAY_SAME"),
		sym.Mul(
			column.Shift(ch.StateSumOngoing, 1),
			sym.Sub(
				column.Shift(ch.StateSumIsConst, 1),
				sym.Mul(
					ch.RomOngoing,
					sym.Add(
						ch.StateSumIsGtRom,
					),
				),
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_ROM_STAY_SAME"),
		sym.Mul(
			column.Shift(ch.RomOngoing, 1),
			sym.Sub(
				column.Shift(ch.RomIsConst, 1),
				sym.Mul(
					ch.StateSumOngoing,
					sym.Add(
						ch.StateSumIsLtRom,
					),
				),
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_MIMC_CONSISTENCY"),
		sym.Mul(
			ch.StateSumIsEqRom,
			sym.Sub(ch.RomMiMC, ch.StateSumMiMC),
		),
	)

	comp.GenericFragmentedConditionalInclusion(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_BACK"),
		[][]ifaces.Column{
			{
				ss.Account.Initial.MiMCCodeHash,
				ss.Account.Initial.KeccakCodeHash.Hi,
				ss.Account.Initial.KeccakCodeHash.Lo,
			},
			{
				ss.Account.Final.MiMCCodeHash,
				ss.Account.Final.KeccakCodeHash.Hi,
				ss.Account.Final.KeccakCodeHash.Lo,
			},
		},
		[]ifaces.Column{
			ch.StateSumMiMC,
			ch.StateSumKeccak.Hi,
			ch.StateSumKeccak.Lo,
		},
		[]ifaces.Column{
			ss.Account.Initial.Exists,
			ss.Account.Final.Exists,
		},
		ch.StateSumOngoing,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_FORTH_INITIAL"),
		[]ifaces.Column{
			ch.StateSumMiMC,
			ch.StateSumKeccak.Hi,
			ch.StateSumKeccak.Lo,
		},
		[]ifaces.Column{
			ss.Account.Initial.MiMCCodeHash,
			ss.Account.Initial.KeccakCodeHash.Hi,
			ss.Account.Initial.KeccakCodeHash.Lo,
		},
		ch.StateSumOngoing,
		ss.Account.Initial.Exists,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_FORTH_FINAL"),
		[]ifaces.Column{
			ch.StateSumMiMC,
			ch.StateSumKeccak.Hi,
			ch.StateSumKeccak.Lo,
		},
		[]ifaces.Column{
			ss.Account.Final.MiMCCodeHash,
			ss.Account.Final.KeccakCodeHash.Hi,
			ss.Account.Final.KeccakCodeHash.Lo,
		},
		ch.StateSumOngoing,
		ss.Account.Final.Exists,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_MIMC_CODE_HASH_FORTH"),
		[]ifaces.Column{
			ch.RomMiMC,
			ch.RomKeccak.Hi,
			ch.RomKeccak.Lo,
		},
		[]ifaces.Column{
			mch.NewState,
			mch.CodeHashHi,
			mch.CodeHashLo,
		},
		ch.RomOngoing,
		mch.IsForConsistency,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_MIMC_CODE_HASH_BACK"),
		[]ifaces.Column{
			mch.NewState,
			mch.CodeHashHi,
			mch.CodeHashLo,
		},
		[]ifaces.Column{
			ch.RomMiMC,
			ch.RomKeccak.Hi,
			ch.RomKeccak.Lo,
		},
		mch.IsForConsistency,
		ch.RomOngoing,
	)

	return ch
}
