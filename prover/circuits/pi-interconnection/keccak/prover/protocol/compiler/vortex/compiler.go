package vortex

import (
	"fmt"
	"math"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

type roundStatus int

// Declare enum values using iota
const (
	// Denotes a round with no polynomials to commit to
	IsEmpty roundStatus = iota
	// Denotes a round when we apply only MiMC hashing
	// on the columns of the round matrix
	IsOnlyMiMCApplied
	// Denotes a round when we apply SIS+MiMC hashing
	// on the columns of the round matrix
	IsSISApplied
)

/*
Applies the Vortex compiler over the current polynomial-IOP
  - blowUpFactor : inverse rate of the reed-solomon code to use
  - SISTreshold : minimal number of polynomial in rounds to consider
    applying the SIS hashing on the columns of the round matrix. Implicitly, we
    consider that applying SIS hash over too few vectors is not worth it.
    For these rounds we will use the MiMC hash function directly to compute
    the leaves of the Merkle tree.

There are the following requirements:
  - FOR ALL ROUNDS, all the polynomials must have the same size
  - The inbound wizard-IOP must be a single-point polynomial-IOP
*/
func Compile(blowUpFactor int, options ...VortexOp) func(*wizard.CompiledIOP) {

	logrus.Trace("started vortex compiler")
	defer logrus.Trace("finished vortex compiler")

	if !utils.IsPowerOfTwo(blowUpFactor) {
		utils.Panic("expected a power of two but rho was %v", blowUpFactor)
	}

	return func(comp *wizard.CompiledIOP) {

		univQ, foundAny := extractTargetQuery(comp)
		if !foundAny {
			logrus.Infof("no query found in the vortex compilation context ...compilation step skipped")
			return
		}

		if len(comp.Columns.AllKeysCommitted()) == 0 {
			logrus.Infof("no committed polynomial in the compilation context... compilation step skipped")
			return
		}

		// create the compilation context
		ctx := newCtx(comp, univQ, blowUpFactor, options...)
		// if there is only a single-round, then this should be 1
		lastRound := comp.NumRounds() - 1

		// Stores a pointer to the cryptographic compiler of Vortex
		comp.PcsCtxs = ctx

		// Process the precomputed columns
		ctx.processStatusPrecomputed()

		// registers all the commitments
		for round := 0; round <= lastRound; round++ {
			ctx.compileRound(round)
			comp.RegisterProverAction(round, &ColumnAssignmentProverAction{
				Ctx:   ctx,
				Round: round,
			})
		}

		ctx.generateVortexParams()
		// Commit to precomputed columnsù
		if ctx.IsNonEmptyPrecomputed() {
			ctx.commitPrecomputeds()
		}
		ctx.registerOpeningProof(lastRound)

		// Registers the prover and verifier steps
		comp.RegisterProverAction(lastRound+1, &LinearCombinationComputationProverAction{
			Ctx: ctx,
		})
		comp.RegisterProverAction(lastRound+2, &OpenSelectedColumnsProverAction{
			Ctx: ctx,
		})
		// This is separated from GnarkVerify because, when doing full-recursion
		// , we want to recurse this verifier step but not [ctx.Verify] which is
		// already handled by the self-recursion mechanism.
		comp.RegisterVerifierAction(lastRound, &ExplicitPolynomialEval{
			Ctx: ctx,
		})
		comp.RegisterVerifierAction(lastRound+2, &VortexVerifierAction{
			Ctx: ctx,
		})

		if ctx.AddMerkleRootToPublicInputsOpt.Enabled {

			for _, round := range ctx.AddMerkleRootToPublicInputsOpt.Round {
				var (
					name = fmt.Sprintf("%v_%v", ctx.AddMerkleRootToPublicInputsOpt.Name, round)
					mr   = ctx.Items.MerkleRoots[round]
				)
				if mr == nil {
					utils.Panic("merkle root not found for round %v", round)
				}
				comp.InsertPublicInput(name, accessors.NewFromPublicColumn(ctx.Items.MerkleRoots[round], 0))
			}
		}

		if ctx.AddPrecomputedMerkleRootToPublicInputsOpt.Enabled {

			var (
				merkleRootColumn = ctx.Items.Precomputeds.MerkleRoot
				merkleRootValue  = ctx.Comp.Precomputed.MustGet(merkleRootColumn.GetColID())
			)

			ctx.AddPrecomputedMerkleRootToPublicInputsOpt.PrecomputedValue = merkleRootValue.Get(0)
			ctx.Comp.Columns.SetStatus(merkleRootColumn.GetColID(), column.Proof)
			ctx.Comp.Precomputed.Del(merkleRootColumn.GetColID())
			ctx.Comp.ExtraData[ctx.AddPrecomputedMerkleRootToPublicInputsOpt.Name] = merkleRootValue.Get(0)

			comp.RegisterProverAction(0, &ReassignPrecomputedRootAction{
				Ctx: ctx,
			})

			comp.InsertPublicInput(
				ctx.AddPrecomputedMerkleRootToPublicInputsOpt.Name,
				accessors.NewFromPublicColumn(
					ctx.Items.Precomputeds.MerkleRoot,
					0,
				),
			)
		}
	}
}

// Placeholder for variable commonly used within the vortex compilation
type Ctx struct {
	// The underlying compiled IOP protocol
	Comp *wizard.CompiledIOP
	// snapshot the self-recursion count immediately
	// when the context is created
	SelfRecursionCount int

	// The (verifiedly) unique polynomial query
	Query                        query.UnivariateEval
	PolynomialsTouchedByTheQuery map[ifaces.ColID]struct{}
	ShadowCols                   map[ifaces.ColID]struct{}

	// Public parameters of the commitment scheme
	BlowUpFactor int
	// Parameters for the optional SIS hashing feature
	// If the number of commitments for a given round
	// is more than this threshold, we apply SIS hashing
	//  and then MiMC hashing for computing the leaves of the Merkle tree.
	// Otherwise, we replace SIS and directly apply MiMC hashing
	// for computing the leaves of the Merkle tree.
	ApplySISHashThreshold int
	RoundStatus           []roundStatus
	// Committed rows count for both SIS and non-SIS rounds
	CommittedRowsCount int
	// Committed rows count for the SIS rounds only
	CommittedRowsCountSIS int
	// Number of columns in the Vortex matrix, i.e., the
	// length of each column to be committed to. Recall,
	// the rows of the vortex matrix are the
	// zkEVM columns.
	NumCols int
	// Maximum round number (including both SIS and non-SIS rounds)
	MaxCommittedRound int
	// Maximum round number for SIS rounds
	MaxCommittedRoundSIS int
	// Maximum round number for non-SIS rounds
	MaxCommittedRoundNonSIS int
	// The vortex parameters
	VortexParams *vortex.Params
	// The SIS hashing parameters
	SisParams *ringsis.Params
	// Optional parameter
	NumOpenedCol int

	// By rounds commitments
	CommitmentsByRounds collection.VecVec[ifaces.ColID]
	// SIS round commitments
	CommitmentsByRoundsSIS collection.VecVec[ifaces.ColID]
	// Non SIS round commitments
	CommitmentsByRoundsNonSIS collection.VecVec[ifaces.ColID]

	// RunStateNamePrefix is used to prefix some of the names of components of the
	// compilation context. Mainly state objects.
	RunStateNamePrefix string

	// Items created by Vortex, includes the proof message and the coins
	Items struct {
		// List of items used only if the CommitPrecomputed flag is set
		Precomputeds struct {
			// List of the precomputeds columns that we are compiling if the
			// the precomputed flag is set.
			PrecomputedColums []ifaces.Column
			// Merkle Root of the precomputeds columns
			MerkleRoot ifaces.Column
			// Committed matrix (rs encoded) of the precomputed columns
			CommittedMatrix vortex.EncodedMatrix
			// Tree in case of Merkle mode
			Tree *smt.Tree
			// colHashes used in self recursion
			DhWithMerkle []field.Element
		}
		// Alpha is a random combination linear coin
		Alpha coin.Info
		// Linear combination of the row-encoded matrix
		Ualpha ifaces.Column
		// Random column selection
		Q coin.Info
		// Opened columns, to be used in the Vortex compilation
		OpenedColumns []ifaces.Column
		// Opened SIS columns, to be used in the Self recursion compilation
		OpenedSISColumns []ifaces.Column
		// Opened non-SIS columns, to be used in the Self recursion compilation
		OpenedNonSISColumns []ifaces.Column
		// MerkleProofs
		// We represents all the Merkle proof as specfied here:
		MerkleProofs ifaces.Column
		// The Merkle roots are represented by a size 1 column
		// in the wizard.
		MerkleRoots []ifaces.Column
	}

	// IsSelfrecursed is a flag that tells the verifier Vortex to perform a
	// NO-OP. This flags can be activated by the self-recursion compiler (whose
	// goal is already to ensure that the verification was passing). It can also
	// be activated by the PreMarkAsSelfRecursed option which is used by the
	// full-recursion compiler.
	IsSelfrecursed bool

	// Additional options that tells the compiler to add a merkle root to the
	// public inputs of the comp. This is useful for the distributed prover.
	AddMerkleRootToPublicInputsOpt struct {
		Enabled bool
		Name    string
		Round   []int
	}

	// This option tells the compiler to add the merkle root of the precomputeds
	// columns to the public inputs. When the option is activated, the merkle root
	// is converted from "precomputed" to "proof" column and its value is set as
	// a public input of the compilation context. Since the column is no longer
	// considered a precomputed column in the wizard, its value is also removed
	// from the precomputed table and is moved to the compilation context. This
	// value will then be assigned to the column at round zero.
	AddPrecomputedMerkleRootToPublicInputsOpt struct {
		Enabled          bool
		Name             string
		PrecomputedValue field.Element
	}
}

// Construct a new compilation context
func newCtx(comp *wizard.CompiledIOP, univQ query.UnivariateEval, blowUpFactor int, options ...VortexOp) *Ctx {
	ctx := &Ctx{
		Comp:                         comp,
		SelfRecursionCount:           comp.SelfRecursionCount,
		Query:                        univQ,
		PolynomialsTouchedByTheQuery: map[ifaces.ColID]struct{}{},
		ShadowCols:                   map[ifaces.ColID]struct{}{},
		BlowUpFactor:                 blowUpFactor,
		// TODO : allows tuning for multiple instances at once
		SisParams: &ringsis.StdParams,
		Items: struct {
			Precomputeds struct {
				PrecomputedColums []ifaces.Column
				MerkleRoot        ifaces.Column
				CommittedMatrix   vortex.EncodedMatrix
				Tree              *smt.Tree
				DhWithMerkle      []field.Element
			}
			Alpha               coin.Info
			Ualpha              ifaces.Column
			Q                   coin.Info
			OpenedColumns       []ifaces.Column
			OpenedSISColumns    []ifaces.Column
			OpenedNonSISColumns []ifaces.Column
			MerkleProofs        ifaces.Column
			MerkleRoots         []ifaces.Column
		}{},
		// Declare the by rounds/sis rounds/non-sis rounds commitments
		CommitmentsByRounds:       collection.NewVecVec[ifaces.ColID](),
		CommitmentsByRoundsSIS:    collection.NewVecVec[ifaces.ColID](),
		CommitmentsByRoundsNonSIS: collection.NewVecVec[ifaces.ColID](),
	}

	for _, pol := range ctx.Query.Pols {
		ctx.PolynomialsTouchedByTheQuery[pol.GetColID()] = struct{}{}
	}

	for _, op := range options {
		op(ctx)
	}

	// Preallocate all the merkle roots for all rounds
	ctx.Items.MerkleRoots = make([]ifaces.Column, comp.NumRounds())

	// Declare the RoundStatus slice
	ctx.RoundStatus = make([]roundStatus, 0, comp.NumRounds())

	return ctx
}

// Compile a round of the wizard protocol
func (ctx *Ctx) compileRound(round int) {

	// List all of the commitments
	allComs := ctx.Comp.Columns.AllKeysCommittedAt(round)

	// edge-case : no commitment for the round = nothing to do
	if len(allComs) == 0 {
		// We add the default value as 0 in the empty round
		ctx.RoundStatus = append(ctx.RoundStatus, IsEmpty)
		return
	}

	// Else, we compile it as a normal round
	ctx.compileRoundWithVortex(round, allComs)
}

// Compile the round as a Vortex round : ensure the number of limbs divides
// the degree by introducing zero valued shadow columns if necessary, increase
// the number of rows count, insert Dh columns in the proof in case of non
// Merkle mode, or the Merkle roots for the Merkle mode.
//
// fillUpTo indicates how many filling "shadow" rows to add to the compilation.
// The function assumes that fillUpTo is "correctly", i.e. the function will
// brainlessly add the asked number of shadow rows.
func (ctx *Ctx) compileRoundWithVortex(round int, coms_ []ifaces.ColID) {

	// Sanity-check for double insertions
	if ctx.CommitmentsByRounds.LenOf(round) > 0 {
		panic("inserted twice")
	}

	// The startingRound is the first round of definition for the Vortex
	// compilation. It corresponds to the smallest round at which one
	// of the evaluated poly is defined. This is used to determine if
	// a Vortex compilation is needed or not.
	startingRound := ctx.startingRound()

	if round < startingRound {
		return
	}

	var (
		// Filters out the coms that are not touched by the query and mark them
		// directly as ignored. (We do not care about them because they are un-
		// constrained). But we still log these to alert during runtime.
		//
		// Also, the order in which the precomputed columns are taken must be the one
		// matching the query. Otherwise, we would not be able to obtain standard
		// proofs for the limitless prover.
		_, comUnconstrained = utils.FilterInSliceWithMap(coms_, ctx.PolynomialsTouchedByTheQuery)
		coms                = ctx.commitmentsAtRoundFromQuery(round)
		numComsActual       = len(coms) // actual == not shadow and not unconstrained
		fillUpTo            = len(coms)
		onlyMiMCApplied     = len(coms) < ctx.ApplySISHashThreshold
	)

	// This part corresponds to an edge-case that is not supposed to happen
	// in practice. Still, sometime, when debugging with upstream compilation
	// steps it can happen that all of the columns of a round end up
	// unconstrained. Importantly, the clause has to be put before attempting
	// to resolving a targetsize.
	if len(coms) == 0 {
		// We add the default value as 0 in the empty round
		ctx.RoundStatus = append(ctx.RoundStatus, IsEmpty)
		return
	}

	targetSize := ctx.Comp.Columns.GetSize(coms[0])

	for i := range coms {
		ctx.Comp.Columns.MarkAsIgnored(coms[i])
	}

	for i := range comUnconstrained {
		ctx.Comp.Columns.MarkAsIgnored(comUnconstrained[i])
	}

	// Note: the above "if-clause" ensures that the fillUpTo >= len(coms), so
	// it fillUpTo is equal to zero then coms is the empty slice. Otherwise, it
	// would have panicked at this point.
	if fillUpTo == 0 {
		return
	}

	// To ensure the number limbs in each subcol divides the degree, we pad the
	// list with shadow columns. This is required for self-recursion to work
	// correctly. In practice they do not cost anything to the prover. When
	// using MiMC, the number of limbs is equal to 1. This skips the
	// aforementioned behaviour.
	if !onlyMiMCApplied && ctx.SisParams.NumFieldPerPoly() > 1 {
		fillUpTo = utils.NextMultipleOf(fillUpTo, ctx.SisParams.NumFieldPerPoly())
	}

	numShadowRows := fillUpTo - len(coms)

	for shadowID := 0; shadowID < numShadowRows; shadowID++ {
		shadowCol := autoAssignedShadowRow(ctx.Comp, targetSize, round, shadowID)
		ctx.ShadowCols[shadowCol.GetColID()] = struct{}{}
		ctx.Comp.Columns.MarkAsIgnored(shadowCol.GetColID())
		coms = append(coms, shadowCol.GetColID())
	}

	logrus.
		WithField("where", "compileRoundWithVortex").
		WithField("using-only-MiMC", onlyMiMCApplied).
		WithField("numComs", numComsActual).
		WithField("numShadowRows", numShadowRows).
		WithField("numUnconstrained", len(comUnconstrained)).
		WithField("round", round).
		Info("Compiled Vortex round")

	if onlyMiMCApplied {
		ctx.RoundStatus = append(ctx.RoundStatus, IsOnlyMiMCApplied)
		ctx.CommitmentsByRoundsNonSIS.AppendToInner(round, coms...)
		ctx.MaxCommittedRoundNonSIS = utils.Max(ctx.MaxCommittedRoundNonSIS, round)
	} else {
		ctx.RoundStatus = append(ctx.RoundStatus, IsSISApplied)
		ctx.CommitmentsByRoundsSIS.AppendToInner(round, coms...)
		// Increase the number of SIS round rows
		ctx.CommittedRowsCountSIS += len(coms)
		ctx.MaxCommittedRoundSIS = utils.Max(ctx.MaxCommittedRoundSIS, round)
	}

	ctx.CommitmentsByRounds.AppendToInner(round, coms...)

	// Ensures all commitments have the same length
	ctx.assertPolynomialHaveSameLength(coms)

	// Increase the number of rows
	ctx.CommittedRowsCount += len(coms)
	ctx.MaxCommittedRound = utils.Max(ctx.MaxCommittedRound, round)

	// Instead, we send Merkle roots that are symbolized with 1-sized
	// columns.
	ctx.Items.MerkleRoots[round] = ctx.Comp.InsertProof(
		round,
		ifaces.ColID(ctx.MerkleRootName(round)),
		1,
	)
}

// asserts that the compiled IOP has only a single query and that this query
// is a univariate evaluation. Also, mark the query as ignored when found.
func extractTargetQuery(comp *wizard.CompiledIOP) (res query.UnivariateEval, foundAny bool) {

	// Tracks the uncompiled queries. There should be only one at the end.
	// Scans the other no params (we expect none)
	uncompiledQueries := append([]ifaces.QueryID{}, comp.QueriesNoParams.AllUnignoredKeys()...)
	if len(uncompiledQueries) > 0 {
		utils.Panic("Expected no unparametrized queries, found %v\n", uncompiledQueries)
	}

	// Scans the univariate evaluatations : there should be only a single one
	uncompiledQueries = append([]ifaces.QueryID{}, comp.QueriesParams.AllUnignoredKeys()...)

	// If no queries are found, then there is nothing to compile. Return a
	// a negative found any to notify the caller context.
	if len(uncompiledQueries) == 0 {
		return query.UnivariateEval{}, foundAny
	}

	// Else, this is suspicious. There are no known use-cases where this would
	// be legitimate.
	if len(uncompiledQueries) != 1 {
		utils.Panic("Expected (exactly) one query, found %v (%v)\n", len(uncompiledQueries), uncompiledQueries)
	}

	res = comp.QueriesParams.Data(uncompiledQueries[0]).(query.UnivariateEval)

	// And mark it the query as compiled
	comp.QueriesParams.MarkAsIgnored(res.QueryID)

	return res, true
}

// asserts that all polynomials have the same length and set the field
// numcols. Also implictly set the value of ctx.numCols.
func (ctx *Ctx) assertPolynomialHaveSameLength(coms []ifaces.ColID) {
	for _, com := range coms {
		handle := ctx.Comp.Columns.GetHandle(com)
		length := handle.Size()

		// if the numCols has not been set (e.g. it is zero), set it
		if ctx.NumCols == 0 {
			ctx.NumCols = length
		}

		if length != ctx.NumCols {
			utils.Panic("commitments %v (size %v) does not have the target size %v", com, length, ctx.NumCols)
		}
	}
}

// generates the sis params. first check if we have any value to commit to
func (ctx *Ctx) generateVortexParams() {

	totalCommitted := ctx.CommittedRowsCount + len(ctx.Items.Precomputeds.PrecomputedColums)

	// edge-case : we in fact have nothing to commit to. So no need to gene-
	// rate the vortex params.
	if ctx.NumCols == 0 || totalCommitted == 0 {
		logrus.Infof("nothing to commit to")
		return
	}

	// Initialize the Params in the vanilla mode by default
	sisParams := ctx.SisParams
	if sisParams == nil {
		// In this case we pass the default SIS instance to vortex.
		sisParams = &ringsis.StdParams
	}
	ctx.VortexParams = vortex.NewParams(ctx.BlowUpFactor, ctx.NumCols, totalCommitted, *sisParams)
}

// return the number of columns to open
func (ctx *Ctx) NbColsToOpen() int {

	// opportunistic sanity-check : params should be set by now
	if ctx.VortexParams == nil {
		utils.Panic("VortexParams was not set")
	}

	// If the context was created with the relevant option,
	// we return the instructed value
	if ctx.NumOpenedCol > 0 {
		return ctx.NumOpenedCol
	}

	if !utils.IsPowerOfTwo(ctx.BlowUpFactor) {
		utils.Panic("expected a power of two but rho was %v", ctx.BlowUpFactor)
	}

	logBlowUpFactor := int(math.Log2(float64(ctx.BlowUpFactor)))

	if 1<<logBlowUpFactor != ctx.BlowUpFactor {
		utils.Panic("rho %v, logRho %v", ctx.BlowUpFactor, logBlowUpFactor)
	}

	// the 2 factor comes from the factor that we rely on the Guruswami-Sudan
	// list decoding regime and not the (alleged)-capacity decoding level.
	nb := 2 * crypto.TargetSecurityLevel / logBlowUpFactor
	if nb*logBlowUpFactor < crypto.TargetSecurityLevel {
		nb++
	}

	return nb
}

// registers the vortex opening proof. As an input, we pass the last round
// of the protocol. Also register them in the item set of the context.
func (ctx *Ctx) registerOpeningProof(lastRound int) {

	// register the linear combination randomness
	ctx.Items.Alpha = ctx.Comp.InsertCoin(
		lastRound+1,
		ctx.LinCombRandCoinName(),
		coin.Field,
	)

	// registers the linear combination claimed by the prover
	ctx.Items.Ualpha = ctx.Comp.InsertProof(
		lastRound+1,
		ctx.LinCombName(),
		ctx.NumEncodedCols(),
	)

	// registers the random's verifier column selection
	ctx.Items.Q = ctx.Comp.InsertCoin(
		lastRound+2,
		ctx.RandColSelectionName(),
		coin.IntegerVec,
		ctx.NbColsToOpen(),
		ctx.NumEncodedCols(),
	)

	// and registers the opened columns
	numRows := utils.NextPowerOfTwo(ctx.CommittedRowsCount)
	numRowsSIS := utils.NextPowerOfTwo(ctx.CommittedRowsCountSIS)
	numRowsNonSIS := utils.NextPowerOfTwo(ctx.CommittedRowsCount - ctx.CommittedRowsCountSIS)
	for col := 0; col < ctx.NbColsToOpen(); col++ {
		openedCol := ctx.Comp.InsertProof(
			lastRound+2,
			ctx.SelectedColName(col),
			numRows,
		)
		ctx.Items.OpenedColumns = append(ctx.Items.OpenedColumns, openedCol)
		if numRowsSIS != 0 {
			openedColSIS := ctx.Comp.InsertProof(
				lastRound+2,
				ctx.SelectedColSISName(col),
				numRowsSIS,
			)
			ctx.Items.OpenedSISColumns = append(ctx.Items.OpenedSISColumns, openedColSIS)
		}
		if numRowsNonSIS != 0 {
			openedColNonSIS := ctx.Comp.InsertProof(
				lastRound+2,
				ctx.SelectedColNonSISName(col),
				numRowsNonSIS,
			)
			ctx.Items.OpenedNonSISColumns = append(ctx.Items.OpenedNonSISColumns, openedColNonSIS)
		}
	}

	// In case of the Merkle-proof mode, we also registers the
	// column that will contain the Merkle proofs altogether. But
	// first, we need to evaluate its size. The proof size needs to
	// be padded up to a power of two. Otherwise, we can't use PeriodicSampling.
	ctx.Items.MerkleProofs = ctx.Comp.InsertProof(
		lastRound+2,
		ifaces.ColID(ctx.MerkleProofName()),
		ctx.MerkleProofSize(),
	)

}

// Returns the number of encoded columns in the vortex commitment. NB: it overlaps with
// `vortex.Params.NumEncodedCols` but we need a separate function because we need to
// call it at a moment where the vortex.Params are not all available.
func (ctx *Ctx) NumEncodedCols() int {
	res := utils.NextPowerOfTwo(ctx.NumCols) * ctx.BlowUpFactor

	// sanity-check : it is never supposed to return 0
	if ctx.NumCols == 0 {
		utils.Panic("ctx.numCols is zero")
	}

	// sanity-check : the blow up factor is zero
	if ctx.BlowUpFactor == 0 {
		utils.Panic("ctx.blowUpFactor is zero")
	}

	return res
}

// We check if there are non zero numbers of precomputed columns to commit to.
func (ctx *Ctx) IsNonEmptyPrecomputed() bool {
	if len(ctx.Items.Precomputeds.PrecomputedColums) > 0 {
		return true
	} else {
		return false
	}
}

// IsSISAppliedToPrecomputed returns true if SIS is applied to the precomputed
// columns. This happens when the number of precomputed columns is greater than
// the ApplySISHashThreshold.
func (ctx *Ctx) IsSISAppliedToPrecomputed() bool {
	if ctx.Items.Precomputeds.PrecomputedColums == nil {
		return false
	}
	return len(ctx.Items.Precomputeds.PrecomputedColums) > ctx.ApplySISHashThreshold
}

// Turns the precomputed into verifying key messages. A possible improvement
// would be to make them an entire commitment but we estimate that it will not
// be worth it. If the flag `CommitPrecomputed` is set to `true`, this will
// instead register the precomputed columns.
func (ctx *Ctx) processStatusPrecomputed() {

	var (
		comp = ctx.Comp
	)

	// This captures the precomputed column.  It is essential to do it this
	// particular moment, because the number of precomputed columns is going to
	// change during the compilation time (in later compilation stage). And
	// we want the precomputed columns defined at the beginning of the current
	// vortex compilation step to be captured only.
	//
	// Also, the order in which the precomputed columns are taken must be the one
	// matching the query. Otherwise, we would not be able to obtain standard
	// proofs for the limitless prover.
	precomputedColNames := ctx.commitmentsAtRoundFromQueryPrecomputed()
	if len(precomputedColNames) == 0 {
		ctx.Items.Precomputeds.PrecomputedColums = []ifaces.Column{}
		return
	}

	// This captures the name of the precomputed columns that we ignore
	precomputedColSkipped := []ifaces.ColID{}

	// Sanity-check. This should be enforced by the splitter compiler already.
	ctx.assertPolynomialHaveSameLength(precomputedColNames)
	precomputedCols := []ifaces.Column{}

	// If there are not enough columns, for a commitment to be meaningful,
	// explicitly sends the column to the verifier so that he can check the
	// evaluation by itself.

	for _, name := range precomputedColNames {

		_, ok := ctx.PolynomialsTouchedByTheQuery[name]
		if !ok {
			precomputedColSkipped = append(precomputedColSkipped, name)
			comp.Columns.MarkAsIgnored(name)
			continue
		}

		pCol := comp.Columns.GetHandle(name)
		// Marking all these columns as "Ignored" to mean that the compiler
		// should ignore theses columns.
		comp.Columns.MarkAsIgnored(name)
		ctx.NumCols = pCol.Size()
		precomputedCols = append(precomputedCols, pCol)
	}

	var (
		nbUnskippedPrecomputedCols = len(precomputedCols)
		fillUpTo                   = nbUnskippedPrecomputedCols
		onlyMiMCApplied            = nbUnskippedPrecomputedCols < ctx.ApplySISHashThreshold
	)

	// Note: the above "if-clause" ensures that the fillUpTo >= len(coms), so
	// it fillUpTo is equal to zero then coms is the empty slice. Otherwise, it
	// would have panicked at this point.
	if fillUpTo == 0 {
		return
	}

	// To ensure the number limbs in each subcol divides the degree, we pad the
	// list with shadow columns. This is required for self-recursion to work
	// correctly. In practice they do not cost anything to the prover. When
	// using MiMC, the number of limbs is equal to 1. This skips the
	// aforementioned behaviour.
	if !onlyMiMCApplied && ctx.SisParams.NumFieldPerPoly() > 1 {
		fillUpTo = utils.NextMultipleOf(fillUpTo, ctx.SisParams.NumFieldPerPoly())
	}

	numShadowRows := fillUpTo - nbUnskippedPrecomputedCols

	// This corresponds to a technical edge-case. The SIS hash function works
	// by mapping fields elements to a list of limbs. Limbs are in turn mapped
	// to coefficients of a polynomial in some polynomial ring. In some cases,
	// we may have more than one field element that is mapped to the same
	// polynomial. When that happens, we want to ensure that "rows" belonging
	// to different rounds to do not end up mapped to the same SIS polynomials
	// (recall that we SIS-hash the column of the committed matrix). Otherwise,
	// it becomes a problem later on when dealing with self-recursion. To
	// prevent that from happening, we insert "shadow" rows, which are rows that
	// may only contain zero and may only evaluate to zero whichever is the
	// queried evaluation point.
	for i := 0; i < numShadowRows; i++ {
		// The shift by 20 is to avoid collision with the committed cols
		// at round zero.
		shadowCol := autoAssignedShadowRow(comp, ctx.NumCols, 0, 1<<20+i)
		ctx.ShadowCols[shadowCol.GetColID()] = struct{}{}
		precomputedColNames = append(precomputedColNames, shadowCol.GetColID())
		precomputedCols = append(precomputedCols, shadowCol)
		ctx.Comp.Columns.MarkAsIgnored(shadowCol.GetColID())
	}

	ctx.Items.Precomputeds.PrecomputedColums = precomputedCols

	log := logrus.
		WithField("where isSISAppliedForCommitment", !onlyMiMCApplied).
		WithField("nbPrecomputedRows", nbUnskippedPrecomputedCols).
		WithField("nbShadowRows", numShadowRows)

	if len(precomputedColSkipped) > 0 {
		log.
			WithField("nbSkippedRows", len(precomputedColSkipped)).
			WithField("skipped-columns", precomputedColSkipped).
			Warnf("Found unconstrained columns. Skipping them and mark them as ignored")
		return
	}
	log.Info("processed the precomputed columns")
}

// Returns the number of committed rounds. Must be called after the
// method compileRound has been executed. Otherwise, it will output zero.
func (ctx *Ctx) NumCommittedRounds() int {
	res := 0

	// MaxCommittedRounds is unset if the function is called before
	// the compileRound method. Careful, the stopping condition is
	// an LE and not a strict LT condition.
	for i := 0; i <= ctx.MaxCommittedRound; i++ {
		if ctx.RoundStatus[i] == IsEmpty {
			// We skip the empty rounds
			continue
		}
		res++
	}

	return res
}

// Returns the number of rounds committed with SIS hashing. Must be called after the
// method compileRound has been executed. Otherwise, it will output zero.
func (ctx *Ctx) NumCommittedRoundsSis() int {
	res := 0

	// MaxCommittedRounds is unset if the function is called before
	// the compileRound method. Careful, the stopping condition is
	// an LE and not a strict LT condition.
	for i := 0; i <= ctx.MaxCommittedRound; i++ {
		if ctx.RoundStatus[i] != IsSISApplied {
			// We skip the no SIS and the empty rounds
			continue
		}
		res++
	}

	return res
}

// Returns the number of rounds committed without SIS hashing. Must be called after the
// method compileRound has been executed. Otherwise, it will output zero.
func (ctx *Ctx) NumCommittedRoundsNoSis() int {
	res := 0

	// MaxCommittedRounds is unset if the function is called before
	// the compileRound method. Careful, the stopping condition is
	// an LE and not a strict LT condition.
	for i := 0; i <= ctx.MaxCommittedRound; i++ {
		if ctx.RoundStatus[i] != IsOnlyMiMCApplied {
			// We skip the SIS and the empty rounds
			continue
		}
		res++
	}

	return res
}

// MerkleProofSize Returns the size of the allocated Merkle proof vector
func (ctx *Ctx) MerkleProofSize() int {
	// We registers the column that will contain the Merkle proofs altogether. But
	// first, we need to evaluate its size. The proof size needs to
	// be padded up to a power of two. Otherwise, we can't use PeriodicSampling.
	var (
		depth      = utils.Log2Ceil(ctx.NumEncodedCols())
		numComs    = ctx.NumCommittedRounds()
		numOpening = ctx.NbColsToOpen()
	)
	// The number of rounds increases by 1 for committing to the precomputed
	if ctx.IsNonEmptyPrecomputed() {
		numComs += 1
	}

	if depth*numComs*numOpening == 0 {
		utils.Panic("something was zero : %v, %v, %v", depth, numComs, numOpening)
	}

	return utils.NextPowerOfTwo(depth * numComs * numOpening)
}

// Commit to the precomputed columns
func (ctx *Ctx) commitPrecomputeds() {
	var (
		committedMatrix vortex.EncodedMatrix
		tree            *smt.Tree
		colHashes       []field.Element
	)
	precomputeds := ctx.Items.Precomputeds.PrecomputedColums
	numPrecomputeds := len(precomputeds)

	// This can happens if either there are no precomputed columns or
	// a bug in the code.
	if numPrecomputeds == 0 {
		logrus.Tracef("skip commit precomputeds, as there are no precomputed columns!")
		return
	}

	// Fetch the assignments (known at compile time since they are precomputeds)
	pols := make([]smartvectors.SmartVector, numPrecomputeds)
	for i, precomputed := range precomputeds {
		if _, ok := ctx.ShadowCols[precomputed.GetColID()]; ok {
			pols[i] = smartvectors.NewConstant(field.Zero(), ctx.NumCols)
			continue
		}
		pols[i] = ctx.Comp.Precomputed.MustGet(precomputed.GetColID())
	}

	// Increase the number of committed rows
	ctx.CommittedRowsCount += numPrecomputeds

	// Committing to the precomputed columns with SIS or without SIS.
	if ctx.IsSISAppliedToPrecomputed() {
		// We increase the number of committed rows for SIS rounds
		// in this case
		ctx.CommittedRowsCountSIS += numPrecomputeds
		logrus.Infof("committing to precomputed columns with SIS")
		committedMatrix, tree, colHashes = ctx.VortexParams.CommitMerkleWithSIS(pols)
	} else {
		logrus.Infof("committing to precomputed columns without SIS")
		committedMatrix, tree, colHashes = ctx.VortexParams.CommitMerkleWithoutSIS(pols)
	}
	ctx.Items.Precomputeds.DhWithMerkle = colHashes
	ctx.Items.Precomputeds.CommittedMatrix = committedMatrix
	ctx.Items.Precomputeds.Tree = tree

	// And assign the 1-sized column to contain the root
	var root field.Element
	root.SetBytes(tree.Root[:])
	ctx.Items.Precomputeds.MerkleRoot = ctx.Comp.RegisterVerifyingKey(ctx.PrecomputedMerkleRootName(), smartvectors.NewConstant(root, 1))

}

// GetPrecomputedSelectedCol returns the selected column
// of the precomputed columns stored as matrix
func (ctx *Ctx) GetPrecomputedSelectedCol(index int) []field.Element {
	if ctx.Items.Precomputeds.CommittedMatrix == nil {
		utils.Panic("precomputed matrix is nil")
	}

	col := make([]field.Element, len(ctx.Items.Precomputeds.PrecomputedColums))

	for i := 0; i < len(ctx.Items.Precomputeds.PrecomputedColums); i++ {
		col[i] = ctx.Items.Precomputeds.CommittedMatrix[i].Get(index)
	}
	return col
}

// GetNumPolsForNonSisRounds returns an integer
// giving the number of polynomials for the given
// non SIS round
func (ctx *Ctx) GetNumPolsForNonSisRounds(round int) int {
	// Sanity check
	if ctx.RoundStatus[round] != IsOnlyMiMCApplied {
		utils.Panic("Expected a non SIS round!")
	}
	return ctx.CommitmentsByRounds.LenOf(round)
}

// startingRound returns the first round of definition for the Vortex
// compilation. It corresponds to the smallest round at which one
// of the evaluated poly is defined. This is used to determine if
// a Vortex compilation is needed or not. The function ignores
// precomputed and verifier-defined columns.
func (ctx *Ctx) startingRound() int {

	startingRound := math.MaxInt
	for _, p := range ctx.Query.Pols {
		if ctx.Comp.Precomputed.Exists(p.GetColID()) {
			continue
		}

		if _, isV := p.(verifiercol.VerifierCol); isV {
			continue
		}

		startingRound = min(startingRound, p.Round())
	}

	return startingRound
}

// commitmentAtRoundFromQuery returns the commitment at the given round
// in the same order of appearance as in the query. The function ignores
// the precomputed columns.
func (ctx *Ctx) commitmentsAtRoundFromQuery(round int) []ifaces.ColID {

	res := make([]ifaces.ColID, 0, len(ctx.Query.Pols))

	for _, p := range ctx.Query.Pols {

		if p.Round() != round {
			continue
		}

		if ctx.Comp.Precomputed.Exists(p.GetColID()) {
			continue
		}

		if _, isV := p.(verifiercol.VerifierCol); isV {
			panic("verifiercol")
		}

		if nat, isNat := p.(column.Natural); !isNat || nat.Status() != column.Committed {
			panic("not committed")
		}

		res = append(res, p.GetColID())
	}
	return res
}

// commitmentAtRoundFromQueryPrecomputed returns the commitment at the given round
// in the same order of appearance as in the query. The function only considers
// the precomputed columns.
func (ctx *Ctx) commitmentsAtRoundFromQueryPrecomputed() []ifaces.ColID {

	res := make([]ifaces.ColID, 0, len(ctx.Query.Pols))

	for _, p := range ctx.Query.Pols {

		if !ctx.Comp.Precomputed.Exists(p.GetColID()) {
			continue
		}

		if _, isV := p.(verifiercol.VerifierCol); isV {
			panic("verifiercol")
		}

		nat, isNat := p.(column.Natural)
		if !isNat {
			utils.Panic("not a Natural column: %v, type=%T", p.GetColID(), p)
		}

		if nat.Status() != column.Precomputed {
			utils.Panic("not precomputed, nat.status=%v nat.id=%v", nat.Status().String(), nat.ID)
		}

		res = append(res, p.GetColID())
	}

	return res
}
