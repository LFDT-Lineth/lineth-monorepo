package codehashconsistency

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// Assign assigns the columns internally defined in `mod` into `run`.
func (mod Module) Assign(run *wizard.ProverRuntime) {

	var (
		ssInput  = mod.StateSummaryInput
		mchInput = mod.MimcCodeHashInput
	)

	externalSs := struct {
		IsActive      smartvectors.SmartVector
		IsStorage     smartvectors.SmartVector
		InitMiMC      smartvectors.SmartVector
		InitKeccakLo  smartvectors.SmartVector
		InitKeccakHi  smartvectors.SmartVector
		FinalMiMC     smartvectors.SmartVector
		FinalKeccakLo smartvectors.SmartVector
		FinalKeccakHi smartvectors.SmartVector
	}{
		IsActive:      ssInput.IsActive.GetColAssignment(run),
		IsStorage:     ssInput.IsStorage.GetColAssignment(run),
		InitMiMC:      ssInput.Account.Initial.MiMCCodeHash.GetColAssignment(run),
		InitKeccakHi:  ssInput.Account.Initial.KeccakCodeHash.Hi.GetColAssignment(run),
		InitKeccakLo:  ssInput.Account.Initial.KeccakCodeHash.Lo.GetColAssignment(run),
		FinalMiMC:     ssInput.Account.Final.MiMCCodeHash.GetColAssignment(run),
		FinalKeccakHi: ssInput.Account.Final.KeccakCodeHash.Hi.GetColAssignment(run),
		FinalKeccakLo: ssInput.Account.Final.KeccakCodeHash.Lo.GetColAssignment(run),
	}

	externalRom := struct {
		IsActive   smartvectors.SmartVector
		IsHashEnd  smartvectors.SmartVector
		NewState   smartvectors.SmartVector
		CodeHashHi smartvectors.SmartVector
		CodeHashLo smartvectors.SmartVector
	}{
		IsActive:   mchInput.IsActive.GetColAssignment(run),
		IsHashEnd:  mchInput.IsHashEnd.GetColAssignment(run),
		NewState:   mchInput.NewState.GetColAssignment(run),
		CodeHashHi: mchInput.CodeHashHi.GetColAssignment(run),
		CodeHashLo: mchInput.CodeHashLo.GetColAssignment(run),
	}

	var (
		ssData  = make([][3]field.Element, 0, 2*externalSs.InitMiMC.Len())
		romData = make([][3]field.Element, 0, externalRom.IsHashEnd.Len())
	)

	for i := 0; i < externalSs.InitMiMC.Len(); i++ {

		if isActive := externalSs.IsActive.Get(i); isActive.IsZero() {
			break
		}

		if isStorage := externalSs.IsStorage.Get(i); isStorage.IsOne() {
			continue
		}

		ssData = append(ssData,
			[3]field.Element{
				externalSs.InitMiMC.Get(i),
				externalSs.InitKeccakHi.Get(i),
				externalSs.InitKeccakLo.Get(i),
			},
			[3]field.Element{
				externalSs.FinalMiMC.Get(i),
				externalSs.FinalKeccakHi.Get(i),
				externalSs.FinalKeccakLo.Get(i),
			},
		)
	}

	for i := 0; i < externalRom.NewState.Len(); i++ {

		if isActive := externalRom.IsActive.Get(i); isActive.IsZero() {
			break
		}

		if isHashEnd := externalRom.IsHashEnd.Get(i); isHashEnd.IsZero() {
			continue
		}

		romData = append(romData,
			[3]field.Element{
				externalRom.NewState.Get(i),
				externalRom.CodeHashHi.Get(i),
				externalRom.CodeHashLo.Get(i),
			},
		)
	}

	cmp := func(a, b [3]field.Element) int {
		if res := a[1].Cmp(&b[1]); res != 0 {
			return res
		}
		return a[2].Cmp(&b[2])
	}

	slices.SortFunc(ssData, cmp)
	slices.SortFunc(romData, cmp)
	ssData = slices.Compact(ssData)
	romData = slices.Compact(romData)
	ssData = slices.Clip(ssData)
	romData = slices.Clip(romData)

	assignment := struct {
		IsActive        *common.VectorBuilder
		StateSumKeccak  common.HiLoAssignmentBuilder
		StateSumMiMC    *common.VectorBuilder
		RomKeccak       common.HiLoAssignmentBuilder
		RomMiMC         *common.VectorBuilder
		RomOngoing      *common.VectorBuilder
		StateSumOngoing *common.VectorBuilder
	}{
		IsActive:        common.NewVectorBuilder(mod.IsActive),
		StateSumKeccak:  common.NewHiLoAssignmentBuilder(mod.StateSumKeccak),
		RomKeccak:       common.NewHiLoAssignmentBuilder(mod.RomKeccak),
		StateSumMiMC:    common.NewVectorBuilder(mod.StateSumMiMC),
		RomMiMC:         common.NewVectorBuilder(mod.RomMiMC),
		RomOngoing:      common.NewVectorBuilder(mod.RomOngoing),
		StateSumOngoing: common.NewVectorBuilder(mod.StateSumOngoing),
	}

	var (
		cRom     int = 0
		cSS      int = 0
		nbRowMax     = len(romData) + len(ssData)
	)

assign_loop:
	for i := 0; i < nbRowMax; i++ {

		var (
			romRow   = romData[cRom]
			ssRow    = ssData[cSS]
			romCmpSs = cmp(romRow, ssRow)
		)

		assignment.IsActive.PushOne()
		assignment.RomMiMC.PushField(romRow[0])
		assignment.RomKeccak.Hi.PushField(romRow[1])
		assignment.RomKeccak.Lo.PushField(romRow[2])
		assignment.RomOngoing.PushBoolean(cRom < len(romData)-1)
		assignment.StateSumMiMC.PushField(ssRow[0])
		assignment.StateSumKeccak.Hi.PushField(ssRow[1])
		assignment.StateSumKeccak.Lo.PushField(ssRow[2])
		assignment.StateSumOngoing.PushBoolean(cSS < len(ssData)-1)

		var (
			isLastSS  = cSS >= len(ssData)-1
			isLastRom = cRom >= len(romData)-1
		)

		switch {
		case isLastSS && isLastRom:
			break assign_loop
		case !isLastSS && isLastRom:
			cSS++
		case isLastSS && !isLastRom:
			cRom++
		case romCmpSs < 0:
			cRom++
		case romCmpSs == 0:
			cRom++
			cSS++
		case romCmpSs > 0:
			cSS++
		}
	}

	assignment.IsActive.PadAndAssign(run, field.Zero())
	assignment.RomMiMC.PadAndAssign(run, field.Zero())
	assignment.RomKeccak.Hi.PadAndAssign(run, field.Zero())
	assignment.RomKeccak.Lo.PadAndAssign(run, field.Zero())
	assignment.RomOngoing.PadAndAssign(run, field.Zero())
	assignment.StateSumMiMC.PadAndAssign(run, field.Zero())
	assignment.StateSumKeccak.Hi.PadAndAssign(run, field.Zero())
	assignment.StateSumKeccak.Lo.PadAndAssign(run, field.Zero())
	assignment.StateSumOngoing.PadAndAssign(run, field.Zero())

	mod.CptStateSumKeccakLimbsHi.Run(run)
	mod.CptStateSumKeccakLimbsLo.Run(run)
	mod.CptRomKeccakLimbsHi.Run(run)
	mod.CptRomKeccakLimbsLo.Run(run)
	mod.CmpStateSumLimbs.Run(run)
	mod.CmpRomLimbs.Run(run)
	mod.CmpRomVsStateSumLimbs.Run(run)
}
