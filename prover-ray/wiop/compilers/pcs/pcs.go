// pcs implements the polynomial commitment part of the proof system as protocol
// compilation step.
package pcs

import (
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	pcsProverStateKey = "pcsProverState"
)

func Compile(sys *wiop.System) {
	// Scan all the columns:
	//		- map them to a round or precomputed round
	//		- mark them as internal
	// 	- register an object to hold the commitment
	//  - register an object to hold the FRI commitment
	//	- register an object to hold the Merkle proofs

}

type ColumnLocation struct {
	RoundID  int
	SizeID   int
	Position int
	IsExt    bool
}

func computeLayout(rt *wiop.Runtime) {

}

// markColumnsAsInternal marks all the columns in a round as internal.
func markColumnsAsInternal(round *wiop.Round) {
	for i := range round.Columns {
		round.Columns[i].Visibility = wiop.VisibilityInternal
	}
}

func commitToRound(encoder []*fri.RSEncoder, round *wiop.Round,
	rt *wiop.Runtime) *fri.CommitterState {

	var (
		cols          = round.Columns
		sortedColumns = make(fri.MultiSizeTable, 64)
		maxSizeIndex  = 0
	)

	for _, col := range cols {

		size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
		sizeIndex := utils.Log2Ceil(size)
		assignment := rt.GetColumnAssignment(col)

		if size != 1<<sizeIndex {
			panic("wiop: only powers of 2 are supported")
		}

		maxSizeIndex = max(maxSizeIndex, sizeIndex)

		if col.IsExtension {
			sortedColumns[sizeIndex].Ext = append(
				sortedColumns[sizeIndex].Ext,
				writeDownVectorExt(assignment, size),
			)
		} else {
			sortedColumns[sizeIndex].Base = append(
				sortedColumns[sizeIndex].Base,
				writeDownVectorBase(assignment, size),
			)
		}
	}

	committerState := fri.Commit(encoder, sortedColumns[:maxSizeIndex+1])
	return &committerState
}

func getLayout(round *wiop.Round, rt *wiop.Runtime) (map[wiop.ObjectID]ColumnLocation, fri.Shape) {

	var (
		cols         = round.Columns
		layout       = make(map[wiop.ObjectID]ColumnLocation, len(cols))
		shape        = make(fri.Shape, 64)
		maxSizeIndex = 0
	)

	for _, col := range cols {

		size := utils.NextPowerOfTwo(col.Module.RuntimeSize(rt))
		sizeIndex := utils.Log2Ceil(size)

		if size != 1<<sizeIndex {
			panic("wiop: only powers of 2 are supported")
		}

		position := shape[sizeIndex].BaseWidth
		if col.IsExtension {
			position = shape[sizeIndex].ExtWidth
		}

		layout[col.Context.ID] = ColumnLocation{
			SizeID:   sizeIndex,
			Position: position,
			IsExt:    col.IsExtension,
			RoundID:  round.ID,
		}

		maxSizeIndex = max(maxSizeIndex, sizeIndex)
	}

	return layout, shape
}

func verifyOpenings(
	pcs *fri.PCS,
	rt *wiop.Runtime,
	proof fri.OpeningProof,
) error {

	batchShifts, batchClaims, shapes, evalPoint := recoverBatchClaims(rt)

	fs := rt.GetFS()
	alphaDeep := fs.RandomFext()

	// Mirror the prover's Fiat-Shamir transcript: one fold challenge per round,
	// absorbing each intermediate layer root. The final round reveals the final
	// polynomial and commits no root, so its challenge is squeezed without a
	// matching absorption.
	foldAlphas := make([]field.Ext, 0, len(proof.FRIProof.FRIRoots)+1)
	for _, friRoot := range proof.FRIProof.FRIRoots {
		foldAlphas = append(foldAlphas, fs.RandomFext())
		fs.Update(friRoot[:]...)
	}
	foldAlphas = append(foldAlphas, fs.RandomFext())

	fs.UpdateExt(proof.FRIProof.FinalPolyExt...)
	queryPositions := fs.RandomManyIntegers(
		pcs.Params.NumQueries,
		pcs.Params.N,
	)

	inputs := fri.VerifyInputs{
		Roots:         getCommitmentRootList(rt),
		ClaimedValues: batchClaims,
		Shapes:        shapes,
		Shifts:        batchShifts,
		Zeta:          evalPoint,
		Challenges: fri.Challenges{
			AlphaDeep:      alphaDeep,
			FoldAlphas:     foldAlphas,
			QueryPositions: queryPositions,
		},
	}

	return pcs.Verify(inputs, proof)
}

func openEvaluations(
	pcs *fri.PCS,
	committedStates []*fri.CommitterState,
	rt *wiop.Runtime,
) fri.OpeningProof {

	batchShifts, batchClaims, _, evalPoint := recoverBatchClaims(rt)

	pcs.Reset()
	for i := range committedStates {
		err := pcs.AddOpening(*committedStates[i], evalPoint, batchShifts[i], batchClaims[i])
		if err != nil {
			panic(err)
		}
	}

	var (
		fs        = rt.GetFS()
		alphaDeep = fs.RandomFext()
	)

	// The DEEP quotient is virtual: it is reconstructed by the verifier from the
	// opened committed rows, so there is no separate quotient commitment to absorb
	// here. Seeding the FRI prover with alphaDeep is enough.
	state, err := pcs.NewProverState(alphaDeep)
	if err != nil {
		panic(err)
	}

	for state.HasNext() {
		alphaFold := fs.RandomFext()
		friCom := state.Fold(alphaFold)
		// The last fold reveals the final polynomial and has no committed root
		// (state.Fold returns the zero octuplet), so only absorb intermediate
		// layer roots.
		if state.HasNext() {
			fs.Update(friCom[:]...)
		}
	}

	finalPoly := state.FinalPolyExt
	// @alex: there is a small optimization we could do here as the poly is
	// of constant degree. It should be OK to just hash one of its coordinates.
	//
	// Of course, this still requires the verifier checking the low-degreeness
	// of the provided polynomials. If done here, it should also be reflected
	// on the verification helper.
	fs.UpdateExt(finalPoly...)

	openedPositions := fs.RandomManyIntegers(pcs.Params.NumQueries, pcs.Params.N)
	return fri.OpeningProof{
		RowOpenings: pcs.OpenedRows(openedPositions),
		FRIProof:    state.Open(openedPositions),
	}
}

func recoverBatchClaims(rt *wiop.Runtime) (
	[]fri.BatchShifts,
	[]fri.BatchClaimedValues,
	[]fri.Shape,
	field.Ext) {

	var (
		precomputedRoundLayout, precomputedRoundShape = getLayout(
			&rt.System.PrecomputedRound.Round,
			rt,
		)

		rounds                = rt.System.Rounds
		evals                 = rt.System.LagrangeEvals
		evalPoint             *field.Ext
		precomputedBatchShift = initializeBatchShift(precomputedRoundShape)
		precomputedBatchClaim = initializeBatchClaims(precomputedRoundShape)
		roundLayouts          = make([]map[wiop.ObjectID]ColumnLocation, len(rounds))
		roundBatchShifts      = make([]fri.BatchShifts, len(rounds), len(rounds)+1)
		roundBatchClaims      = make([]fri.BatchClaimedValues, len(rounds), len(rounds)+1)
		roundShapes           = make([]fri.Shape, len(rounds), len(rounds)+1)
	)

	for i, round := range rounds {
		roundLayouts[i], roundShapes[i] = getLayout(round, rt)
		roundBatchShifts[i] = initializeBatchShift(roundShapes[i])
		roundBatchClaims[i] = initializeBatchClaims(roundShapes[i])
	}

	for _, eval := range evals {

		xExt := eval.EvaluationPoint.EvaluateSingle(rt).Value.AsExt()
		if evalPoint == nil {
			evalPoint = &xExt
		}

		if !evalPoint.Equal(&xExt) {
			// This check is a bit overly strict as it enforces pointer equality
			// but we can relax it into a value equality.
			panic("the evaluation point should be unique")
		}

		for k, colView := range eval.Polynomials {

			var (
				shift           = colView.ShiftingOffset
				roundID         = colView.Column.Round().ID
				location, found = roundLayouts[roundID][colView.Column.Context.ID]
				batchShift      = roundBatchShifts[roundID]
				batchClaim      = roundBatchClaims[roundID]
				claimedValue    = rt.GetCellValue(eval.EvaluationClaims[k])
			)

			if colView.Column.Round() == &rt.System.PrecomputedRound.Round {
				location, found = precomputedRoundLayout[colView.Column.Context.ID]
				batchShift = precomputedBatchShift
				batchClaim = precomputedBatchClaim
			}

			if !found {
				panic("column not found in the layout")
			}

			var (
				sizedShift = batchShift[location.SizeID]
				sizedClaim = batchClaim[location.SizeID]
			)

			if location.IsExt {

				sizedShift.Ext[location.Position] = append(
					sizedShift.Ext[location.Position],
					shift,
				)

				assertNoDuplicate(sizedShift.Ext[location.Position])

				sizedClaim.Ext[location.Position] = append(
					sizedClaim.Ext[location.Position],
					claimedValue.AsExt(),
				)

			} else {

				sizedShift.Base[location.Position] = append(
					sizedShift.Base[location.Position],
					shift,
				)

				assertNoDuplicate(sizedShift.Base[location.Position])

				sizedClaim.Base[location.Position] = append(
					sizedClaim.Base[location.Position],
					claimedValue.AsExt(),
				)
			}
		}
	}

	return append(roundBatchShifts, precomputedBatchShift),
		append(roundBatchClaims, precomputedBatchClaim),
		append(roundShapes, precomputedRoundShape),
		*evalPoint
}

func initializeBatchShift(shape fri.Shape) fri.BatchShifts {
	batchShifts := make(fri.BatchShifts, len(shape))
	for i, sizedShape := range shape {
		batchShifts[i] = fri.SizedShifts{
			Base: make([][]int, sizedShape.BaseWidth),
			Ext:  make([][]int, sizedShape.ExtWidth),
		}
	}
	return batchShifts
}

func initializeBatchClaims(shape fri.Shape) fri.BatchClaimedValues {
	batchShifts := make(fri.BatchClaimedValues, len(shape))
	for i, sizedShape := range shape {
		batchShifts[i] = fri.SizedClaimedValues{
			Base: make([][]field.Ext, sizedShape.BaseWidth),
			Ext:  make([][]field.Ext, sizedShape.ExtWidth),
		}
	}
	return batchShifts
}

func getCommitmentRootList(rt *wiop.Runtime) []field.Octuplet {

	res := make([]field.Octuplet, len(rt.System.Rounds))
	for i := range rt.System.Rounds {
		res[i] = rt.Commitments[i]
	}

	res = append(res, rt.System.PrecomputedCommitment)
	return res
}

// assertNoDuplicate checks if the list contains any duplicates.
func assertNoDuplicate(list []int) {
	newList := slices.Clone(list)
	slices.Sort(newList)
	for i := range newList[1:] {
		if newList[i] == newList[i+1] {
			panic("duplicate")
		}
	}
}

func writeDownVectorBase(concrete *wiop.ConcreteVector, size int) []field.Element {

	if !concrete.Plain.IsBase() {
		panic("is not base")
	}

	plainBase := concrete.Plain.AsBase()
	plain := slices.Grow(plainBase, size)
	plain = plain[:size]

	for i := len(plainBase); i < size; i++ {
		// Sanity-check that the padding is only overwriting zeroes.
		if plain[i].IsZero() {
			panic("overwriting non-zero value. Check that PadWith is only called once on the same vector.")
		}

		plain[i] = concrete.Padding
	}

	return plain
}

func writeDownVectorExt(concrete *wiop.ConcreteVector, size int) []field.Ext {

	plainExt := concrete.Plain.AsExt()
	plain := slices.Grow(plainExt, size)
	padExt := field.Lift(concrete.Padding)
	plain = plain[:size]

	for i := len(plainExt); i < size; i++ {
		// Sanity-check that the padding is only overwriting zeroes.
		if plain[i].IsZero() {
			panic("overwriting non-zero value. Check that PadWith is only called once on the same vector.")
		}

		plain[i] = padExt
	}

	return plain
}
