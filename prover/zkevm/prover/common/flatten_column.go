package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	// NbLimbU32 represents the number of 16-bit limbs for a 32-bit integer.
	NbLimbU32 = 2
	// NbLimbU48 represents the number of 16-bit limbs for a 48-bit integer.
	NbLimbU48 = 3
	// NbLimbU64 represents the number of 16-bit limbs for a 64-bit integer.
	NbLimbU64 = 4
	// NbLimbEthAddress represents the number of 16-bit limbs for an Ethereum address (160 bits).
	NbLimbEthAddress = 10
	// NbLimbU128 represents the number of 16-bit limbs for a 128-bit integer.
	NbLimbU128 = 8
	// NbLimbU256 represents the number of 16-bit limbs for a 256-bit integer.
	NbLimbU256 = 16
	// NbElemPerHash represents the number of field elements per Posseidon hash.
	NbElemPerHash = 8
	// NbElemForHasingU64 represents the number of field elements per 64-bit integers.
	NbElemForHasingU64 = 16
	// NbElemForHashingByte32Sandwitch represents the number of field elements per byte32 (256 bits).
	NbElemForHashingByte32Sandwitch = 16
	// NbBytesForEncodingU64 represents the number of bytes for encoding a 64-bit integer.
	NbBytesForEncodingU64 = 64
	// NbBytesForEncodingFieldHash represents the number of bytes for encoding a field hash.
	NbBytesForEncodingFieldHash = 32
)

// FlattenColumn flattens multiple limb columns and an accompanying mask into single columns,
// provides consistency checks via a precomputed projection mask.
type FlattenColumn struct {
	// OriginalLimbs holds the original limb columns to flatten.
	OriginalLimbs []ifaces.Column
	// OriginalMask holds the original mask column that selects elements for gnark circuit.
	OriginalMask ifaces.Column
	// Limbs is the row-wise concatenation of all limb columns.
	Limbs ifaces.Column
	// Mask is the row-wise concatenation of the original Mask column.
	Mask ifaces.Column
	// AuxProjectionMask selects flattenLimbs's positions to validate flattening consistency.
	AuxProjectionMask ifaces.Column
	// OnesColumn selects elements from the original limbs. This is always a column of 1s.
	OnesColumn ifaces.Column

	// NbLimbsCols is the number of limb columns to flatten.
	NbLimbsCols int
	// Size is the length of the produced flattened column.
	Size int
	// IsDuplicated indicates if this FlattenColumn is already registered by other circuit,
	// so we don't need to commit to a new one.
	IsDuplicated bool
	// ShiftedFlattenLimbs are the manually shifted versions of the flattened limbs column needed for the distributed prover
	ShiftedFlattenLimbs []*dedicated.ManuallyShifted
	// ShiftedFlattenMask are the manually shifted versions of the flattened mask column needed for the distributed prover
	ShiftedFlattenMask []*dedicated.ManuallyShifted
}

// NewFlattenColumn initializes a FlattenColumn with:
//   - size: length of the original limbs columns
//   - nbLimbsCols: number of limb columns to flatten
//   - limbs: original limb columns to flatten
//   - mask: original mask column for original limbs
//
// It commits placeholders for flattened limbs and mask, and precomputes the projection mask.
func NewFlattenColumn[E limbs.Endianness](
	comp *wizard.CompiledIOP,
	limbs limbs.Limbs[E],
	mask ifaces.Column,
) *FlattenColumn {
	var (
		onesColumnID = ifaces.ColIDf("%s_FLATTEN_ORIG_LIMBS_MASK", limbs.String())
		initialSize  = mask.Size()
		nbLimbsCols  = limbs.NumLimbs()
		// If the column already exists, we assume it is already registered by another circuit.
		isDuplicated bool
		onesColumn   ifaces.Column
	)

	if comp.Columns.Exists(onesColumnID) {
		onesColumn = comp.Columns.GetHandle(onesColumnID)
	} else {
		onesColumn = verifiercol.NewConstantCol(field.One(), initialSize, string(onesColumnID))
	}

	flattenSize := utils.NextPowerOfTwo(initialSize * nbLimbsCols)
	res := &FlattenColumn{
		Size:          flattenSize,
		OriginalMask:  mask,
		OriginalLimbs: limbs.GetLimbs(),
		NbLimbsCols:   nbLimbsCols,
		OnesColumn:    onesColumn,
		IsDuplicated:  isDuplicated,
	}

	res.initColumns(comp)
	res.Mask = comp.InsertCommit(0, res.MaskColID(), flattenSize, true)

	return res
}

// // Limbs returns the flattened limbs column.
// func (l *FlattenColumn) Limbs() ifaces.Column {
// 	return l.limbs
// }

// // Mask returns the flattened mask column.
// func (l *FlattenColumn) Mask() ifaces.Column {
// 	return l.mask
// }

// LimbsColID returns the column ID of the flattened limbs.
func (l *FlattenColumn) LimbsColID() ifaces.ColID {
	return ifaces.ColIDf("%s_FLATTEN_LIMBS", l.OriginalLimbs[0].GetColID())
}

// MaskColID returns the column ID of the flattened mask.
func (l *FlattenColumn) MaskColID() ifaces.ColID {
	return ifaces.ColIDf("%s_FLATTEN_MASK", l.OriginalMask.GetColID())
}

// TODO: This function is commented out because it does not work with the limitless mode.

// CsFlattenProjection adds a single batched projection constraint that enforces
// the “flattened” limbs and mask columns exactly match the row‐wise concatenation
// of the original limb columns and their mask.
//
// It works by:
//  1. Shifting the committed flattened limbs and mask by 0…nbLimbsCols−1 rows.
//  2. Duplicating the original limbs and mask into nbLimbsCols slots.
//  3. Using auxProjectionMask (a sparse selector with a 1 at each block start)
//     and a onesColumn to feed into the projection gadget.
//  4. Requiring at each selected position that
//     shiftedFlattenLimbs[i] == originalLimbs[i]
//     shiftedFlattenMask[i]  == originalMask
//     for all limb indices i and row positions.
//
// Suppose nbLimbsCols = 3, size = 4:
// original limbs (rows × limbs):
//
//	r\i   0    1    2
//	 0  [a0,  b0,  c0]
//	 1  [a1,  b1,  c1]
//	 2  [a2,  b2,  c2]
//	 3  [a3,  b3,  c3]
//
// flattened (size*3 = 12):
//
//	[a0, b0, c0,  a1, b1, c1,  a2, b2, c2,  a3, b3, c3]
//
// shift0 (i=0):
//
//	[a0, b0, c0,  a1, b1, c1,  a2, b2, c2,  a3, b3, c3]
//
// shift1 (i=1):
//
//	[b0, c0, a1,  b1, c1, a2,  b2, c2, a3,  b3, c3,  0 ]
//
// shift2 (i=2):
//
//	[c0, a1, b1,  c1, a2, b2,  c2, a3, b3,  c3,  0,   0 ]
//
// auxProjectionMask   = [1,0,0, 1,0,0, 1,0,0, 1,0,0]
//
// At each ‘1’ at idx = r*3, for shift i, enforce:
//
//	shift_i[idx] == original[r][i]
//
// CsFlattenProjection batches all these equalities into one projection check.
func (l *FlattenColumn) CsFlattenProjection(comp *wizard.CompiledIOP) {
	/*
		masks := make([]ifaces.Column, l.NbLimbsCols)
		l.ShiftedFlattenLimbs = make([]*dedicated.ManuallyShifted, l.NbLimbsCols)
		l.ShiftedFlattenMask = make([]*dedicated.ManuallyShifted, l.NbLimbsCols)

		for i := 0; i < l.NbLimbsCols; i++ {
			masks[i] = l.OriginalMask
			l.ShiftedFlattenLimbs[i] = dedicated.ManuallyShift(comp, l.Limbs, i, fmt.Sprintf("MANUALLY_SHIFTED_FLATTEN_LIMBS_%v_%v", i, comp.Columns.NumEntriesTotal()))
			l.ShiftedFlattenMask[i] = dedicated.ManuallyShift(comp, l.Mask, i, fmt.Sprintf("MANUALLY_SHIFTED_FLATTEN_MASK_%v_%v", i, comp.Columns.NumEntriesTotal()))
		}

		// append the Natural part of the shiftedFlattenLimbs and shiftedFlattenMask in colA and colB
		var (
			colA = make([]ifaces.Column, 0, len(l.ShiftedFlattenLimbs)+len(l.ShiftedFlattenMask))
		)

		for i := 0; i < l.NbLimbsCols; i++ {
			colA = append(colA, l.ShiftedFlattenLimbs[i].Natural)
		}

		for i := 0; i < l.NbLimbsCols; i++ {
			colA = append(colA, l.ShiftedFlattenMask[i].Natural)
		}

		// // This query needs manual shifting as per log
		// comp.InsertProjection(
		//
		//	ifaces.QueryIDf("%v_FLATTEN_LIMBS_PROJECTION", l.OriginalMask.GetColID()),
		//	query.ProjectionInput{
		//		ColumnA: colA,
		//		ColumnB: append(l.OriginalLimbs[:], masks[:]...),
		//		FilterA: l.AuxProjectionMask,
		//		FilterB: l.OnesColumn,
		//	},
		//
		// )
	*/
}

// initColumns initializes the FlattenColumn by committing to the flattened limbs
// and mask columns, and precomputing the projection mask.
func (l *FlattenColumn) initColumns(comp *wizard.CompiledIOP) {
	baseID := l.LimbsColID()
	auxProjectionMaskID := ifaces.ColIDf("%s_PROJECTION_MASK", baseID)

	// If the column already exists, we assume it is already registered by another circuit.
	if comp.Columns.Exists(baseID) {
		l.IsDuplicated = true

		l.Limbs = comp.Columns.GetHandle(baseID)
		l.AuxProjectionMask = comp.Columns.GetHandle(auxProjectionMaskID)

		return
	}

	l.Limbs = comp.InsertCommit(0, baseID, l.Size, true)
	l.AuxProjectionMask = comp.InsertPrecomputed(auxProjectionMaskID,
		precomputeAuxProjectionMask(l.Size, l.OriginalMask.Size(), l.NbLimbsCols))

	pragmas.MarkRightPadded(l.Limbs)

	// This is needed to hint the distributer that this column is completely
	// periodic and is no issue for splitting. We could as well replace it
	// by a RepeatedPattern column but the fact that the repetition period is
	// a power of two spares us the need to use more expensive and heavy
	// [RepeatedPattern] column.
	pragmas.MarkCompletelyPeriodic(l.AuxProjectionMask)
}

// Run maps trace limb columns and mask into the flattened columns.
func (l *FlattenColumn) Run(run *wizard.ProverRuntime) {
	l.assignMask(run)

	if !run.HasColumn(l.Limbs.GetColID()) {
		l.assignLimbs(run)
	}

	for i := range l.ShiftedFlattenLimbs {
		// GetColAssignment triggers the assignment of the column if it has not
		// been assigned already.
		l.ShiftedFlattenLimbs[i].GetColAssignment(run)
	}

	for i := range l.ShiftedFlattenMask {
		// GetColAssignment triggers the assignment of the column if it has not
		// been assigned already.
		l.ShiftedFlattenMask[i].GetColAssignment(run)
	}
}

func (l *FlattenColumn) assignMask(run *wizard.ProverRuntime) {
	maskCol := l.OriginalMask.GetColAssignment(run).IntoRegVecSaveAlloc()

	flattenMask := NewVectorBuilder(l.Mask)
	for i := 0; i < l.OriginalMask.Size(); i++ {
		for j := 0; j < l.NbLimbsCols; j++ {
			flattenMask.PushField(maskCol[i])
		}
	}

	flattenMask.PadAndAssign(run, field.Zero())
	for i := range l.ShiftedFlattenMask {
		l.ShiftedFlattenMask[i].GetColAssignment(run)
	}
}

func (l *FlattenColumn) assignLimbs(run *wizard.ProverRuntime) {

	// In theory, this should be set in the define function but this is a patch
	// that avoid needing to regenerate the setup.
	pragmas.MarkRightPadded(l.Limbs)

	limbsCols := make([][]field.Element, l.NbLimbsCols)
	for i, limb := range l.OriginalLimbs {
		limbsCols[i] = limb.GetColAssignment(run).IntoRegVecSaveAlloc()
	}

	flattenLimbs := NewVectorBuilder(l.Limbs)
	for i := 0; i < l.OriginalMask.Size(); i++ {
		for j := 0; j < l.NbLimbsCols; j++ {
			flattenLimbs.PushField(limbsCols[j][i])
		}
	}

	flattenLimbs.PadAndAssign(run, field.Zero())

	for i := range l.ShiftedFlattenLimbs {
		l.ShiftedFlattenLimbs[i].GetColAssignment(run)
	}
}

// precomputeAuxProjectionMask creates a SmartVector with total size `size`,
// where `nbMasked` positions are periodically set to one.
func precomputeAuxProjectionMask(size, nbMarks, period int) smartvectors.SmartVector {
	resSlice := make([]field.Element, size)

	offset := 0
	for i := 0; i < nbMarks; i++ {
		resSlice[offset].SetOne()
		offset += period
	}

	return smartvectors.NewRegular(resSlice)
}
