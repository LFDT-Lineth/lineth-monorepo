package vortex

import (
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

func (a *ExplicitPolynomialEval) RunGnark(api frontend.API, c wizard.GnarkRuntime) {
	a.gnarkExplicitPublicEvaluation(api, c) // Adjust based on context; see note below
}

func (ctx *VortexVerifierAction) RunGnark(api frontend.API, vr wizard.GnarkRuntime) {

	// The skip verification flag may be on, if the current vortex
	// context get self-recursed. In this case, the verifier does
	// not need to
	if ctx.IsSelfrecursed {
		return
	}

	// In non-Merkle mode, this is left as empty
	roots := []frontend.Variable{}

	// Append the precomputed roots when IsCommitToPrecomputed is true
	if ctx.IsNonEmptyPrecomputed() {
		precompRootSv := vr.GetColumn(ctx.Items.Precomputeds.MerkleRoot.GetColID()) // len 1 smart vector
		roots = append(roots, precompRootSv[0])
	}

	// Collect all the commitments : rounds by rounds
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		if ctx.RoundStatus[round] == IsEmpty {
			continue // skip the dry rounds
		}
		rootSv := vr.GetColumn(ctx.MerkleRootName(round)) // len 1 smart vector
		roots = append(roots, rootSv[0])
	}

	randomCoin := vr.GetRandomCoinField(ctx.LinCombRandCoinName())

	// Collect the linear combination
	proof := vortex.GProof{}
	proof.Rate = uint64(ctx.BlowUpFactor)
	proof.RsDomain = fft.NewDomain(uint64(ctx.NumEncodedCols()))
	proof.LinearCombination = vr.GetColumn(ctx.LinCombName())

	// Collect the random entry List and the random coin
	entryList := vr.GetRandomCoinIntegerVec(ctx.RandColSelectionName())

	// Collect the opened columns and split them "by-commitment-rounds"
	proof.Columns = ctx.GnarkRecoverSelectedColumns(api, vr)
	x := vr.GetUnivariateParams(ctx.Query.QueryID).X

	// function that will defer the hashing to gkr
	factoryHasherFunc := func(_ frontend.API) (hash.FieldHasher, error) {
		factory := vr.GetHasherFactory()
		if factory != nil {
			h := vr.GetHasherFactory().NewHasher()
			return h, nil
		}
		h, err := mimc.NewMiMC(api)
		return &h, err
	}

	packedMProofs := vr.GetColumn(ctx.MerkleProofName())
	proof.MerkleProofs = ctx.unpackMerkleProofsGnark(packedMProofs, entryList)

	// pass the parameters for a merkle-mode sis verification
	params := vortex.GParams{}
	params.HasherFunc = factoryHasherFunc
	params.NoSisHasher = factoryHasherFunc
	params.Key = ctx.VortexParams.Key

	vortex.GnarkVerifyOpeningWithMerkleProof(
		api,
		params,
		roots,
		proof,
		x,
		ctx.gnarkGetYs(api, vr),
		randomCoin,
		entryList,
	)

}

// returns the Ys as a vector
func (ctx *Ctx) gnarkGetYs(_ frontend.API, vr wizard.GnarkRuntime) (ys [][]frontend.Variable) {

	query := ctx.Query
	params := vr.GetUnivariateParams(ctx.Query.QueryID)

	// Build an index table to efficiently lookup an alleged
	// prover evaluation from its colID.
	ysMap := make(map[ifaces.ColID]frontend.Variable, len(params.Ys))
	for i := range query.Pols {
		ysMap[query.Pols[i].GetColID()] = params.Ys[i]
	}

	// Also add the shadow evaluations into ysMap. Since the shadow columns
	// are full-zeroes. We know that the evaluation will also always be zero.
	//
	// The sorting is necessary to ensure that the iteration below happens in
	// deterministic order over the [ShadowCols] map.
	shadowIDs := utils.SortedKeysOf(ctx.ShadowCols, func(a, b ifaces.ColID) bool {
		return a < b
	})

	for _, shadowID := range shadowIDs {
		ysMap[shadowID] = field.Zero()
	}

	ys = [][]frontend.Variable{}

	// add ys for precomputed when IsCommitToPrecomputed is true
	if ctx.IsNonEmptyPrecomputed() {
		names := make([]ifaces.ColID, len(ctx.Items.Precomputeds.PrecomputedColums))
		for i, poly := range ctx.Items.Precomputeds.PrecomputedColums {
			names[i] = poly.GetColID()
		}
		ysPrecomputed := make([]frontend.Variable, len(names))
		for i, name := range names {
			y, yFound := ysMap[name]
			if !yFound {
				utils.Panic("was not found: %v", name)
			}
			if y == nil {
				utils.Panic("found Y but it was nil: %v", name)
			}
			ysPrecomputed[i] = y
		}
		ys = append(ys, ysPrecomputed)
	}

	// Get the list of the polynomials
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		if ctx.RoundStatus[round] == IsEmpty {
			continue // skip the dry rounds
		}
		names := ctx.CommitmentsByRounds.MustGet(round)
		ysRounds := make([]frontend.Variable, len(names))
		for i, name := range names {
			y, yFound := ysMap[name]
			if !yFound {
				utils.Panic("was not found: %v", name)
			}

			if y == nil {
				utils.Panic("found Y but it was nil: %v", name)
			}

			ysRounds[i] = ysMap[name]
		}

		ys = append(ys, ysRounds)
	}

	return ys
}

// Returns the opened columns from the messages. The returned columns are
// split "by-commitment-round".
func (ctx *Ctx) GnarkRecoverSelectedColumns(api frontend.API, vr wizard.GnarkRuntime) [][][]frontend.Variable {

	// Collect the columns : first extract the full columns
	// Bear in mind that the prover messages are zero-padded
	fullSelectedCols := make([][]frontend.Variable, ctx.NbColsToOpen())
	for j := 0; j < ctx.NbColsToOpen(); j++ {
		fullSelectedCols[j] = vr.GetColumn(ctx.SelectedColName(j))
	}

	// Split the columns per commitment for the verification
	openedSubColumns := [][][]frontend.Variable{}
	roundStartAt := 0

	// Process precomputed
	if ctx.IsNonEmptyPrecomputed() {
		openedPrecompCols := make([][]frontend.Variable, ctx.NbColsToOpen())
		numPrecomputeds := len(ctx.Items.Precomputeds.PrecomputedColums)
		for j := 0; j < ctx.NbColsToOpen(); j++ {
			openedPrecompCols[j] = fullSelectedCols[j][roundStartAt : roundStartAt+numPrecomputeds]
		}
		// update the start counter to ensure we do not pass twice the same row
		roundStartAt += numPrecomputeds
		openedSubColumns = append(openedSubColumns, openedPrecompCols)

	}

	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		if ctx.RoundStatus[round] == IsEmpty {
			continue // skip the dry rounds
		}
		openedSubColumnsForRound := make([][]frontend.Variable, ctx.NbColsToOpen())
		numRowsForRound := ctx.getNbCommittedRows(round)
		for j := 0; j < ctx.NbColsToOpen(); j++ {
			openedSubColumnsForRound[j] = fullSelectedCols[j][roundStartAt : roundStartAt+numRowsForRound]
		}

		// update the start counter to ensure we do not pass twice the same row
		roundStartAt += numRowsForRound
		openedSubColumns = append(openedSubColumns, openedSubColumnsForRound)
	}

	// sanity-check : make sure we have not forgotten any column
	if roundStartAt != ctx.CommittedRowsCount {
		utils.Panic("we have a mistmatch in the row count : %v != %v", roundStartAt, ctx.CommittedRowsCount)
	}

	return openedSubColumns
}

// Evaluates explicitly the public polynomials (proof, vk, public inputs)
func (ctx *Ctx) gnarkExplicitPublicEvaluation(api frontend.API, vr wizard.GnarkRuntime) {

	var (
		params     = vr.GetUnivariateParams(ctx.Query.QueryID)
		polys      = make([][]frontend.Variable, 0)
		expectedYs = make([]frontend.Variable, 0)
	)

	for i, pol := range ctx.Query.Pols {

		// If the column is a VerifierDefined column, then it is directly
		// concerned by direct verification but we cannot access its status.
		// status so we need a hierarchical check to make sure we can access
		// its status.
		if _, isVerifierCol := pol.(verifiercol.VerifierCol); !isVerifierCol {
			status := ctx.Comp.Columns.Status(pol.GetColID())
			if !status.IsPublic() {
				// then, its not concerned by direct evaluation because the
				// evaluation is implicitly checked by the invokation of the
				// Vortex protocol.
				continue
			}
		}

		polys = append(polys, pol.GetColAssignmentGnark(vr))
		expectedYs = append(expectedYs, params.Ys[i])
	}

	ys := fastpoly.BatchInterpolateGnark(api, polys, params.X)

	for i := range polys {
		api.AssertIsEqual(ys[i], expectedYs[i])
	}
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackMerkleProofsGnark(sv []frontend.Variable, entryList []frontend.Variable) (proofs [][]smt.GnarkProof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs += 1
	}

	numEntries := len(entryList)

	proofs = make([][]smt.GnarkProof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.
	for i := range proofs {
		proofs[i] = make([]smt.GnarkProof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt.GnarkProof{
				Path:     entryList[j],
				Siblings: make([]frontend.Variable, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {
				proof.Siblings[depth-k-1] = sv[curr]
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}
