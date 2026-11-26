package pcs

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark/frontend"
)

// CheckGnark implements [wiop.GnarkVerifierAction]. It is the in-circuit
// counterpart of [compiled.verify]: it replays the opening transcript on the
// runtime's Fiat-Shamir state, gathers the claimed evaluations from the proof
// cells and hands everything to [fri.VerifierPlan.VerifyGnark].
//
// The opening's structure is [OpeningVerifierAction.Static]'s, shared with
// the circuit's allocation ([OpeningVerifierAction.OpeningProofShape]), so
// both agree on the geometry by construction. Only statically sized modules
// and all-Present manifests are supported.
func (a *OpeningVerifierAction) CheckGnark(api frontend.API, run *wiop.GnarkRuntime) {
	if run.PCSOpeningProof == nil {
		panic("pcs: CheckGnark: the witness carries no PCS opening proof")
	}
	// The circuit's FRI layout is structural, while column manifests are
	// prover-chosen witness data: only all-Present manifests are supported.
	if a.c.elisionEnabled {
		panic("pcs: CheckGnark: all-Present manifests only: compile without WithColumnElision")
	}
	st, err := a.Static(run.System)
	if err != nil {
		panic(fmt.Sprintf("pcs: CheckGnark: %v", err))
	}
	proof := run.PCSOpeningProof
	claims, zeta := staticClaimsGnark(run, st)

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
	queryPositions := fs.RandomManyIntegers(st.NumQueries, st.QueryRange)

	st.Plan.VerifyGnark(api, fri.GnarkVerifyInputs{
		Roots:          collectRootsGnark(run, st),
		ClaimedValues:  claims,
		Zeta:           zeta,
		FoldAlphas:     foldAlphas,
		QueryPositions: queryPositions,
	}, *proof)
}

// collectRootsGnark mirrors [compiled.collectRoots]: transported roots from the
// witness, the precomputed root as a circuit constant.
func collectRootsGnark(run *wiop.GnarkRuntime, st *StaticOpening) []circuit.Octuplet {
	api := run.API()
	roots := make([]circuit.Octuplet, len(st.Batches))
	for i, b := range st.Batches {
		if b.IsPrecomp {
			for j := range roots[i] {
				roots[i][j] = api.ConstBig(st.PrecomputedRoot[j].BigInt(new(big.Int)))
			}
			continue
		}
		roots[i] = run.GetCommitment(b.Round.ID)
	}
	return roots
}

// staticClaimsGnark mirrors [RecoverBatchClaims] over circuit values. The
// layout is st's; the checks the native version performs by comparison
// (all-Present manifests, duplicate openings, shared evaluation point) become
// equality constraints.
func staticClaimsGnark(run *wiop.GnarkRuntime, st *StaticOpening) ([]fri.GnarkBatchClaimedValues, circuit.Ext) {
	api := run.API()

	// The circuit's all-Present layout must be the one the transcript
	// committed to.
	for _, cell := range st.Manifests {
		api.AssertIsEqualExt(run.GetCellValue(cell), api.ZeroExt())
	}

	cellValues := func(rows [][]*wiop.Cell) [][]circuit.Ext {
		res := make([][]circuit.Ext, len(rows))
		for k, row := range rows {
			for _, cell := range row {
				res[k] = append(res[k], run.GetCellValue(cell))
			}
		}
		return res
	}
	claims := make([]fri.GnarkBatchClaimedValues, len(st.Claims))
	for i, batch := range st.Claims {
		claims[i] = make(fri.GnarkBatchClaimedValues, len(batch))
		for s, sized := range batch {
			claims[i][s] = fri.GnarkSizedClaimedValues{Base: cellValues(sized.Base), Ext: cellValues(sized.Ext)}
		}
	}
	for _, dup := range st.Duplicates {
		api.AssertIsEqualExt(run.GetCellValue(dup[0]), run.GetCellValue(dup[1]))
	}

	// Every LagrangeEval opens at the same point. Sharing the same symbolic
	// expression is the common case and needs no constraint; a distinct
	// expression is constrained to evaluate to the same value.
	first := st.EvalPoints[0]
	zeta := run.EvaluateSingle(first, nil)
	for _, point := range st.EvalPoints[1:] {
		if point != first {
			api.AssertIsEqualExt(zeta, run.EvaluateSingle(point, nil))
		}
	}
	return claims, zeta
}

// allPresentManifests returns an all-Present manifest for every batch.
func allPresentManifests(batches []BatchRef) []ColumnManifest {
	out := make([]ColumnManifest, len(batches))
	for i, b := range batches {
		out[i] = allPresentManifest(len(b.Round.Columns))
	}
	return out
}

// OpeningProofShape implements [wiop.GnarkOpeningProofAllocator]: it returns
// an opening proof with the geometry of every honest proof of sys, derived from
// the System alone through [OpeningVerifierAction.Static] and
// [fri.VerifierPlan.OpeningProofShape].
//
// It panics on a system compiled with column elision: the manifests, and with
// them the batch shapes, are then chosen by the prover per proof.
func (a *OpeningVerifierAction) OpeningProofShape(sys *wiop.System) fri.OpeningProof {
	if a.c.elisionEnabled {
		panic("pcs: OpeningProofShape: all-Present manifests only: compile without WithColumnElision")
	}
	st, err := a.Static(sys)
	if err != nil {
		panic(fmt.Sprintf("pcs: OpeningProofShape: %v", err))
	}
	return st.Plan.OpeningProofShape()
}

var _ wiop.GnarkOpeningProofAllocator = (*OpeningVerifierAction)(nil)
