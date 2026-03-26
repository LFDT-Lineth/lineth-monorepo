// codehashconsistency implements the necessary constraints to enforce consistency
// between the mimccodehash module and the statesummary module. The constraints
// generated in this package essentially aim at ensuring that
package codehashconsistency

import (
	"slices"

	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/statemanager/lineacodehash"
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
	StateSummaryModule  *statesummary.Module
	LineaCodeHashModule *lineacodehash.Module

	IsActive          ifaces.Column
	StateSumKeccak    common.HiLoColumns
	StateSumLineaHash [poseidon2.BlockSize]ifaces.Column // Poseidon2 code hash
	StateSumOngoing   ifaces.Column
	RomKeccak         common.HiLoColumns
	RomLineaHash      [poseidon2.BlockSize]ifaces.Column // Poseidon2 code hash
	RomOngoing        ifaces.Column

	StateSumKeccakLimbs byte32cmp.LimbColumns
	RomKeccakLimbs      byte32cmp.LimbColumns

	CptStateSumKeccakLimbsHi [common.NbLimbU128]wizard.ProverAction
	CptStateSumKeccakLimbsLo [common.NbLimbU128]wizard.ProverAction
	CptRomKeccakLimbsHi      [common.NbLimbU128]wizard.ProverAction
	CptRomKeccakLimbsLo      [common.NbLimbU128]wizard.ProverAction
	CmpStateSumLimbs         wizard.ProverAction
	CmpRomLimbs              wizard.ProverAction
	CmpRomVsStateSumLimbs    wizard.ProverAction

	StateSumIsGtRom, StateSumIsEqRom, StateSumIsLtRom ifaces.Column
	StateSumIsConst, StateSumIncreased                ifaces.Column
	RomIsConst, RomIncreased                          ifaces.Column
}

// NewModule returns a constrained [Module] connecting `ss` with `mch`. `name`
// is used as a prefix for the name of all the generated columns and constraints.
func NewModule(comp *wizard.CompiledIOP, name string, ss *statesummary.Module, chm *lineacodehash.Module) Module {

	name = name + "_CODEHASH_CONSISTENCY"
	size := 1 << 16

	ch := Module{
		StateSummaryModule:  ss,
		LineaCodeHashModule: chm,
		IsActive:            comp.InsertCommit(0, ifaces.ColID(name+"_IS_ACTIVE"), size, true),
		StateSumKeccak:      common.NewHiLoColumns(comp, size, name+"_STATE_SUMMARY_KECCAK"),
		RomKeccak:           common.NewHiLoColumns(comp, size, name+"_ROM_KECCAK"),
		RomOngoing:          comp.InsertCommit(0, ifaces.ColID(name+"_ROM_ONGOING"), size, true),
		StateSumOngoing:     comp.InsertCommit(0, ifaces.ColID(name+"_STATE_SUM_ONGOING"), size, true),
	}

	for i := range poseidon2.BlockSize {
		ch.StateSumLineaHash[i] = comp.InsertCommit(0, ifaces.ColIDf("%v_STATE_SUMMARY_POSEIDON_%v", name, i), size, true)
		ch.RomLineaHash[i] = comp.InsertCommit(0, ifaces.ColIDf("%v_ROM_POSEIDON_%v", name, i), size, true)
	}

	commonconstraints.MustBeActivationColumns(comp, ch.IsActive)
	commonconstraints.MustBeActivationColumns(comp, ch.RomOngoing)
	commonconstraints.MustBeActivationColumns(comp, ch.StateSumOngoing)

	chCols := []ifaces.Column{
		ch.StateSumOngoing,
		ch.RomOngoing,
	}
	chCols = append(chCols, ch.StateSumKeccak.Hi[:]...)
	chCols = append(chCols, ch.StateSumKeccak.Lo[:]...)
	chCols = append(chCols, ch.StateSumLineaHash[:]...)
	chCols = append(chCols, ch.RomKeccak.Hi[:]...)
	chCols = append(chCols, ch.RomKeccak.Lo[:]...)
	chCols = append(chCols, ch.RomLineaHash[:]...)

	commonconstraints.MustZeroWhenInactive(comp, ch.IsActive, chCols...)

	var (
		romDecreased      ifaces.Column
		stateSumDecreased ifaces.Column
		romLimbsHi        = byte32cmp.LimbColumns{LimbBitSize: 16, IsBigEndian: false}
		romLimbsLo        = byte32cmp.LimbColumns{LimbBitSize: 16, IsBigEndian: false}
		stateSumLimbsHi   = byte32cmp.LimbColumns{LimbBitSize: 16, IsBigEndian: false}
		stateSumLimbsLo   = byte32cmp.LimbColumns{LimbBitSize: 16, IsBigEndian: false}
	)

	romLimbsLo.Limbs = make([]ifaces.Column, common.NbLimbU128)
	romLimbsHi.Limbs = make([]ifaces.Column, common.NbLimbU128)
	stateSumLimbsHi.Limbs = make([]ifaces.Column, common.NbLimbU128)
	stateSumLimbsLo.Limbs = make([]ifaces.Column, common.NbLimbU128)
	for i := range common.NbLimbU128 {
		ind := common.NbLimbU128 - 1 - i

		romLimbHi, cptRomKeccakLimbsHi := byte32cmp.Decompose(comp, ch.RomKeccak.Hi[i], 1, 16)
		romLimbsHi.Limbs[ind] = romLimbHi.Limbs[0]
		ch.CptRomKeccakLimbsHi[i] = cptRomKeccakLimbsHi

		romLimbLo, cptRomKeccakLimbsLo := byte32cmp.Decompose(comp, ch.RomKeccak.Lo[i], 1, 16)
		romLimbsLo.Limbs[ind] = romLimbLo.Limbs[0]
		ch.CptRomKeccakLimbsLo[i] = cptRomKeccakLimbsLo

		stateSumLimbHi, cptStateSumKeccakLimbsHi := byte32cmp.Decompose(comp, ch.StateSumKeccak.Hi[i], 1, 16)
		stateSumLimbsHi.Limbs[ind] = stateSumLimbHi.Limbs[0]
		ch.CptStateSumKeccakLimbsHi[i] = cptStateSumKeccakLimbsHi

		stateSumLimbLo, cptStateSumKeccakLimbsLo := byte32cmp.Decompose(comp, ch.StateSumKeccak.Lo[i], 1, 16)
		stateSumLimbsLo.Limbs[ind] = stateSumLimbLo.Limbs[0]
		ch.CptStateSumKeccakLimbsLo[i] = cptStateSumKeccakLimbsLo
	}

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

	// The reciproqual constraint is enforced over the ROM module.
	comp.InsertGlobal(
		0,
		ifaces.QueryID(name+"_STATE_SUM_STAY_SAME"),
		sym.Mul(
			column.Shift(ch.StateSumOngoing, 1),
			ch.StateSumLineaHash[0], // For delegation account, the state-summary may be zero and that's ok
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
			ch.StateSumKeccakLimbs.Limbs[0],
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

	for i := range poseidon2.BlockSize {
		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("%s_LINEA_CODE_HASH_CONSISTENCY_%d", name, i),
			sym.Mul(
				ch.StateSumIsEqRom,
				sym.Sub(ch.RomLineaHash[i], ch.StateSumLineaHash[i]),
			),
		)
	}

	var (
		accInitialHashColumns = slices.Concat(
			ss.Account.Initial.LineaCodeHash[:],
			ss.Account.Initial.KeccakCodeHash.Hi[:],
			ss.Account.Initial.KeccakCodeHash.Lo[:],
		)

		accFinalHashColumns = slices.Concat(
			ss.Account.Final.LineaCodeHash[:],
			ss.Account.Final.KeccakCodeHash.Hi[:],
			ss.Account.Final.KeccakCodeHash.Lo[:],
		)

		stateSumHashColumns = slices.Concat(
			ch.StateSumLineaHash[:],
			ch.StateSumKeccak.Hi[:],
			ch.StateSumKeccak.Lo[:],
		)
	)

	comp.GenericFragmentedConditionalInclusion(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_BACK"),
		[][]ifaces.Column{
			accInitialHashColumns,
			accFinalHashColumns,
		},
		stateSumHashColumns,
		[]ifaces.Column{
			ss.Account.Initial.Exists,
			ss.Account.Final.Exists,
		},
		ch.StateSumOngoing,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_FORTH_INITIAL"),
		stateSumHashColumns,
		accInitialHashColumns,
		ch.StateSumOngoing,
		ss.Account.Initial.Exists,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_STATE_SUMMARY_FORTH_FINAL"),
		stateSumHashColumns,
		accFinalHashColumns,
		ch.StateSumOngoing,
		ss.Account.Final.Exists,
	)

	var chHashColumns []ifaces.Column
	chHashColumns = append(chHashColumns, ch.RomLineaHash[:]...)
	chHashColumns = append(chHashColumns, ch.RomKeccak.Hi[:]...)
	chHashColumns = append(chHashColumns, ch.RomKeccak.Lo[:]...)

	var mchHashColumns []ifaces.Column
	mchHashColumns = append(mchHashColumns, chm.NewState[:]...)
	mchHashColumns = append(mchHashColumns, chm.CodeHash[:]...)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_POSEIDON_CODE_HASH_FORTH"),
		chHashColumns,
		mchHashColumns,
		ch.RomOngoing,
		chm.IsForConsistency,
	)

	comp.InsertInclusionDoubleConditional(
		0,
		ifaces.QueryID(name+"_IMPORT_POSEIDON_CODE_HASH_BACK"),
		mchHashColumns,
		chHashColumns,
		chm.IsForConsistency,
		ch.RomOngoing,
	)

	return ch
}
