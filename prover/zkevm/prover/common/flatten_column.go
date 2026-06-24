package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
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

// FlattenColumn flattens multiple limb columns and an accompanying mask into
// single columns, enforces consistency via a multi-ary projection query.
type FlattenColumn struct {
	// OriginalLimbs holds the original limb columns to flatten.
	OriginalLimbs []ifaces.Column
	// OriginalMask holds the original mask column that selects elements for gnark circuit.
	OriginalMask ifaces.Column
	// Limbs is the row-wise concatenation of all limb columns.
	Limbs ifaces.Column
	// Mask is the row-wise concatenation of the original Mask column.
	Mask ifaces.Column
	// NbLimbsCols is the number of limb columns to flatten.
	NbLimbsCols int
	// Size is the length of the produced flattened column.
	Size int
}

// NewFlattenColumn initializes a FlattenColumn with:
//   - limbs: original limb columns to flatten
//   - mask: original mask column for original limbs
//
// It commits placeholders for flattened limbs and mask used by CsFlattenProjection.
func NewFlattenColumn[E limbs.Endianness](
	comp *wizard.CompiledIOP,
	limbs limbs.Limbs[E],
	mask ifaces.Column,
) *FlattenColumn {
	var (
		initialSize = mask.Size()
		nbLimbsCols = limbs.NumLimbs()
	)

	flattenSize := utils.NextPowerOfTwo(initialSize * nbLimbsCols)
	res := &FlattenColumn{
		Size:          flattenSize,
		OriginalMask:  mask,
		OriginalLimbs: limbs.GetLimbs(),
		NbLimbsCols:   nbLimbsCols,
	}

	res.initColumns(comp)
	res.Mask = comp.InsertCommit(0, res.MaskColID(), flattenSize, true)

	return res
}

// LimbsColID returns the column ID of the flattened limbs.
func (l *FlattenColumn) LimbsColID() ifaces.ColID {
	return ifaces.ColIDf("%s_FLATTEN_LIMBS", l.OriginalLimbs[0].GetColID())
}

// MaskColID returns the column ID of the flattened mask.
func (l *FlattenColumn) MaskColID() ifaces.ColID {
	return ifaces.ColIDf("%s_FLATTEN_MASK", l.OriginalMask.GetColID())
}

// CsFlattenProjection adds a multi-ary projection constraint enforcing that
// the flattened limbs column exactly matches the row-wise interleaving of the
// original limb columns.
//
// The projection has one A-part (the flattened Limbs column, filtered by the
// flattened Mask) and N B-parts — one per original limb column — each filtered
// by OriginalMask. Both iterators advance row-outer / part-inner, so the
// effective B sequence is:
//
//	origLimbs[0][0], origLimbs[1][0], …, origLimbs[N-1][0],
//	origLimbs[0][1], origLimbs[1][1], …, origLimbs[N-1][1], …
//
// which is exactly the row-major interleaving expected in Limbs.
//
// For each original row r where OriginalMask[r]==1 and limb index i:
//
//	flatLimbs[r*N+i] == originalLimbs[i][r]
//
// Note: the Mask column's interleaving pattern (flatMask[r*N+i] == origMask[r])
// is NOT directly enforced by this query. Mask correctness depends on the
// prover faithfully executing assignMask. A separate constraint would be
// required for full mask soundness.
func (l *FlattenColumn) CsFlattenProjection(comp *wizard.CompiledIOP) {
	columnsB := make([][]ifaces.Column, l.NbLimbsCols)
	filtersB := make([]ifaces.Column, l.NbLimbsCols)
	for i := 0; i < l.NbLimbsCols; i++ {
		columnsB[i] = []ifaces.Column{l.OriginalLimbs[i]}
		// One filter per B part; same mask column repeated (query matches filter count to parts).
		filtersB[i] = l.OriginalMask
	}

	if !comp.QueriesNoParams.Exists(ifaces.QueryIDf("%v_MUST_BE_BINARY", l.OriginalMask.GetColID())) {
		commonconstraints.MustBeBinary(comp, l.OriginalMask)
	}

	commonconstraints.MustBeBinary(comp, l.Mask)

	comp.InsertProjection(
		ifaces.QueryIDf("%v_FLATTEN_LIMBS_PROJECTION", l.OriginalMask.GetColID()),
		query.ProjectionMultiAryInput{
			ColumnsA: [][]ifaces.Column{{l.Limbs}},
			ColumnsB: columnsB,
			FiltersA: []ifaces.Column{l.Mask},
			FiltersB: filtersB,
		},
	)
}

// initColumns commits to the flattened limbs column.
// When the limbs column already exists (shared by another circuit), it
// reuses the existing handles.
func (l *FlattenColumn) initColumns(comp *wizard.CompiledIOP) {
	baseID := l.LimbsColID()

	if comp.Columns.Exists(baseID) {

		l.Limbs = comp.Columns.GetHandle(baseID)
		return
	}

	l.Limbs = comp.InsertCommit(0, baseID, l.Size, true)

}

// Run maps trace limb columns and mask into the flattened columns.
func (l *FlattenColumn) Run(run *wizard.ProverRuntime) {
	l.assignMask(run)
	if !run.HasColumn(l.Limbs.GetColID()) {
		l.assignLimbs(run)
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
}

func (l *FlattenColumn) assignLimbs(run *wizard.ProverRuntime) {
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
}
