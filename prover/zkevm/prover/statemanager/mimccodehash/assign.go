package mimccodehash

import (
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type assignBuilder struct {
	isActive         []field.Element
	cfi              []field.Element
	limb             []field.Element
	codeHashHi       []field.Element
	codeHashLo       []field.Element
	codeSize         []field.Element
	isNewHash        []field.Element
	isHashEnd        []field.Element
	prevState        []field.Element
	newState         []field.Element
	isNonEmptyKeccak []field.Element
}

func newAssignmentBuilder(length int) *assignBuilder {
	return &assignBuilder{
		isActive:         make([]field.Element, 0, length),
		cfi:              make([]field.Element, 0, length),
		limb:             make([]field.Element, 0, length),
		codeHashHi:       make([]field.Element, 0, length),
		codeHashLo:       make([]field.Element, 0, length),
		codeSize:         make([]field.Element, 0, length),
		isNewHash:        make([]field.Element, 0, length),
		isHashEnd:        make([]field.Element, 0, length),
		prevState:        make([]field.Element, 0, length),
		newState:         make([]field.Element, 0, length),
		isNonEmptyKeccak: make([]field.Element, 0, length),
	}
}

// Assign function assigns columns of the MiMCCodeHash module
func (mh *Module) Assign(run *wizard.ProverRuntime) {

	if mh.inputModules == nil {
		utils.Panic("Module.ConnectToRom has not been run")
	}

	var (
		rom    = mh.inputModules.RomInput
		romLex = mh.inputModules.RomLexInput
	)

	if !run.Columns.Exists(rom.CounterIsEqualToNBytesMinusOne.GetColID()) {
		rom.completeAssign(run)
	}

	var (
		cfi        = rom.CFI.GetColAssignment(run).IntoRegVecSaveAlloc()
		acc        = rom.Acc.GetColAssignment(run).IntoRegVecSaveAlloc()
		cfiRomLex  = romLex.CFIRomLex.GetColAssignment(run).IntoRegVecSaveAlloc()
		codeHashHi = romLex.CodeHashHi.GetColAssignment(run).IntoRegVecSaveAlloc()
		codeHashLo = romLex.CodeHashLo.GetColAssignment(run).IntoRegVecSaveAlloc()
		codeSize   = rom.CodeSize.GetColAssignment(run).IntoRegVecSaveAlloc()
		filter     = rom.CounterIsEqualToNBytesMinusOne.GetColAssignment(run).IntoRegVecSaveAlloc()
		length     = len(cfi)
		builder    = newAssignmentBuilder(length)
	)

	for i := 0; i < length; i++ {

		if !cfi[i].IsZero() && ((i+1 == length) || cfi[i+1].IsZero()) {
			// This is the last row in the active area of the rom input.
			// We assign one more row to make the assignment of the last row
			// for other columns below work correctly, we exclude codeHash and
			// assign it below from the romLex input.
			builder.isActive = append(builder.isActive, field.Zero())
			builder.cfi = append(builder.cfi, field.Zero())
			builder.limb = append(builder.limb, field.Zero())
			builder.codeSize = append(builder.codeSize, field.Zero())
			break
		}

		if filter[i].IsZero() {
			continue
		}

		// Append 1 to isActive column
		builder.isActive = append(builder.isActive, field.One())

		// Inject the other incoming columns from the rom input
		builder.cfi = append(builder.cfi, cfi[i])
		builder.limb = append(builder.limb, acc[i])
		builder.codeSize = append(builder.codeSize, codeSize[i])
	}

	// The content of this statement is constructing isNewHash and isHashEnd
	// prevState and newState. However, it is only needed when there is any
	// codehash to hash in the first place.
	if len(builder.cfi) > 0 {

		// Initialize the first row of the remaining columns
		builder.isNewHash = append(builder.isNewHash, field.One())

		// Detect if it is a one limb segment (at the begining) and assign IsHashEnd accordingly
		if builder.cfi[1] != builder.cfi[0] {
			builder.isHashEnd = append(builder.isHashEnd, field.One())
		} else {
			builder.isHashEnd = append(builder.isHashEnd, field.Zero())
		}

		builder.prevState = append(builder.prevState, field.Zero())
		builder.newState = append(builder.newState, mimc.BlockCompression(builder.prevState[0], builder.limb[0]))

		// Assign other rows of the remaining columns
		for i := 1; i < len(builder.cfi); i++ {

			// We do not need to continue if we are in the inactive area
			if builder.isActive[i].IsZero() {
				break
			}

			var (
				cfiPrev          = builder.cfi[i-1]
				cfiCurr          = builder.cfi[i]
				cfiNext          = builder.cfi[i+1]
				isSegmentBegin   = false
				isSegmentMiddle  = false
				isSegmentEnd     = false
				isOneLimbSegment = false
			)

			if cfiPrev.Equal(&cfiCurr) && cfiCurr.Equal(&cfiNext) {
				isSegmentMiddle = true
			}

			if !cfiPrev.Equal(&cfiCurr) && cfiCurr.Equal(&cfiNext) {
				isSegmentBegin = true
			}

			if cfiPrev.Equal(&cfiCurr) && !cfiCurr.Equal((&cfiNext)) {
				isSegmentEnd = true
			}

			if !cfiPrev.Equal(&cfiCurr) && !cfiCurr.Equal((&cfiNext)) {
				isOneLimbSegment = true
			}

			// Assign for begining of a segment
			if isSegmentBegin {
				builder.isNewHash = append(builder.isNewHash, field.One())
				builder.isHashEnd = append(builder.isHashEnd, field.Zero())
				builder.prevState = append(builder.prevState, field.Zero())
				builder.newState = append(builder.newState, mimc.BlockCompression(builder.prevState[i], builder.limb[i]))
				continue
			}

			// Assign for middle of a segment
			if isSegmentMiddle {
				builder.isNewHash = append(builder.isNewHash, field.Zero())
				builder.isHashEnd = append(builder.isHashEnd, field.Zero())
				builder.prevState = append(builder.prevState, builder.newState[i-1])
				builder.newState = append(builder.newState, mimc.BlockCompression(builder.prevState[i], builder.limb[i]))
				continue
			}

			// Assign for end of a segment
			if isSegmentEnd {
				builder.isNewHash = append(builder.isNewHash, field.Zero())
				builder.isHashEnd = append(builder.isHashEnd, field.One())
				builder.prevState = append(builder.prevState, builder.newState[i-1])
				builder.newState = append(builder.newState, mimc.BlockCompression(builder.prevState[i], builder.limb[i]))
				continue
			}

			// Assign for a one limb segment
			if isOneLimbSegment {
				builder.isNewHash = append(builder.isNewHash, field.One())
				builder.isHashEnd = append(builder.isHashEnd, field.One())
				builder.prevState = append(builder.prevState, field.Zero())
				builder.newState = append(builder.newState, mimc.BlockCompression(builder.prevState[i], builder.limb[i]))
				continue
			}
		}

		// Assign codehash from the romLex input
		for i := 0; i < len(builder.cfi); i++ {

			// We do not need to continue if we are in the inactive area
			if builder.isActive[i].IsZero() {
				break
			}

			currCFI := builder.cfi[i]

			// For each currCFI, we look over all the CFIs in the Romlex input,
			// and append only that codehash for which the cfi matches with currCFI
			for j := 0; j < len(cfiRomLex); j++ {
				if currCFI == cfiRomLex[j] {

					currIsNonEmptyKeccak := field.One()

					if builder.isHashEnd[i].IsZero() {
						currIsNonEmptyKeccak = field.Zero()
					}

					if codeHashHi[j] == emptyKeccakHi && codeHashLo[j] == emptyKeccakLo {
						currIsNonEmptyKeccak = field.Zero()
					}

					builder.isNonEmptyKeccak = append(builder.isNonEmptyKeccak, currIsNonEmptyKeccak)
					builder.codeHashHi = append(builder.codeHashHi, codeHashHi[j])
					builder.codeHashLo = append(builder.codeHashLo, codeHashLo[j])
					break
				}
				continue
			}
		}
	}

	// Assign the columns of the mimc code hash module
	run.AssignColumn(mh.IsActive.GetColID(), smartvectors.RightZeroPadded(builder.isActive, mh.inputs.Size))
	run.AssignColumn(mh.CFI.GetColID(), smartvectors.RightZeroPadded(builder.cfi, mh.inputs.Size))
	run.AssignColumn(mh.Limb.GetColID(), smartvectors.RightZeroPadded(builder.limb, mh.inputs.Size))
	run.AssignColumn(mh.CodeHashHi.GetColID(), smartvectors.RightZeroPadded(builder.codeHashHi, mh.inputs.Size))
	run.AssignColumn(mh.CodeHashLo.GetColID(), smartvectors.RightZeroPadded(builder.codeHashLo, mh.inputs.Size))
	run.AssignColumn(mh.CodeSize.GetColID(), smartvectors.RightZeroPadded(builder.codeSize, mh.inputs.Size))
	run.AssignColumn(mh.IsNewHash.GetColID(), smartvectors.RightZeroPadded(builder.isNewHash, mh.inputs.Size))
	run.AssignColumn(mh.IsHashEnd.GetColID(), smartvectors.RightZeroPadded(builder.isHashEnd, mh.inputs.Size))
	run.AssignColumn(mh.PrevState.GetColID(), smartvectors.RightZeroPadded(builder.prevState, mh.inputs.Size))
	run.AssignColumn(mh.IsForConsistency.GetColID(), smartvectors.RightZeroPadded(builder.isNonEmptyKeccak, mh.inputs.Size))

	// Assignment of new state with the zero hash padding
	newStatePad := mimc.BlockCompression(field.Zero(), field.Zero())
	run.AssignColumn(mh.NewState.GetColID(), smartvectors.RightPadded(builder.newState, newStatePad, mh.inputs.Size))

	mh.CptIsEmptyKeccakHi.Run(run)
	mh.CptIsEmptyKeccakLo.Run(run)
}
