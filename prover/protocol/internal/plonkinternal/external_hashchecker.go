package plonkinternal

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// addHashConstraint adds the constraints the hashes of the tagged witness.
// It checks that the assignment of the hash column is consistent with the
// LRO columns using a lookup and then it uses a MiMC query to enforce the
// hash.
func (ctx *compilationCtx) addHashConstraint() {

	var (
		numRowLRO = ctx.DomainSize()
		round     = ctx.Columns.L[0].Round()
		eho       = &ctx.ExternalHasherOption

		// Records the positions of the hash claims in the Plonk rows.
		posOsSv, posBlSv, posNsSv = ctx.getHashCheckedPositionSV()
		size                      = posOsSv.Len()

		// Declare the L, R, O position columns. These will be cached and
		// reused by the fixed permutation compiler.
		posL = dedicated.CounterPrecomputed(ctx.comp, 0, numRowLRO)
		posR = dedicated.CounterPrecomputed(ctx.comp, numRowLRO, 2*numRowLRO)
		posO = dedicated.CounterPrecomputed(ctx.comp, 2*numRowLRO, 3*numRowLRO)
	)

	eho.PosOldState = ctx.comp.InsertPrecomputed(ctx.colIDf("HashCheckPositionOS"), posOsSv)
	eho.PosBlock = ctx.comp.InsertPrecomputed(ctx.colIDf("HashCheckPositionBL"), posBlSv)
	eho.PosNewState = ctx.comp.InsertPrecomputed(ctx.colIDf("HashCheckPositionNS"), posNsSv)
	eho.OldStates = make([]ifaces.Column, ctx.MaxNbInstances)
	eho.Blocks = make([]ifaces.Column, ctx.MaxNbInstances)
	eho.NewStates = make([]ifaces.Column, ctx.MaxNbInstances)

	for i := 0; i < ctx.MaxNbInstances; i++ {

		var (
			selector = verifiercol.NewRepeatedAccessor(
				accessors.NewFromPublicColumn(ctx.Columns.Activators[i], 0),
				size,
			)

			lookupTables = [][]ifaces.Column{
				{
					posL, ctx.Columns.L[i],
				},
				{
					posR, ctx.Columns.R[i],
				},
				{
					posO, ctx.Columns.O[i],
				},
			}
		)

		eho.OldStates[i] = ctx.comp.InsertCommit(round, ctx.colIDf("HashCheckOldState_%v", i), size)
		eho.Blocks[i] = ctx.comp.InsertCommit(round, ctx.colIDf("HashCheckBlock_%v", i), size)
		eho.NewStates[i] = ctx.comp.InsertCommit(round, ctx.colIDf("HashCheckNewState_%v", i), size)

		// Those are lookups checking that the LRO columns are consistent with
		// the hash claims.

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("HashCheckImportOldState_%v", i),
			// including
			lookupTables,
			// included
			[]ifaces.Column{
				eho.PosOldState, eho.OldStates[i],
			},
			// no filters
			nil, nil,
		)

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("HashCheckImportBlock_%v", i),
			// including
			lookupTables,
			// included
			[]ifaces.Column{
				eho.PosBlock, eho.Blocks[i],
			},
			// no filters
			nil, nil,
		)

		ctx.comp.GenericFragmentedConditionalInclusion(
			round,
			ctx.queryIDf("HashCheckImportNewState_%v", i),
			// including
			lookupTables,
			// included
			[]ifaces.Column{
				eho.PosNewState, eho.NewStates[i],
			},
			// no filters
			nil, nil,
		)

		ctx.comp.InsertMiMC(
			round,
			ctx.queryIDf("HashCheckMiMC_%v", i),
			eho.Blocks[i],
			eho.OldStates[i],
			eho.NewStates[i],
			selector,
		)
	}
}

// assignHashColumns assigns the hash c olumns.
func (ctx *GenericPlonkProverAction) assignHashColumns(run *wizard.ProverRuntime) {

	var (
		eho         = &ctx.ExternalHasherOption
		posOs       = eho.PosOldState.GetColAssignment(run).IntoRegVecSaveAlloc()
		posBl       = eho.PosBlock.GetColAssignment(run).IntoRegVecSaveAlloc()
		posNs       = eho.PosNewState.GetColAssignment(run).IntoRegVecSaveAlloc()
		sizeHashing = len(posOs)
	)

	for i := 0; i < ctx.MaxNbInstances; i++ {

		var (
			src = []smartvectors.SmartVector{
				ctx.Columns.L[i].GetColAssignment(run),
				ctx.Columns.R[i].GetColAssignment(run),
				ctx.Columns.O[i].GetColAssignment(run),
			}
			sizeLRO  = ctx.Columns.L[i].Size()
			oldState = make([]field.Element, sizeHashing)
			block    = make([]field.Element, sizeHashing)
			newState = make([]field.Element, sizeHashing)
		)

		for j := range oldState {

			var (
				osID = int(posOs[j].Uint64())
				blID = int(posBl[j].Uint64())
				nsID = int(posNs[j].Uint64())
				os   = src[osID/sizeLRO].Get(osID % sizeLRO)
				bl   = src[blID/sizeLRO].Get(blID % sizeLRO)
				ns   = src[nsID/sizeLRO].Get(nsID % sizeLRO)
			)

			oldState[j] = os
			block[j] = bl
			newState[j] = ns
		}

		run.AssignColumn(eho.OldStates[i].GetColID(), smartvectors.NewRegular(oldState))
		run.AssignColumn(eho.Blocks[i].GetColID(), smartvectors.NewRegular(block))
		run.AssignColumn(eho.NewStates[i].GetColID(), smartvectors.NewRegular(newState))
	}
}

// getHashCheckedPositionSV returns the smartvectors containing the position
// of the hash claims in the LRO columns.
func (ctx *compilationCtx) getHashCheckedPositionSV() (posOS, posBl, posNS smartvectors.SmartVector) {

	var (
		sls         = ctx.Plonk.hashedGetter()
		size        = utils.NextPowerOfTwo(len(sls))
		numRowPlonk = ctx.DomainSize()
	)

	if ctx.ExternalHasherOption.FixedNbRows > 0 {
		fixedNbRow := ctx.ExternalHasherOption.FixedNbRows
		if fixedNbRow < size {
			utils.Panic("the fixed number of rows %v is smaller than the number of hash claims %v", fixedNbRow, len(sls))
		}
		size = ctx.ExternalHasherOption.FixedNbRows
	}

	var (
		ost = make([]field.Element, size)
		blk = make([]field.Element, size)
		nst = make([]field.Element, size)
	)

	for i, ss := range sls {
		ost[i].SetUint64(uint64(ss[0][0] + ss[0][1]*numRowPlonk))
		blk[i].SetUint64(uint64(ss[1][0] + ss[1][1]*numRowPlonk))
		nst[i].SetUint64(uint64(ss[2][0] + ss[2][1]*numRowPlonk))
	}

	for i := len(sls); i < size; i++ {
		ost[i] = ost[i-1]
		blk[i] = blk[i-1]
		nst[i] = nst[i-1]
	}

	return smartvectors.NewRegular(ost), smartvectors.NewRegular(blk), smartvectors.NewRegular(nst)
}
