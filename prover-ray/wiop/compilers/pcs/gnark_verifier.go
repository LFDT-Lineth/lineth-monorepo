package pcs

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It is the in-circuit
// counterpart of [compiled.verify]: it replays the opening transcript on the
// runtime's Fiat-Shamir state, gathers the claimed evaluations from the proof
// cells and hands everything to [fri.PCS.VerifyGnark].
//
// Only statically sized modules are supported: the batch shapes, and with
// them the circuit geometry, are a function of the module sizes.
func (a *OpeningVerifierAction) CheckGnark(api frontend.API, run *wiop.GnarkRuntime) {
	if run.PCSOpeningProof == nil {
		panic("pcs: CheckGnark: the witness carries no PCS opening proof")
	}
	proof := run.PCSOpeningProof
	batches := CommittedBatches(run.System)
	shifts, claims, shapes, zeta := recoverBatchClaimsGnark(run, batches)

	pcs := newStaticPCS()
	fs := run.FS()

	// Same transcript as compiled.verify: one fold challenge per round, each
	// intermediate root absorbed, the final challenge squeezed on its own.
	foldAlphas := make([]circuit.Ext, 0, len(proof.FRIProof.RoundRoots)+1)
	for _, friRoot := range proof.FRIProof.RoundRoots {
		foldAlphas = append(foldAlphas, fs.RandomFext())
		fs.UpdateOctuplet(friRoot)
	}
	foldAlphas = append(foldAlphas, fs.RandomFext())

	fs.UpdateExt(proof.FRIProof.FinalPoly...)
	queryPositions := fs.RandomManyIntegers(int(pcs.Params.NumQueries), effectiveNWith(staticSizeOf, batches))

	pcs.VerifyGnark(api, fri.GnarkVerifyInputs{
		Roots:          a.c.collectRootsGnark(run, batches),
		Shapes:         shapes,
		Shifts:         shifts,
		ClaimedValues:  claims,
		Zeta:           zeta,
		FoldAlphas:     foldAlphas,
		QueryPositions: queryPositions,
	}, *proof)
}

// collectRootsGnark mirrors [compiled.collectRoots]: transported roots from the
// witness, the precomputed root as a circuit constant.
func (c *compiled) collectRootsGnark(run *wiop.GnarkRuntime, batches []BatchRef) []poseidon2.KoalagnarkOctuplet {
	api := run.API()
	roots := make([]poseidon2.KoalagnarkOctuplet, len(batches))
	for i, b := range batches {
		if b.IsPrecomp {
			for j := range roots[i] {
				roots[i][j] = api.ConstBig(c.precomputedRoot[j].BigInt(new(big.Int)))
			}
			continue
		}
		roots[i] = run.GetCommitment(b.Round.ID)
	}
	return roots
}

// recoverBatchClaimsGnark mirrors [RecoverBatchClaims] over circuit values.
// The shift schedule and shapes are structural and identical to the native
// ones; the claimed values are proof cells, and the consistency checks the
// native version performs by comparison (shared evaluation point, duplicate
// openings) become equality constraints.
func recoverBatchClaimsGnark(run *wiop.GnarkRuntime, batches []BatchRef) (
	[]fri.BatchShifts,
	[]fri.GnarkBatchClaimedValues,
	[]fri.Shape,
	circuit.Ext,
) {
	var (
		sys       = run.System
		api       = run.API()
		layouts   = make([]map[wiop.ObjectID]ColumnLocation, len(batches))
		shapes    = make([]fri.Shape, len(batches))
		shifts    = make([]fri.BatchShifts, len(batches))
		claims    = make([]fri.GnarkBatchClaimedValues, len(batches))
		batchOf   = make(map[*wiop.Round]int, len(batches))
		seen      = make(map[claimKey]circuit.Ext)
		evalPoint *circuit.Ext
		evalExpr  wiop.FieldPromise
	)

	for i, b := range batches {
		layouts[i], shapes[i] = getLayoutWith(b.Round, staticSizeOf)
		shifts[i] = initializeBatchShift(shapes[i])
		claims[i] = initializeGnarkBatchClaims(shapes[i])
		batchOf[b.Round] = i
	}

	for _, eval := range sys.LagrangeEvals {
		// Every LagrangeEval opens at the same point. Sharing the same symbolic
		// expression is the common case and needs no constraint; a distinct
		// expression is constrained to evaluate to the same value.
		if evalPoint == nil {
			x := run.EvaluateSingle(eval.EvaluationPoint, nil)
			evalPoint, evalExpr = &x, eval.EvaluationPoint
		} else if eval.EvaluationPoint != evalExpr {
			api.AssertIsEqualExt(*evalPoint, run.EvaluateSingle(eval.EvaluationPoint, nil))
		}

		for k, colView := range eval.Polynomials {
			round := colView.Column.Round()
			batchIdx, ok := batchOf[round]
			if !ok {
				panic(fmt.Sprintf("pcs: column %q is in a round that owns no committed batch",
					colView.Column.Context.Path()))
			}

			loc := layouts[batchIdx][colView.Column.Context.ID]
			size := 1 << loc.SizeID
			shift := ((colView.ShiftingOffset % size) + size) % size
			value := run.GetCellValue(eval.EvaluationClaims[k])

			key := claimKey{batchIdx, loc.SizeID, loc.IsExt, loc.Position, shift}
			if prev, dup := seen[key]; dup {
				api.AssertIsEqualExt(prev, value)
				continue
			}
			seen[key] = value

			sizedShift := &shifts[batchIdx][loc.SizeID]
			sizedClaim := &claims[batchIdx][loc.SizeID]
			if loc.IsExt {
				sizedShift.Ext[loc.Position] = append(sizedShift.Ext[loc.Position], shift)
				sizedClaim.Ext[loc.Position] = append(sizedClaim.Ext[loc.Position], value)
			} else {
				sizedShift.Base[loc.Position] = append(sizedShift.Base[loc.Position], shift)
				sizedClaim.Base[loc.Position] = append(sizedClaim.Base[loc.Position], value)
			}
		}
	}

	if evalPoint == nil {
		panic("pcs: no LagrangeEval queries to open")
	}
	return shifts, claims, shapes, *evalPoint
}

func initializeGnarkBatchClaims(shape fri.Shape) fri.GnarkBatchClaimedValues {
	batchClaims := make(fri.GnarkBatchClaimedValues, len(shape))
	for i, sizedShape := range shape {
		batchClaims[i] = fri.GnarkSizedClaimedValues{
			Base: make([][]circuit.Ext, sizedShape.BaseWidth),
			Ext:  make([][]circuit.Ext, sizedShape.ExtWidth),
		}
	}
	return batchClaims
}
