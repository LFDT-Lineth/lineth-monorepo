package lineacodehash

import (
	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

type assignBuilder struct {
	isActive         []field.Element
	cfi              [][common.NbLimbU32]field.Element
	limb             [common.NbLimbU128][]field.Element
	codeHash         [common.NbLimbU256][]field.Element
	codeSize         [common.NbLimbU32][]field.Element
	isNewHash        []field.Element
	isHashEnd        []field.Element
	prevState        [poseidon2.BlockSize][]field.Element
	newState         [poseidon2.BlockSize][]field.Element
	isNonEmptyKeccak []field.Element
}

func newAssignmentBuilder(length int) *assignBuilder {
	ab := &assignBuilder{
		isActive:         make([]field.Element, 0, length),
		isNewHash:        make([]field.Element, 0, length),
		isHashEnd:        make([]field.Element, 0, length),
		isNonEmptyKeccak: make([]field.Element, 0, length),
	}

	for i := range common.NbLimbU256 {
		ab.codeHash[i] = make([]field.Element, 0, length)
	}

	for i := range poseidon2.BlockSize {
		ab.prevState[i] = make([]field.Element, 0, length)
		ab.newState[i] = make([]field.Element, 0, length)
	}

	for i := range common.NbLimbU128 {
		ab.limb[i] = make([]field.Element, 0, length)
	}

	for i := range common.NbLimbU32 {
		ab.codeSize[i] = make([]field.Element, 0, length)
	}

	ab.cfi = make([][common.NbLimbU32]field.Element, 0, length)

	return ab
}

// Assign function assigns columns of the MiMCCodeHash module
func (mh *Module) Assign(run *wizard.ProverRuntime) {

	if mh.InputModules == nil {
		utils.Panic("Module.ConnectToRom has not been run")
	}

	var (
		rom    = mh.InputModules.RomInput
		romLex = mh.InputModules.RomLexInput
	)

	if !run.Columns.Exists(rom.CounterIsEqualToNBytesMinusOne.GetColID()) {
		rom.completeAssign(run)
	}

	var (
		filter    = rom.CounterIsEqualToNBytesMinusOne.GetColAssignment(run).IntoRegVecSaveAlloc()
		codeHash  = common.GetMultiColumnAssignment(run, romLex.CodeHash[:])
		acc       = common.GetMultiColumnAssignment(run, rom.Acc[:])
		codeSize  = common.GetMultiColumnAssignment(run, rom.CodeSize[:])
		cfi       = common.GetMultiColumnAssignment(run, rom.CFI[:])
		cfiRomLex = common.GetMultiColumnAssignment(run, romLex.CFIRomLex[:])
		// Since we need to operate on limb slices, we need to transpose limb columns.
		cfiTransposed       = transposeLimbs(cfi[:])
		cfiRomLexTransposed = transposeLimbs(cfiRomLex[:])
		length              = len(cfiTransposed)
		builder             = newAssignmentBuilder(length)
	)

	for row := 0; row < length; row++ {

		if !areLimbsZero(cfiTransposed[row]) && ((row+1 == length) || areLimbsZero(cfiTransposed[row+1])) {
			// This is the last row in the active area of the rom input.
			// We assign one more row to make the assignment of the last row
			// for other columns below work correctly, we exclude codeHash and
			// assign it below from the romLex input.
			builder.isActive = append(builder.isActive, field.Zero())
			builder.cfi = append(builder.cfi, [common.NbLimbU32]field.Element{field.Zero(), field.Zero()})

			for j := range builder.limb {
				builder.limb[j] = append(builder.limb[j], field.Zero())
			}

			for j := range builder.codeSize {
				builder.codeSize[j] = append(builder.codeSize[j], field.Zero())
			}

			break
		}

		if filter[row].IsZero() {
			continue
		}

		// Append 1 to isActive column
		builder.isActive = append(builder.isActive, field.One())

		// Inject the other incoming columns from the rom input
		var cfiRow [common.NbLimbU32]field.Element
		copy(cfiRow[:], cfiTransposed[row])
		builder.cfi = append(builder.cfi, cfiRow)

		for j := range builder.limb {
			builder.limb[j] = append(builder.limb[j], acc[j][row])
		}

		for j := range builder.codeSize {
			builder.codeSize[j] = append(builder.codeSize[j], codeSize[j][row])
		}
	}

	transposedLimb := transposeLimbs(builder.limb[:])
	// The content of this statement is constructing isNewHash and isHashEnd
	// prevState and newState. However, it is only needed when there is any
	// codehash to hash in the first place.
	if len(builder.cfi) > 0 {

		var (
			prevState = make([]field.Element, poseidon2.BlockSize)
		)
		// Initialize the first row of the remaining columns
		builder.isNewHash = append(builder.isNewHash, field.One())

		// Detect if it is a one-limb segment (at the beginning) and assign IsHashEnd accordingly
		if builder.cfi[1] != builder.cfi[0] {
			builder.isHashEnd = append(builder.isHashEnd, field.One())
		} else {
			builder.isHashEnd = append(builder.isHashEnd, field.Zero())
		}

		for i := range poseidon2.BlockSize {
			builder.prevState[i] = append(builder.prevState[i], field.Zero())
			prevState[i] = field.Zero()
		}

		compression := vortex.CompressPoseidon2(field.Octuplet(prevState), field.Octuplet(transposedLimb[0]))
		for i := range builder.newState {
			builder.newState[i] = append(builder.newState[i], compression[i])
		}

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

			if cfiPrev == cfiCurr && cfiCurr == cfiNext {
				isSegmentMiddle = true
			}

			if cfiPrev != cfiCurr && cfiCurr == cfiNext {
				isSegmentBegin = true
			}

			if cfiPrev == cfiCurr && cfiCurr != cfiNext {
				isSegmentEnd = true
			}

			if cfiPrev != cfiCurr && cfiCurr != cfiNext {
				isOneLimbSegment = true
			}

			switch {
			case isSegmentBegin:
				builder.isNewHash = append(builder.isNewHash, field.One())
				builder.isHashEnd = append(builder.isHashEnd, field.Zero())

				for j := range poseidon2.BlockSize {
					builder.prevState[j] = append(builder.prevState[j], field.Zero())
					prevState[j] = field.Zero()
				}
			case isSegmentMiddle:
				builder.isNewHash = append(builder.isNewHash, field.Zero())
				builder.isHashEnd = append(builder.isHashEnd, field.Zero())

				for j := range poseidon2.BlockSize {
					builder.prevState[j] = append(builder.prevState[j], builder.newState[j][i-1])
					prevState[j] = builder.newState[j][i-1]
				}
			case isSegmentEnd:
				builder.isNewHash = append(builder.isNewHash, field.Zero())
				builder.isHashEnd = append(builder.isHashEnd, field.One())

				for j := range poseidon2.BlockSize {
					builder.prevState[j] = append(builder.prevState[j], builder.newState[j][i-1])
					prevState[j] = builder.newState[j][i-1]
				}
			case isOneLimbSegment:
				builder.isNewHash = append(builder.isNewHash, field.One())
				builder.isHashEnd = append(builder.isHashEnd, field.One())

				for j := range poseidon2.BlockSize {
					builder.prevState[j] = append(builder.prevState[j], field.Zero())
					prevState[j] = field.Zero()
				}
			default:
				panic("invalid segment type")
			}

			compression = vortex.CompressPoseidon2(field.Octuplet(prevState), field.Octuplet(transposedLimb[i]))

			for j := range poseidon2.BlockSize {
				builder.newState[j] = append(builder.newState[j], compression[j])
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
			for j := 0; j < len(cfiRomLexTransposed); j++ {
				areCfiEqual := true
				for k := range common.NbLimbU32 {
					if currCFI[k] != cfiRomLexTransposed[j][k] {
						areCfiEqual = false
						break
					}
				}

				if areCfiEqual {

					currIsNonEmptyKeccakLimbs := true
					for k := range common.NbLimbU256 {
						if builder.isHashEnd[i].IsZero() {
							currIsNonEmptyKeccakLimbs = false
						}

						if codeHash[k][j] == emptyKeccak[k] {
							currIsNonEmptyKeccakLimbs = false
						}

						builder.codeHash[k] = append(builder.codeHash[k], codeHash[k][j])
					}

					if currIsNonEmptyKeccakLimbs {
						builder.isNonEmptyKeccak = append(builder.isNonEmptyKeccak, field.One())
					} else {
						builder.isNonEmptyKeccak = append(builder.isNonEmptyKeccak, field.Zero())
					}

					break
				}
				continue
			}
		}
	}

	// Assign the columns of the Poseidon2 code hash module
	run.AssignColumn(mh.IsActive.GetColID(), smartvectors.RightZeroPadded(builder.isActive, mh.Inputs.Size))

	for j := range builder.cfi[0] {
		var cfiLimbCol []field.Element
		for i := range builder.cfi {
			cfiLimbCol = append(cfiLimbCol, builder.cfi[i][j])
		}

		run.AssignColumn(mh.CFI[j].GetColID(), smartvectors.RightZeroPadded(cfiLimbCol, mh.Inputs.Size))
	}

	for i := range common.NbLimbU128 {
		run.AssignColumn(mh.Limb[i].GetColID(), smartvectors.RightZeroPadded(builder.limb[i], mh.Inputs.Size))
	}

	run.AssignColumn(mh.IsNewHash.GetColID(), smartvectors.RightZeroPadded(builder.isNewHash, mh.Inputs.Size))
	run.AssignColumn(mh.IsHashEnd.GetColID(), smartvectors.RightZeroPadded(builder.isHashEnd, mh.Inputs.Size))
	run.AssignColumn(mh.IsForConsistency.GetColID(), smartvectors.RightZeroPadded(builder.isNonEmptyKeccak, mh.Inputs.Size))

	newStatePad := vortex.CompressPoseidon2(field.Octuplet(vector.Zero(8)), field.Octuplet(vector.Zero(8)))
	for i := range common.NbLimbU256 {
		run.AssignColumn(mh.CodeHash[i].GetColID(), smartvectors.RightZeroPadded(builder.codeHash[i], mh.Inputs.Size))

		// assign isEmptyKeccak[i]
		mh.CptIsEmptyKeccak[i].Run(run)
	}

	for i := range poseidon2.BlockSize {
		run.AssignColumn(mh.PrevState[i].GetColID(), smartvectors.RightZeroPadded(builder.prevState[i], mh.Inputs.Size))
		// Assignment of new state with the zero hash padding
		run.AssignColumn(mh.NewState[i].GetColID(), smartvectors.RightPadded(builder.newState[i], newStatePad[i], mh.Inputs.Size))
	}

	for i := range common.NbLimbU32 {
		run.AssignColumn(mh.CodeSize[i].GetColID(), smartvectors.RightZeroPadded(builder.codeSize[i], mh.Inputs.Size))
	}
}

// transposeLimbs transforms a [dim1][dim2]field.Element columns into a [dim2][dim1]field.Element columns.
func transposeLimbs(inputMatrix [][]field.Element) [][]field.Element {
	if len(inputMatrix) == 0 || len(inputMatrix[0]) == 0 {
		return [][]field.Element{}
	}

	rows := len(inputMatrix)
	cols := len(inputMatrix[0])

	outputMatrix := make([][]field.Element, cols)
	for i := range outputMatrix {
		outputMatrix[i] = make([]field.Element, rows)
	}

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			outputMatrix[j][i] = inputMatrix[i][j]
		}
	}
	return outputMatrix
}

// areLimbsZero checks whether the provided value (represented in limbs) is zero.
// It returns false if some limb is not zero.
func areLimbsZero(limbs []field.Element) bool {
	for i := range limbs {
		if !limbs[i].IsZero() {
			return false
		}
	}

	return true
}
