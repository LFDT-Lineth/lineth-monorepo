package vortex

import (
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	vortex_bls12377 "github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_koalabear"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	crypto_vortex "github.com/consensys/linea-monorepo/prover/crypto/vortex"

	"github.com/consensys/linea-monorepo/prover/maths/common/fastpoly"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
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
	blsRoots := []frontend.Variable{}
	koalaRoots := []poseidon2_koalabear.GnarkOctuplet{}

	// Append the precomputed roots when IsCommitToPrecomputed is true

	if ctx.IsNonEmptyPrecomputed() {
		if ctx.IsBLS {
			preRoots := [encoding.KoalabearChunks]koalagnark.Element{}

			for i := 0; i < encoding.KoalabearChunks; i++ {
				precompRootSv := vr.GetColumn(ctx.Items.Precomputeds.BLSMerkleRoot[i].GetColID())
				preRoots[i] = precompRootSv[0]
			}

			blsRoots = append(blsRoots, encoding.Encode9WVsToFV(api, preRoots))
		} else {
			preRoots := poseidon2_koalabear.GnarkOctuplet{}

			for i := 0; i < poseidon2_koalabear.BlockSize; i++ {
				precompRootSv := vr.GetColumn(ctx.Items.Precomputeds.MerkleRoot[i].GetColID())
				preRoots[i] = precompRootSv[0].Native()
			}

			koalaRoots = append(koalaRoots, preRoots)
		}
	}

	// Collect all the commitments : rounds by rounds
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		if ctx.RoundStatus[round] == IsEmpty {
			continue // skip the dry rounds
		}
		if ctx.IsBLS {
			preRoots := [encoding.KoalabearChunks]koalagnark.Element{}

			for i := 0; i < encoding.KoalabearChunks; i++ {
				rootSv := vr.GetColumn(ctx.MerkleRootName(round, i))
				preRoots[i] = rootSv[0]
			}
			blsRoots = append(blsRoots, encoding.Encode9WVsToFV(api, preRoots))
		} else {
			preRoots := poseidon2_koalabear.GnarkOctuplet{}

			for i := 0; i < poseidon2_koalabear.BlockSize; i++ {
				rootSv := vr.GetColumn(ctx.MerkleRootName(round, i))
				preRoots[i] = rootSv[0].Native()
			}
			koalaRoots = append(koalaRoots, preRoots)
		}
	}

	randomCoin := vr.GetRandomCoinFieldExt(ctx.LinCombRandCoinName())

	// Collect the linear combination
	proof := crypto_vortex.GnarkProof{}
	proof.LinearCombination = vr.GetColumnExt(ctx.LinCombName())

	// Collect the random entry List and the random coin
	entryList := vr.GetRandomCoinIntegerVec(ctx.RandColSelectionName())

	// Collect the opened columns and split them "by-commitment-rounds"
	proof.Columns = ctx.GnarkRecoverSelectedColumns(api, vr)
	x := vr.GetUnivariateParams(ctx.Query.QueryID).ExtX

	blsMerkleProofs := []([]smt_bls12377.GnarkProof){}
	koalaMerkleProofs := []([]smt_koalabear.GnarkProof){}

	if ctx.IsBLS {
		packedMProofs := [encoding.KoalabearChunks][]koalagnark.Element{}
		for i := range packedMProofs {
			packedMProofs[i] = vr.GetColumn(ctx.MerkleProofName(i))
		}

		blsMerkleProofs = ctx.unpackBLSMerkleProofsGnark(api, packedMProofs, entryList) // TODO@yao: check if this is BLS or Koala
	} else {
		packedMProofs := [poseidon2_koalabear.BlockSize][]koalagnark.Element{}
		for i := range packedMProofs {
			packedMProofs[i] = vr.GetColumn(ctx.MerkleProofName(i))
		}

		koalaMerkleProofs = ctx.unpackKoalaMerkleProofsGnark(packedMProofs, entryList)
	}
	Vi := crypto_vortex.GnarkVerifierInput{}
	Vi.Alpha = randomCoin
	Vi.X = x
	Vi.EntryList = make([]frontend.Variable, len(entryList))

	for i := 0; i < len(entryList); i++ {
		Vi.EntryList[i] = entryList[i].Native()
	}
	Vi.Ys = ctx.gnarkGetYs(api, vr)

	if ctx.IsBLS {
		crypto_vortex.GnarkVerify(api, vr.Fs(), ctx.VortexBLSParams.Params, proof, Vi)
		vortex_bls12377.GnarkCheckColumnInclusionNoSis(api, proof.Columns, blsMerkleProofs, blsRoots)
	} else {
		crypto_vortex.GnarkVerify(api, vr.Fs(), ctx.VortexKoalaParams.Params, proof, Vi)
		vortex_koalabear.GnarkCheckColumnInclusionNoSis(api, proof.Columns, koalaMerkleProofs, koalaRoots)
	}
}

// returns the Ys as a vector
func (ctx *Ctx) gnarkGetYs(_ frontend.API, vr wizard.GnarkRuntime) (ys [][]koalagnark.Ext) {

	query := ctx.Query
	params := vr.GetUnivariateParams(ctx.Query.QueryID)
	zeroExt := koalagnark.NewExt(fext.Zero())

	// Build an index table to efficiently lookup an alleged
	// prover evaluation from its colID.
	ysMap := make(map[ifaces.ColID]koalagnark.Ext, len(params.ExtYs))
	for i := range query.Pols {
		ysMap[query.Pols[i].GetColID()] = params.ExtYs[i]
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
		ysMap[shadowID] = zeroExt
	}

	ys = [][]koalagnark.Ext{}

	// add ys for precomputed when IsCommitToPrecomputed is true
	if ctx.IsNonEmptyPrecomputed() {
		names := make([]ifaces.ColID, len(ctx.Items.Precomputeds.PrecomputedColums))
		for i, poly := range ctx.Items.Precomputeds.PrecomputedColums {
			names[i] = poly.GetColID()
		}
		ysPrecomputed := make([]koalagnark.Ext, len(names))
		for i, name := range names {
			y, yFound := ysMap[name]
			if !yFound {
				utils.Panic("was not found: %v", name)
			}
			if y.B0.A0.IsEmpty() {
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
		ysRounds := make([]koalagnark.Ext, len(names))
		for i, name := range names {
			y, yFound := ysMap[name]
			if !yFound {
				utils.Panic("was not found: %v", name)
			}

			if y.B0.A0.IsEmpty() {
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
func (ctx *Ctx) GnarkRecoverSelectedColumns(api frontend.API, vr wizard.GnarkRuntime) [][][]koalagnark.Element {

	// Collect the columns : first extract the full columns
	// Bear in mind that the prover messages are zero-padded
	fullSelectedCols := make([][]koalagnark.Element, ctx.NbColsToOpen())
	for j := 0; j < ctx.NbColsToOpen(); j++ {
		fullSelectedCols[j] = vr.GetColumn(ctx.SelectedColName(j))
	}

	// Split the columns per commitment for the verification
	openedSubColumns := [][][]koalagnark.Element{}
	roundStartAt := 0

	// Process precomputed
	if ctx.IsNonEmptyPrecomputed() {
		openedPrecompCols := make([][]koalagnark.Element, ctx.NbColsToOpen())
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
		openedSubColumnsForRound := make([][]koalagnark.Element, ctx.NbColsToOpen())
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
		polys      = make([][]koalagnark.Element, 0)
		expectedYs = make([]koalagnark.Ext, 0)
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
		expectedYs = append(expectedYs, params.ExtYs[i])
	}

	ys := fastpoly.BatchEvaluateLagrangeGnarkMixed(api, polys, params.ExtX)

	koalaAPI := koalagnark.NewAPI(api)
	for i := range expectedYs {
		koalaAPI.AssertIsEqualExt(ys[i], expectedYs[i])
	}
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackBLSMerkleProofsGnark(api frontend.API, sv [encoding.KoalabearChunks][]koalagnark.Element, entryList []koalagnark.Element) (proofs [][]smt_bls12377.GnarkProof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs += 1
	}

	numEntries := len(entryList)

	proofs = make([][]smt_bls12377.GnarkProof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.
	for i := range proofs {
		proofs[i] = make([]smt_bls12377.GnarkProof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt_bls12377.GnarkProof{
				Path:     entryList[j].Native(),
				Siblings: make([]frontend.Variable, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {

				var v [encoding.KoalabearChunks]koalagnark.Element
				for coord := 0; coord < encoding.KoalabearChunks; coord++ {
					v[coord] = sv[coord][curr]
				}
				proof.Siblings[depth-k-1] = encoding.Encode9WVsToFV(api, v)
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackKoalaMerkleProofsGnark(sv [poseidon2_koalabear.BlockSize][]koalagnark.Element, entryList []koalagnark.Element) (proofs [][]smt_koalabear.GnarkProof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs += 1
	}

	numEntries := len(entryList)

	proofs = make([][]smt_koalabear.GnarkProof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.
	for i := range proofs {
		proofs[i] = make([]smt_koalabear.GnarkProof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt_koalabear.GnarkProof{
				Path:     entryList[j].Native(),
				Siblings: make([]poseidon2_koalabear.GnarkOctuplet, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {

				var v poseidon2_koalabear.GnarkOctuplet
				for coord := 0; coord < poseidon2_koalabear.BlockSize; coord++ {
					v[coord] = sv[coord][curr].Native()
				}
				proof.Siblings[depth-k-1] = v
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}
