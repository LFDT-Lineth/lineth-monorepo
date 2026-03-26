package codehashconsistency

import (
	"slices"

	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// Assign assigns the columns internally defined in `mod` into `run`.
func (mod Module) Assign(run *wizard.ProverRuntime) {

	var (
		ssInput        = mod.StateSummaryModule
		lineahashInput = mod.LineaCodeHashModule
	)

	externalSs := struct {
		InitAccountExists  smartvectors.SmartVector
		FinalAccountExists smartvectors.SmartVector
		IsStorage          smartvectors.SmartVector
		InitLineaHash      [poseidon2.BlockSize]smartvectors.SmartVector
		InitKeccakLo       [common.NbLimbU128]smartvectors.SmartVector
		InitKeccakHi       [common.NbLimbU128]smartvectors.SmartVector
		FinalLineaHash     [poseidon2.BlockSize]smartvectors.SmartVector
		FinalKeccakLo      [common.NbLimbU128]smartvectors.SmartVector
		FinalKeccakHi      [common.NbLimbU128]smartvectors.SmartVector
	}{
		InitAccountExists:  ssInput.Account.Initial.Exists.GetColAssignment(run),
		FinalAccountExists: ssInput.Account.Final.Exists.GetColAssignment(run),
		IsStorage:          ssInput.IsStorage.GetColAssignment(run),
	}

	for i := range poseidon2.BlockSize {
		externalSs.InitLineaHash[i] = ssInput.Account.Initial.LineaCodeHash[i].GetColAssignment(run)
		externalSs.FinalLineaHash[i] = ssInput.Account.Final.LineaCodeHash[i].GetColAssignment(run)
	}

	for i := range common.NbLimbU128 {
		externalSs.InitKeccakHi[i] = ssInput.Account.Initial.KeccakCodeHash.Hi[i].GetColAssignment(run)
		externalSs.InitKeccakLo[i] = ssInput.Account.Initial.KeccakCodeHash.Lo[i].GetColAssignment(run)
		externalSs.FinalKeccakHi[i] = ssInput.Account.Final.KeccakCodeHash.Hi[i].GetColAssignment(run)
		externalSs.FinalKeccakLo[i] = ssInput.Account.Final.KeccakCodeHash.Lo[i].GetColAssignment(run)
	}

	externalRom := struct {
		IsActive         smartvectors.SmartVector
		IsForConsistency smartvectors.SmartVector
		NewState         [poseidon2.BlockSize]smartvectors.SmartVector // Poseidon2 code hash
		CodeHash         [common.NbLimbU256]smartvectors.SmartVector   // Keccak code hash
	}{
		IsActive:         lineahashInput.IsActive.GetColAssignment(run),
		IsForConsistency: lineahashInput.IsForConsistency.GetColAssignment(run),
	}

	for i := range poseidon2.BlockSize {
		externalRom.NewState[i] = lineahashInput.NewState[i].GetColAssignment(run)
	}

	for i := range common.NbLimbU256 {
		externalRom.CodeHash[i] = lineahashInput.CodeHash[i].GetColAssignment(run)
	}

	// rowData holds a Poseidon2 hash (8 elements) and a Keccak hash (16 elements)
	type rowData struct {
		poseidon2Hash [poseidon2.BlockSize]field.Element
		keccakHash    [common.NbLimbU256]field.Element
	}

	var (
		ssData  = make([]rowData, 0, 2*externalSs.InitLineaHash[0].Len()) // state summary data, size is 2* the number of rows in the state summary because there are 2 accounts (initial and final)
		romData = make([]rowData, 0, externalRom.IsForConsistency.Len())  // rom data
	)

	for i := 0; i < externalSs.InitLineaHash[0].Len(); i++ {

		if isStorage := externalSs.IsStorage.Get(i); isStorage.IsOne() {
			continue
		}

		var (
			initAccountExists  = externalSs.InitAccountExists.Get(i)
			finalAccountExists = externalSs.FinalAccountExists.Get(i)
		)

		if initAccountExists.IsOne() {
			var initLineaHash [poseidon2.BlockSize]field.Element
			var initKeccak [common.NbLimbU256]field.Element
			for j := range poseidon2.BlockSize {
				initLineaHash[j] = externalSs.InitLineaHash[j].Get(i)
			}

			for j := range common.NbLimbU128 {
				initKeccak[j] = externalSs.InitKeccakHi[j].Get(i)
				initKeccak[common.NbLimbU128+j] = externalSs.InitKeccakLo[j].Get(i)
			}

			ssData = append(ssData, rowData{
				poseidon2Hash: initLineaHash,
				keccakHash:    initKeccak,
			})
		}

		if finalAccountExists.IsOne() {
			var finalLineaHash [poseidon2.BlockSize]field.Element
			var finalKeccak [common.NbLimbU256]field.Element
			for j := range poseidon2.BlockSize {
				finalLineaHash[j] = externalSs.FinalLineaHash[j].Get(i)
			}

			for j := range common.NbLimbU128 {
				finalKeccak[j] = externalSs.FinalKeccakHi[j].Get(i)
				finalKeccak[common.NbLimbU128+j] = externalSs.FinalKeccakLo[j].Get(i)
			}

			ssData = append(ssData, rowData{
				poseidon2Hash: finalLineaHash,
				keccakHash:    finalKeccak,
			})
		}
	}

	for i := 0; i < externalRom.NewState[0].Len(); i++ {

		if isActive := externalRom.IsActive.Get(i); isActive.IsZero() {
			break
		}

		isForConsistency := externalRom.IsForConsistency.Get(i)
		isForConsistencyInt := isForConsistency.Uint64()
		isHashEnd := 1 - isForConsistencyInt

		if isHashEnd == 1 {
			continue
		}

		var newState [poseidon2.BlockSize]field.Element
		var codeHash [common.NbLimbU256]field.Element
		for j := range poseidon2.BlockSize {
			newState[j] = externalRom.NewState[j].Get(i)
		}
		for j := range common.NbLimbU256 {
			codeHash[j] = externalRom.CodeHash[j].Get(i)
		}

		romData = append(romData, rowData{
			poseidon2Hash: newState,
			keccakHash:    codeHash,
		})
	}

	cmp := func(a, b rowData) int {
		if res := CmpLimbs(a.keccakHash[:], b.keccakHash[:]); res != 0 {
			return res
		}
		return CmpLimbs(a.poseidon2Hash[:], b.poseidon2Hash[:])
	}

	slices.SortFunc(ssData, cmp)
	slices.SortFunc(romData, cmp)
	ssData = slices.Compact(ssData)
	romData = slices.Compact(romData)
	ssData = slices.Clip(ssData)
	romData = slices.Clip(romData)

	assignment := struct {
		IsActive          *common.VectorBuilder
		StateSumKeccak    common.HiLoAssignmentBuilder
		StateSumLineaHash [poseidon2.BlockSize]*common.VectorBuilder
		RomKeccak         common.HiLoAssignmentBuilder
		RomLineaHash      [poseidon2.BlockSize]*common.VectorBuilder
		RomOngoing        *common.VectorBuilder
		StateSumOngoing   *common.VectorBuilder
	}{
		IsActive:        common.NewVectorBuilder(mod.IsActive),
		StateSumKeccak:  common.NewHiLoAssignmentBuilder(mod.StateSumKeccak),
		RomKeccak:       common.NewHiLoAssignmentBuilder(mod.RomKeccak),
		RomOngoing:      common.NewVectorBuilder(mod.RomOngoing),
		StateSumOngoing: common.NewVectorBuilder(mod.StateSumOngoing),
	}

	for i := range poseidon2.BlockSize {
		assignment.RomLineaHash[i] = common.NewVectorBuilder(mod.RomLineaHash[i])
		assignment.StateSumLineaHash[i] = common.NewVectorBuilder(mod.StateSumLineaHash[i])
	}

	var (
		cRom     int = 0
		cSS      int = 0
		nbRowMax     = len(romData) + len(ssData)
	)

	for i := 0; i < nbRowMax; i++ {

		if cSS >= len(ssData) && cRom >= len(romData) {
			break
		}

		// importantly, we have to account for the fact that romData and/or ssData
		// can perfectly be empty slices. This can happen when a block is full of
		// eoa-transactions only. Therefore, we need to check that cRom and cSS
		// are within bounds. Otherwise, it will panic.
		var (
			romRow = rowData{}
			ssRow  = rowData{}
		)

		if cRom < len(romData) {
			romRow = romData[cRom]
		}

		if cSS < len(ssData) {
			ssRow = ssData[cSS]
		}

		romCmpSs := cmp(romRow, ssRow)

		assignment.IsActive.PushOne()

		for j := range poseidon2.BlockSize {
			assignment.RomLineaHash[j].PushField(romRow.poseidon2Hash[j])
			assignment.StateSumLineaHash[j].PushField(ssRow.poseidon2Hash[j])
		}

		for j := range common.NbLimbU128 {
			assignment.RomKeccak.Hi[j].PushField(romRow.keccakHash[j])
			assignment.RomKeccak.Lo[j].PushField(romRow.keccakHash[common.NbLimbU128+j])

			assignment.StateSumKeccak.Hi[j].PushField(ssRow.keccakHash[j])
			assignment.StateSumKeccak.Lo[j].PushField(ssRow.keccakHash[common.NbLimbU128+j])
		}

		assignment.RomOngoing.PushBoolean(cRom < len(romData))
		assignment.StateSumOngoing.PushBoolean(cSS < len(ssData))

		newCRom, newCSS := cRom, cSS

		if cRom < len(romData) && romCmpSs <= 0 {
			newCRom = cRom + 1
		}

		if cSS < len(ssData) && romCmpSs >= 0 {
			newCSS = cSS + 1
		}

		if cRom < len(romData) && newCSS >= len(ssData) {
			newCRom = cRom + 1
		}

		if cSS < len(ssData) && newCRom >= len(romData) {
			newCSS = cSS + 1
		}

		cRom, cSS = newCRom, newCSS
	}

	for i := range poseidon2.BlockSize {
		assignment.RomLineaHash[i].PadAndAssign(run, field.Zero())
		assignment.StateSumLineaHash[i].PadAndAssign(run, field.Zero())
	}

	for i := range common.NbLimbU128 {
		assignment.RomKeccak.Hi[i].PadAndAssign(run, field.Zero())
		assignment.RomKeccak.Lo[i].PadAndAssign(run, field.Zero())

		assignment.StateSumKeccak.Hi[i].PadAndAssign(run, field.Zero())
		assignment.StateSumKeccak.Lo[i].PadAndAssign(run, field.Zero())
	}

	assignment.IsActive.PadAndAssign(run, field.Zero())
	assignment.RomOngoing.PadAndAssign(run, field.Zero())
	assignment.StateSumOngoing.PadAndAssign(run, field.Zero())

	for i := range common.NbLimbU128 {
		mod.CptStateSumKeccakLimbsHi[i].Run(run)
		mod.CptStateSumKeccakLimbsLo[i].Run(run)
		mod.CptRomKeccakLimbsHi[i].Run(run)
		mod.CptRomKeccakLimbsLo[i].Run(run)
	}

	mod.CmpStateSumLimbs.Run(run)
	mod.CmpRomLimbs.Run(run)
	mod.CmpRomVsStateSumLimbs.Run(run)
}

// trimLeadingZeros removes any leading zero limbs from a number's representation.
// For example, [0, 0, 5, 10] becomes [5, 10], and [0, 0, 0] becomes [].
// This ensures that numbers like 0 are consistently represented (e.g., as []field.Element{}).
func trimLeadingZeros(limbs []field.Element) []field.Element {
	firstNonZeroIdx := -1
	for i := len(limbs) - 1; i >= 0; i-- {
		if !limbs[i].IsZero() {
			firstNonZeroIdx = i
			break
		}
	}

	if firstNonZeroIdx == -1 {
		return []field.Element{}
	}

	return limbs[:firstNonZeroIdx+1]
}

// CmpLimbs compares two numbers represented as slices of uint64 limbs.
// The most significant limb is expected to be at the highest index.
//
// It returns:
//
//	 0 if a == b
//	 1 if a > b
//	-1 if a < b
func CmpLimbs(a, b []field.Element) int {
	trimmedA := trimLeadingZeros(a)
	trimmedB := trimLeadingZeros(b)

	lenA := len(trimmedA)
	lenB := len(trimmedB)

	if lenA > lenB {
		return 1
	}
	if lenA < lenB {
		return -1
	}

	for i := 0; i < lenA; i++ {
		if trimmedA[i].Cmp(&trimmedB[i]) == 1 {
			return 1
		}
		if trimmedA[i].Cmp(&trimmedB[i]) == -1 {
			return -1
		}
	}

	return 0
}
