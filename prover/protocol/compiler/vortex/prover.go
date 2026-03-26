package vortex

import (
	"github.com/consensys/linea-monorepo/prover/crypto/encoding"
	"github.com/consensys/linea-monorepo/prover/utils/types"

	bls12377 "github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	gnarkvortex "github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	vortex_bls12377 "github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex/vortex_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

type commitmentMode int

const (
	// Denotes the Vortex mode when we don't apply
	// self recursion
	NonSelfRecursion commitmentMode = iota
	// Denotes the Vortex mode when we apply
	// self recursion and commit using SIS
	SelfRecursionSIS
	// Denotes the Vortex mode when we apply
	// self recursion and commit using only Poseidon2
	SelfRecursionPoseidon2Only
)

// ReassignPrecomputedRootAction is a [wizard.ProverAction] that assigns the
// precomputed Merkle root of the Vortex invokation. The action is defined
// for round 0 only and only if the AddPrecomputedMerkleRootToPublicInputsOpt
// is enabled.
type ReassignPrecomputedRootAction struct {
	*Ctx
}

func (r ReassignPrecomputedRootAction) Run(run *wizard.ProverRuntime) {
	if r.IsBLS {
		for i := 0; i < encoding.KoalabearChunks; i++ {
			run.AssignColumn(
				r.Items.Precomputeds.MerkleRoot[i].GetColID(),
				smartvectors.NewConstant(r.AddPrecomputedMerkleRootToPublicInputsOpt.PrecomputedBLSValue[i], 1),
			)
		}
	} else {
		for i := 0; i < blockSize; i++ {
			run.AssignColumn(
				r.Items.Precomputeds.MerkleRoot[i].GetColID(),
				smartvectors.NewConstant(r.AddPrecomputedMerkleRootToPublicInputsOpt.PrecomputedValue[i], 1),
			)
		}
	}

}

// ColumnAssignmentProverAction is a [wizard.ProverAction] that assigns the
// the columns at a given round.
type ColumnAssignmentProverAction struct {
	*Ctx
	Round int
}

// Prover steps of Vortex that is run in place of committing to polynomials
func (ctx *ColumnAssignmentProverAction) Run(run *wizard.ProverRuntime) {

	round := ctx.Round

	// Check if that is a dry round
	if ctx.RoundStatus[round] == IsEmpty {
		// Nothing special to do.
		return
	}

	var (
		committedMatrix vortex_bls12377.EncodedMatrix
		sisColHashes    []field.Element // column hashes generated from SisTransversalHash
		noSisColHashes  []field.Element // column hashes generated from noSisTransversalHash, using LeafHashFunc
	)

	pols := ctx.getPols(run, round)

	// If there are no polynomials to commit to, we don't need to do anything
	if len(pols) == 0 {
		logrus.Infof("Vortex AssignColumn at round %v: No polynomials to commit to", round)
		return
	}

	// We commit to the polynomials with SIS hashing if the number of polynomials
	// is greater than the [ApplyToSISThreshold].

	if ctx.IsBLS {
		var (
			tree      *smt_bls12377.Tree
			colHashes []bls12377.Element
		)
		committedMatrix, _, tree, colHashes = ctx.VortexBLSParams.CommitMerkleWithoutSIS(pols)

		run.State.InsertNew(ctx.VortexProverStateName(round), committedMatrix)
		run.State.InsertNew(ctx.MerkleTreeName(round), tree)

		if ctx.IsSelfrecursed {
			// We need to store the SIS and non-SIS column hashes in the prover state
			// so that we can use them in the self-recursion compiler.
			if ctx.RoundStatus[round] == IsNoSis {
				run.State.InsertNew(ctx.NoSisHashName(round), colHashes)
			}
		}
		roots := encoding.EncodeBLS12RootToKoalabear(tree.Root)

		for i := 0; i < encoding.KoalabearChunks; i++ {
			run.AssignColumn(ifaces.ColID(ctx.MerkleRootName(round, i)), smartvectors.NewConstant(roots[i], 1))
		}
	} else {
		var tree *smt_koalabear.Tree

		if ctx.RoundStatus[round] == IsNoSis {
			committedMatrix, _, tree, noSisColHashes = ctx.VortexKoalaParams.CommitMerkleWithoutSIS(pols)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedMatrix, _, tree, sisColHashes = ctx.VortexKoalaParams.CommitMerkleWithSIS(pols)
		}

		run.State.InsertNew(ctx.VortexProverStateName(round), committedMatrix)
		run.State.InsertNew(ctx.MerkleTreeName(round), tree)

		// Only to be read by the self-recursion compiler.
		if ctx.IsSelfrecursed {
			// We need to store the SIS and non-SIS column hashes in the prover state
			// so that we can use them in the self-recursion compiler.
			if ctx.RoundStatus[round] == IsNoSis {
				run.State.InsertNew(ctx.NoSisHashName(round), noSisColHashes)
			} else if ctx.RoundStatus[round] == IsSISApplied {
				run.State.InsertNew(ctx.SisHashName(round), sisColHashes)
			}
		}
		for i := 0; i < blockSize; i++ {
			run.AssignColumn(ifaces.ColID(ctx.MerkleRootName(round, i)), smartvectors.NewConstant(tree.Root[i], 1))
		}
	}

}

type LinearCombinationComputationProverAction struct {
	*Ctx
}

// Prover steps of Vortex that is run when committing to the linear combination
// We stack the No SIS round matrices before the SIS round matrices in the committed matrix stack.
// For the precomputed matrix, we stack it on top of the SIS round matrices if SIS is used on it or
// we stack it on top of the No SIS round matrices if SIS is not used on it.
func (ctx *LinearCombinationComputationProverAction) Run(pr *wizard.ProverRuntime) {
	var (
		committedSVSIS   = []smartvectors.SmartVector{}
		committedSVNoSIS = []smartvectors.SmartVector{}
	)
	// Add the precomputed columns
	if ctx.IsNonEmptyPrecomputed() {
		var precomputedSV = []smartvectors.SmartVector{}
		precomputedSV = append(precomputedSV, ctx.Items.Precomputeds.CommittedMatrix...)

		// Add the precomputed columns to commitedSVSIS or commitedSVNoSIS
		if ctx.IsSISAppliedToPrecomputed() {
			committedSVSIS = append(committedSVSIS, precomputedSV...)
		} else {
			committedSVNoSIS = append(committedSVNoSIS, precomputedSV...)
		}
	}

	// Collect all the committed polynomials : round by round
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to compute their linear combination.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}

		committedMatrix := pr.State.MustGet(ctx.VortexProverStateName(round)).(vortex_bls12377.EncodedMatrix)

		// Push pols to the right stack
		if ctx.RoundStatus[round] == IsNoSis {
			committedSVNoSIS = append(committedSVNoSIS, committedMatrix...)

		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedSVSIS = append(committedSVSIS, committedMatrix...)
		}
	}
	// Construct committedSV by stacking the No SIS round
	// matrices before the SIS round matrices
	committedSV := append(committedSVNoSIS, committedSVSIS...)

	// And get the randomness
	randomCoinLC := pr.GetRandomCoinFieldExt(ctx.Items.Alpha.Name)

	// and compute and assign the random linear combination of the rows
	proof := &vortex.OpeningProof{}
	vortex.LinearCombination(proof, committedSV, randomCoinLC)
	pr.AssignColumn(ctx.Items.Ualpha.GetColID(), proof.LinearCombination)

}

// ComputeLinearCombFromRsMatrix is the same as ComputeLinearComb but uses
// the RS encoded matrix instead of using the basic one. It is slower than
// the later but is recommended.
func (ctx *Ctx) ComputeLinearCombFromRsMatrix(run *wizard.ProverRuntime) {

	var (
		committedSVSIS   = []smartvectors.SmartVector{}
		committedSVNoSIS = []smartvectors.SmartVector{}
	)

	// Add the precomputed columns to commitedSVSIS or commitedSVNoSIS
	if ctx.IsSISAppliedToPrecomputed() {
		committedSVSIS = append(committedSVSIS, ctx.Items.Precomputeds.CommittedMatrix...)
	} else {
		committedSVNoSIS = append(committedSVNoSIS, ctx.Items.Precomputeds.CommittedMatrix...)
	}

	// Collect all the committed polynomials : round by round
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to proceed.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}

		committedMatrix := run.State.MustGet(ctx.VortexProverStateName(round)).(vortex_koalabear.EncodedMatrix)

		// Push pols to the right stack
		if ctx.RoundStatus[round] == IsNoSis {
			committedSVNoSIS = append(committedSVNoSIS, committedMatrix...)
		} else if ctx.RoundStatus[round] == IsSISApplied {
			committedSVSIS = append(committedSVSIS, committedMatrix...)
		}
	}

	// Construct committedSV by stacking the No SIS round
	// matrices before the SIS round matrices
	committedSV := append(committedSVNoSIS, committedSVSIS...)

	// And get the randomness
	randomCoinLC := run.GetRandomCoinFieldExt(ctx.Items.Alpha.Name)

	// and compute and assign the random linear combination of the rows
	proof := &vortex.OpeningProof{}
	vortex.LinearCombination(proof, committedSV, randomCoinLC)

	run.AssignColumn(ctx.Items.Ualpha.GetColID(), proof.LinearCombination)
}

// Prover steps of Vortex where he opens the columns selected by the verifier
// We stack the no SIS round matrices before the SIS round matrices in the committed matrix stack.
// The same is done for the tree.
type OpenSelectedColumnsProverAction struct {
	*Ctx
}

func (ctx *OpenSelectedColumnsProverAction) Run(run *wizard.ProverRuntime) {

	var (
		committedMatricesSIS   = []vortex_bls12377.EncodedMatrix{}
		committedMatricesNoSIS = []vortex_bls12377.EncodedMatrix{}
		treesSIS               = []*smt_koalabear.Tree{}
		treesNoSIS             = []*smt_koalabear.Tree{}
		blsTrees               = []*smt_bls12377.Tree{}
		// We need them to assign the opened sis and non sis columns
		// to be used in the self-recursion compiler
		sisProof    = vortex.OpeningProof{}
		nonSisProof = vortex.OpeningProof{}
	)

	// Append the precomputed committedMatrices and trees to the SIS or no SIS matrices
	// or trees as per the number of precomputed columns are more than the [ApplyToSISThreshold]
	if ctx.IsNonEmptyPrecomputed() {
		if ctx.IsSISAppliedToPrecomputed() {
			committedMatricesSIS = append(committedMatricesSIS, ctx.Items.Precomputeds.CommittedMatrix)
			treesSIS = append(treesSIS, ctx.Items.Precomputeds.Tree)
		} else {
			if ctx.IsBLS {
				committedMatricesNoSIS = append(committedMatricesNoSIS, ctx.Items.Precomputeds.CommittedMatrix)
				blsTrees = append(blsTrees, ctx.Items.Precomputeds.BLSTree)
			} else {
				committedMatricesNoSIS = append(committedMatricesNoSIS, ctx.Items.Precomputeds.CommittedMatrix)
				treesNoSIS = append(treesNoSIS, ctx.Items.Precomputeds.Tree)
			}

		}
	}

	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		// There are not included in the commitments so there
		// is no need to proceed.
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		// Fetch it from the state
		committedMatrix := run.State.MustGet(ctx.VortexProverStateName(round)).(vortex_koalabear.EncodedMatrix)
		// and delete it because it won't be needed anymore and its very heavy
		run.State.Del(ctx.VortexProverStateName(round))

		// Also fetches the trees from the prover state
		if ctx.IsBLS {
			tree := run.State.MustGet(ctx.MerkleTreeName(round)).(*smt_bls12377.Tree)
			// conditionally stack the matrix and tree
			// to SIS or no SIS matrices and trees

			if ctx.RoundStatus[round] == IsNoSis {
				committedMatricesNoSIS = append(committedMatricesNoSIS, committedMatrix)
				blsTrees = append(blsTrees, tree)
			} else if ctx.RoundStatus[round] == IsSISApplied {
				utils.Panic("IsSISApplied is not supported in BLS mode (round %v)", round)
			}

		} else {
			tree := run.State.MustGet(ctx.MerkleTreeName(round)).(*smt_koalabear.Tree)
			// conditionally stack the matrix and tree
			// to SIS or no SIS matrices and trees
			if ctx.RoundStatus[round] == IsNoSis {
				committedMatricesNoSIS = append(committedMatricesNoSIS, committedMatrix)
				treesNoSIS = append(treesNoSIS, tree)
			} else if ctx.RoundStatus[round] == IsSISApplied {
				committedMatricesSIS = append(committedMatricesSIS, committedMatrix)
				treesSIS = append(treesSIS, tree)
			}
		}

	}

	// Free original committed columns from run.Columns — their data has been
	// encoded into the Vortex matrices and is no longer needed in raw form.
	for round := 0; round <= ctx.MaxCommittedRound; round++ {
		if ctx.RoundStatus[round] == IsEmpty {
			continue
		}
		for _, colName := range ctx.CommitmentsByRounds.MustGet(round) {
			run.Columns.TryDel(colName)
		}
	}

	// Stack the no SIS matrices and trees before the SIS matrices and trees
	committedMatrices := append(committedMatricesNoSIS, committedMatricesSIS...)
	trees := append(treesNoSIS, treesSIS...)

	entryList := run.GetRandomCoinIntegerVec(ctx.Items.Q.Name)
	proof := vortex.OpeningProof{}

	// Amend the Vortex proof with the Merkle proofs and registers
	// the Merkle proofs in the prover runtime

	if ctx.IsBLS {
		merkleProofs := vortex_bls12377.SelectColumnsAndMerkleProofs(&proof, entryList, committedMatrices, blsTrees)

		packedMProofs := ctx.packBLSMerkleProofs(merkleProofs)

		for i := range ctx.Items.BLSMerkleProofs {
			run.AssignColumn(ctx.Items.BLSMerkleProofs[i].GetColID(), packedMProofs[i])
		}

	} else {

		merkleProofs := vortex_koalabear.SelectColumnsAndMerkleProofs(&proof, entryList, committedMatrices, trees)

		packedMProofs := ctx.packMerkleProofs(merkleProofs)

		for i := range ctx.Items.MerkleProofs {
			run.AssignColumn(ctx.Items.MerkleProofs[i].GetColID(), packedMProofs[i])
		}

	}
	selectedCols := proof.Columns

	// Assign the opened columns
	ctx.assignOpenedColumns(run, entryList, selectedCols, NonSelfRecursion)

	// Assign the SIS and non SIS selected columns.
	// They are not used in the Vortex compilers,
	// but are used in the self-recursion compilers.
	// But we need to assign them anyway as the self-recursion
	// compiler always runs after running the Vortex compiler

	// Handle SIS round
	if len(committedMatricesSIS) > 0 {
		vortex_koalabear.SelectColumnsAndMerkleProofs(&sisProof, entryList, committedMatricesSIS, treesSIS)
		sisSelectedCols := sisProof.Columns
		// Assign the opened columns
		ctx.assignOpenedColumns(run, entryList, sisSelectedCols, SelfRecursionSIS)
	}
	// Handle non SIS round
	if len(committedMatricesNoSIS) > 0 {
		vortex_koalabear.SelectColumnsAndMerkleProofs(&nonSisProof, entryList, committedMatricesNoSIS, treesNoSIS)
		nonSisSelectedCols := nonSisProof.Columns
		ctx.assignOpenedColumns(run, entryList, nonSisSelectedCols, SelfRecursionPoseidon2Only)
		// Store the selected columns for the non sis round
		//  in the prover state
		ctx.storeSelectedColumnsForNonSisRounds(run, nonSisSelectedCols)
	}
}

// returns the list of all committed smartvectors for the given round
// so that we can commit to them
func (ctx *Ctx) getPols(run *wizard.ProverRuntime, round int) (pols []smartvectors.SmartVector) {
	names := ctx.CommitmentsByRounds.MustGet(round)
	pols = make([]smartvectors.SmartVector, len(names))
	for i := range names {
		pols[i] = run.Columns.MustGet(names[i])
	}
	return pols
}

// pack a list of merkle-proofs in a vector as used in the merkle proof module
func (ctx *Ctx) packMerkleProofs(proofs [][]smt_koalabear.Proof) [8]smartvectors.SmartVector {

	depth := len(proofs[0][0].Siblings) // depth of the Merkle-tree
	res := [8][]field.Element{}
	for i := range res {
		res[i] = make([]field.Element, ctx.MerkleProofSize())
	}
	numProofWritten := 0

	// Sanity-checks

	if depth != utils.Log2Ceil(ctx.NumEncodedCols()) {
		utils.Panic(
			"expected depth to be equal to Log2(NumEncodedCols()), got %v, %v",
			depth, utils.Log2Ceil(ctx.NumEncodedCols()),
		)
	}

	// When we commit to the precomputeds, len(proofs) = ctx.NumCommittedRounds + 1,
	// otherwise len(proofs) = ctx.NumCommittedRounds
	if len(proofs) != ctx.NumCommittedRounds() && !ctx.IsNonEmptyPrecomputed() {
		utils.Panic(
			"inconsitent proofs length %v, %v",
			len(proofs), ctx.NumCommittedRounds(),
		)
	}

	if len(proofs[0]) != ctx.NbColsToOpen() {
		utils.Panic(
			"expected proofs[0] and NbColsToOpen to be equal: %v, %v",
			len(proofs[0]), ctx.NbColsToOpen(),
		)
	}

	for i := range proofs {
		for j := range proofs[i] {
			p := proofs[i][j]
			for k := range p.Siblings {
				// The proof stores the sibling bottom-up but we want to pack
				// the proof in top-down order.
				hashOct := p.Siblings[depth-1-k]
				for coord := range res {
					res[coord][numProofWritten*depth+k] = hashOct[coord]
				}
			}
			numProofWritten++
		}
	}

	resSV := [8]smartvectors.SmartVector{}
	for i := range res {
		resSV[i] = smartvectors.NewRegular(res[i])
	}

	return resSV
}

// pack a list of merkle-proofs in a vector as used in the merkle proof module
func (ctx *Ctx) packBLSMerkleProofs(proofs [][]smt_bls12377.Proof) [encoding.KoalabearChunks]smartvectors.SmartVector {

	depth := len(proofs[0][0].Siblings) // depth of the Merkle-tree
	res := [encoding.KoalabearChunks][]field.Element{}
	for i := range res {
		res[i] = make([]field.Element, ctx.MerkleProofSize())
	}

	numProofWritten := 0

	// Sanity-checks
	if depth != utils.Log2Ceil(ctx.NumEncodedCols()) {
		utils.Panic(
			"expected depth to be equal to Log2(NumEncodedCols()), got %v, %v",
			depth, utils.Log2Ceil(ctx.NumEncodedCols()),
		)
	}

	// When we commit to the precomputeds, len(proofs) = ctx.NumCommittedRounds + 1,
	// otherwise len(proofs) = ctx.NumCommittedRounds
	if len(proofs) != ctx.NumCommittedRounds() && !ctx.IsNonEmptyPrecomputed() {
		utils.Panic(
			"inconsitent proofs length %v, %v",
			len(proofs), ctx.NumCommittedRounds(),
		)
	}

	if len(proofs) != (ctx.NumCommittedRounds()+1) && ctx.IsNonEmptyPrecomputed() {
		utils.Panic(
			"inconsitent proofs length %v, %v",
			len(proofs), ctx.NumCommittedRounds()+1,
		)
	}

	if len(proofs[0]) != ctx.NbColsToOpen() {
		utils.Panic(
			"expected proofs[0] and NbColsToOpen to be equal: %v, %v",
			len(proofs[0]), ctx.NbColsToOpen(),
		)
	}

	for i := range proofs {
		for j := range proofs[i] {
			p := proofs[i][j]
			for k := range p.Siblings {
				// The proof stores the sibling bottom-up but
				// we want to pack the proof in top-down order.
				koalaElems := encoding.EncodeBLS12RootToKoalabear(p.Siblings[depth-1-k])

				for coord := range res {
					res[coord][numProofWritten*depth+k] = koalaElems[coord]
				}
			}
			numProofWritten++
		}
	}

	// return smartvectors.NewRegular(res)
	resSV := [encoding.KoalabearChunks]smartvectors.SmartVector{}
	for i := range res {
		resSV[i] = smartvectors.NewRegular(res[i])
	}

	return resSV
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackMerkleProofs(sv [8]smartvectors.SmartVector, entryList []int) (proofs [][]smt_koalabear.Proof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs = ctx.NumCommittedRounds() + 1 // Need to consider the precomputed commitments
	}
	numEntries := len(entryList)

	proofs = make([][]smt_koalabear.Proof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.

	for i := range proofs {
		proofs[i] = make([]smt_koalabear.Proof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt_koalabear.Proof{
				Path:     entryList[j],
				Siblings: make([]types.KoalaOctuplet, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {
				var v gnarkvortex.Hash
				for coord := 0; coord < len(v); coord++ {
					v[coord] = sv[coord].Get(curr)
				}
				proof.Siblings[depth-k-1] = v
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}

// unpack a list of merkle proofs from a vector as in
func (ctx *Ctx) unpackBLSMerkleProofs(sv [encoding.KoalabearChunks]smartvectors.SmartVector, entryList []int) (proofs [][]smt_bls12377.Proof) {

	depth := utils.Log2Ceil(ctx.NumEncodedCols()) // depth of the Merkle-tree
	numComs := ctx.NumCommittedRounds()
	if ctx.IsNonEmptyPrecomputed() {
		numComs = ctx.NumCommittedRounds() + 1 // Need to consider the precomputed commitments
	}
	numEntries := len(entryList)

	proofs = make([][]smt_bls12377.Proof, numComs)
	curr := 0 // tracks the position in sv that we are parsing.

	for i := range proofs {
		proofs[i] = make([]smt_bls12377.Proof, numEntries)
		for j := range proofs[i] {
			// initialize the proof that we are parsing
			proof := smt_bls12377.Proof{
				Path:     entryList[j],
				Siblings: make([]bls12377.Element, depth),
			}

			// parse the siblings accounting for the fact that we
			// are inversing the order.
			for k := range proof.Siblings {
				var v [encoding.KoalabearChunks]field.Element
				for coord := 0; coord < len(v); coord++ {
					v[coord] = sv[coord].Get(curr)
				}
				proof.Siblings[depth-k-1] = encoding.DecodeKoalabearToBLS12Root(v)
				curr++
			}

			proofs[i][j] = proof
		}
	}
	return proofs
}

// assignOpenedColumns assign the opened columns for
// both normal and self-recursion compilers
func (ctx *Ctx) assignOpenedColumns(
	pr *wizard.ProverRuntime,
	entryList []int,
	selectedCols [][][]field.Element,
	mode commitmentMode) {
	// The columns are split by commitment round. So we need to
	// restick them when we commit them.
	for j := range entryList {
		fullCol := []field.Element{}
		for i := range selectedCols {
			fullCol = append(fullCol, selectedCols[i][j]...)
		}

		// Converts it into a smart-vector and zero-pad it if necessary
		var assignable smartvectors.SmartVector = smartvectors.NewRegular(fullCol)
		if assignable.Len() < utils.NextPowerOfTwo(len(fullCol)) {
			assignable = smartvectors.RightZeroPadded(fullCol, utils.NextPowerOfTwo(len(fullCol)))
		}
		if mode == NonSelfRecursion {
			pr.AssignColumn(ctx.Items.OpenedColumns[j].GetColID(), assignable)
		} else if mode == SelfRecursionSIS {
			pr.AssignColumn(ctx.Items.OpenedSISColumns[j].GetColID(), assignable)
		} else if mode == SelfRecursionPoseidon2Only {
			pr.AssignColumn(ctx.Items.OpenedNonSISColumns[j].GetColID(), assignable)
		}
	}

}

// storeSelectedColumnsForNonSisRound stores the selected columns in the prover state
// for the non SIS rounds which is to be used in the self-recursion compilers
func (ctx *Ctx) storeSelectedColumnsForNonSisRounds(
	pr *wizard.ProverRuntime,
	selectedCols [][][]field.Element) {
	numNonSisRound := ctx.NumCommittedRoundsNoSis()
	if ctx.IsNonEmptyPrecomputed() && !ctx.IsSISAppliedToPrecomputed() {
		numNonSisRound++
	}
	// selectedColsQ[i][j][k] stores the jth selected
	// column of the ith non SIS round
	selectedColsQ := make([][][]field.Element, numNonSisRound)
	// Sanity check
	if len(selectedCols) != numNonSisRound {
		utils.Panic(
			"expected selectedCols to be of length %v, got %v",
			numNonSisRound, len(selectedCols),
		)
	}
	for i := range selectedCols {
		// Sanity check
		if len(selectedCols[i]) != ctx.NbColsToOpen() {
			utils.Panic(
				"expected selectedCols[%v] to be of length %v, got %v",
				i, ctx.NbColsToOpen(), len(selectedCols[i]),
			)
		}
		selectedColsQ[i] = make([][]field.Element, ctx.NbColsToOpen())
		for j := range selectedCols[i] {
			selectedColsQ[i][j] = make([]field.Element, len(selectedCols[i][j]))
			copy(selectedColsQ[i][j], selectedCols[i][j])
		}
	}
	// Store the selected columns in the prover state
	pr.State.InsertNew(
		ctx.SelectedColumnNonSISName(),
		selectedColsQ)
}
