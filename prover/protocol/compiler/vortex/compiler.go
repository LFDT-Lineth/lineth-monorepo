package vortex

import (
	"math"

	"github.com/consensys/linea-monorepo/prover/crypto"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/crypto/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

/*
Applies the Vortex compiler over the current polynomial-IOP
  - blowUpFactor : inverse rate of the reed-solomon code to use
  - dryTreshold : minimal number of polynomial in rounds to consider
    applying the Vortex transform (i.e. using Vortex). Implicitly, we
    consider that applying Vortex over too few vectors is not worth it.
    For these rounds all the "Committed" columns are swithed to "prover"

There are the following requirements:
  - FOR NON-DRY ROUNDS, all the polynomials must have the same size
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

		// registers all the commitments
		if len(comp.Columns.AllKeysCommitted()) == 0 {
			logrus.Infof("no committed polynomial in the compilation context... compilation step skipped")
			return
		}

		// create the compilation context
		ctx := newCtx(comp, univQ, blowUpFactor, options...)
		// if there is only a single-round, then this should be 1
		lastRound := comp.NumRounds() - 1

		// Stores a pointer to the cryptographic compiler of Vortex
		comp.PcsCtxs = &ctx

		// Converts the precomputed as verifying key (e.g. send
		// them to the verifier) in the offline phase if the
		// CommitPrecomputes flag is false, otherwise set them as
		// commited. If the flag `CommitPrecomputed` is set to true,
		// then this will instead register the precomputed columns.
		ctx.processStatusPrecomputed()

		// registers all the commitments
		for round := 0; round <= lastRound; round++ {
			ctx.compileRound(round)
			comp.SubProvers.AppendToInner(round, ctx.AssignColumn(round))
		}

		ctx.generateVortexParams()
		// Commit to precomputed in Vortex if IsCommitToPrecomputed is true
		if ctx.IsCommitToPrecomputed() {
			ctx.commitPrecomputeds()
		}
		ctx.registerOpeningProof(lastRound)

		// Registers the prover and verifier steps
		comp.SubProvers.AppendToInner(lastRound+1, ctx.ComputeLinearComb)
		comp.SubProvers.AppendToInner(lastRound+2, ctx.OpenSelectedColumns)
		// This is separated from GnarkVerify because, when doing full-recursion
		// , we want to recurse this verifier step but not [ctx.Verify] which is
		// already handled by the self-recursion mechanism.
		comp.InsertVerifier(lastRound, ctx.explicitPublicEvaluation, ctx.gnarkExplicitPublicEvaluation)
		comp.InsertVerifier(lastRound+2, ctx.Verify, ctx.GnarkVerify)
	}
}

// Placeholder for variable commonly used within the vortex compilation
type Ctx struct {
	// The underlying compiled IOP protocol
	comp *wizard.CompiledIOP
	// snapshot the self-recursion count immediately
	// when the context is created
	SelfRecursionCount int

	// Flag indicating that we want to replace SIS by MiMC
	ReplaceSisByMimc bool

	// The (verifiedly) unique polynomial query
	Query                        query.UnivariateEval
	PolynomialsTouchedByTheQuery map[ifaces.ColID]struct{}
	ShadowCols                   map[ifaces.ColID]struct{}

	// Public parameters of the commitment scheme
	BlowUpFactor       int
	DryTreshold        int
	CommittedRowsCount int
	NumCols            int
	MaxCommittedRound  int
	VortexParams       *vortex.Params
	SisParams          *ringsis.Params
	// Optional parameter
	numOpenedCol int

	// By rounds commitments : if a round is dried we make an empty sublist.
	// Inversely, for the `driedByRounds` which track the dried commitments.
	CommitmentsByRounds collection.VecVec[ifaces.ColID]
	DriedByRounds       collection.VecVec[ifaces.ColID]

	// Items created by Vortex, includes the proof message and the coins
	Items struct {
		// List of items used only if the CommitPrecomputed flag is set
		Precomputeds struct {
			// List of the precomputeds columns that we are compiling if the
			// the precomputed flag is set.
			PrecomputedColums []ifaces.Column
			// Merkle Root of the precomputeds columns
			MerkleRoot ifaces.Column
			// List of the column hashes for the precomputed columns
			Dh ifaces.Column
			// Committed matrix (rs encoded) of the precomputed columns
			CommittedMatrix vortex.EncodedMatrix
			// Tree in case of Merkle mode
			tree *smt.Tree
			// colHashes used in self recursion
			DhWithMerkle []field.Element
		}
		// (not used in the Merkle proof version)
		Dh []ifaces.Column
		// Alpha is a random combination linear coin
		Alpha coin.Info
		// Linear combination of the row-encoded matrix
		Ualpha ifaces.Column
		// Random column selection
		Q coin.Info
		// Opened columns
		OpenedColumns []ifaces.Column
		// MerkleProof (only used with the MerkleProof version)
		// We represents all the Merkle proof as specfied here:
		MerkleProofs ifaces.Column
		// The Merkle roots are represented by a size 1 column
		// in the wizard.
		MerkleRoots []ifaces.Column
	}

	// Skip verification is a flag that tells the verifier Vortex to perform a
	// NO-OP. This flags is can be activated by the self-recursion layer (whose
	// goal is already to ensure that the verification was passing). This only
	// concerns the "Vortex" part of the verification all the dried rounds are
	// still explicitly verified by the verifier.
	IsSelfrecursed bool
}

// Construct a new compilation context
func newCtx(comp *wizard.CompiledIOP, univQ query.UnivariateEval, blowUpFactor int, options ...VortexOp) Ctx {
	ctx := Ctx{
		comp:                         comp,
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
				Dh                ifaces.Column
				CommittedMatrix   vortex.EncodedMatrix
				tree              *smt.Tree
				DhWithMerkle      []field.Element
			}
			Dh            []ifaces.Column
			Alpha         coin.Info
			Ualpha        ifaces.Column
			Q             coin.Info
			OpenedColumns []ifaces.Column
			MerkleProofs  ifaces.Column
			MerkleRoots   []ifaces.Column
		}{},
		// rowsCount : initialized to zero, set a posteriori
		// when compiling the instance.
		// numcol : set a posteriori during the compilation
		// vortex params : the vortex params are set a posteriori
		// during the compilation
		CommitmentsByRounds: collection.NewVecVec[ifaces.ColID](),
		DriedByRounds:       collection.NewVecVec[ifaces.ColID](),
	}

	for _, pol := range ctx.Query.Pols {
		ctx.PolynomialsTouchedByTheQuery[pol.GetColID()] = struct{}{}
	}

	for _, op := range options {
		op(&ctx)
	}

	// Preallocate all the merkle roots for all rounds
	ctx.Items.MerkleRoots = make([]ifaces.Column, comp.NumRounds())

	return ctx
}

// Compile a round of the wizard protocol
func (ctx *Ctx) compileRound(round int) {

	// List all of the commitments
	allComs := ctx.comp.Columns.AllKeysCommittedAt(round)

	// edge-case : no commitment for the round = nothing to do
	if len(allComs) == 0 {
		return
	}

	if len(allComs) <= ctx.DryTreshold {
		// Compile the round as a dry round
		ctx.compileRoundAsDry(round, allComs)
		return
	}

	// Else, we compile it as a normal round
	ctx.compileRoundWithVortex(round, allComs)
}

// Compile the round as a dry round : pass all committed as prover
// messages directly instead of sending them to the oracle.
func (ctx *Ctx) compileRoundAsDry(round int, coms []ifaces.ColID) {

	// sanity-check for double insertions
	if ctx.DriedByRounds.LenOf(round) > 0 {
		utils.Panic("inserted twice in round %v : we had already %v\n", round, ctx.DriedByRounds.LenOf(round))
	}
	ctx.DriedByRounds.AppendToInner(round, coms...)

	// mark the commitments as messages
	for _, com := range coms {
		ctx.comp.Columns.SetStatus(com, column.Proof)
	}
}

// Compile the round as a Vortex round : ensure the number of limbs divides the degree by
// introducing zero valued shadow columns if necessary, increase the number of rows count,
// insert Dh columns in the proof in case of non Merkle mode, or the Merkle roots for the Merkle mode
func (ctx *Ctx) compileRoundWithVortex(round int, coms []ifaces.ColID) {

	// Sanity-check for double insertions
	if ctx.CommitmentsByRounds.LenOf(round) > 0 {
		panic("inserted twice")
	}

	// Filters out the coms that are not touched by the query and mark them
	// directly as ignored. (We do not care about them because they are un-
	// constrained). But we still log these to alert during runtime.
	{
		coms_ := coms
		coms = make([]ifaces.ColID, 0, len(coms_))

		for _, com := range coms_ {

			if _, ok := ctx.PolynomialsTouchedByTheQuery[com]; !ok {
				logrus.Warnf("found unconstrained column : %v", com)
				ctx.comp.Columns.MarkAsIgnored(com)
				continue
			}

			coms = append(coms, com)
		}
	}

	if len(coms) == 0 {
		return
	}

	// To ensure the number limbs in each subcol divides the degree, we pad the list
	// with shadow columns. This is required for self-recursion to work correctly. In
	// practice they do not cost anything to the prover. When using MiMC, the number of
	// limbs is equal to 1. This skips the aforementioned behaviour.
	numLimbs := 1
	deg := 1
	if !ctx.ReplaceSisByMimc {
		numLimbs = ctx.SisParams.NumLimbs()
		deg = ctx.SisParams.OutputSize()
	}

	if deg > numLimbs {
		// We still require that the output size should be divisible by the number of
		// limbs. @alex We could still support it if useful enough.
		if deg%numLimbs != 0 {
			utils.Panic("the number of limbs should at least divide the degree")
		}
		numFieldPerPoly := utils.Max(1, deg/numLimbs)
		numShadow := (numFieldPerPoly - (len(coms) % numFieldPerPoly)) % numFieldPerPoly
		targetSize := ctx.comp.Columns.GetSize(coms[0])

		logrus.Debugf("Vortex compiler, registering shadow columns : round=%v numShadow=%v numFieldPerPoly=%v", round, numShadow, numFieldPerPoly)

		for shadowID := 0; shadowID < numShadow; shadowID++ {
			// Generate the shadow columns
			shadowCol := autoAssignedShadowRow(ctx.comp, targetSize, round, shadowID)
			// Register the column as part of the shadow columns
			ctx.ShadowCols[shadowCol.GetColID()] = struct{}{}
			// And append it to the list of the commitments
			coms = append(coms, shadowCol.GetColID())
		}
	}

	ctx.CommitmentsByRounds.AppendToInner(round, coms...)

	// Ensures all commitments have the same length
	ctx.assertPolynomialHaveSameLength(coms)

	// Also, mark all of them as being ignored
	for _, com := range coms {
		ctx.comp.Columns.MarkAsIgnored(com)
	}

	// Increase the number of rows
	ctx.CommittedRowsCount += len(coms)
	ctx.MaxCommittedRound = utils.Max(ctx.MaxCommittedRound, round)

	// Instead, we send Merkle roots that are symbolized with 1-sized
	// columns.
	ctx.Items.MerkleRoots[round] = ctx.comp.InsertProof(
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
		handle := ctx.comp.Columns.GetHandle(com)
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
	// edge-case : we in fact have nothing to commit to. So no need to gene-
	// rate the vortex params.
	if ctx.NumCols == 0 || ctx.CommittedRowsCount == 0 {
		logrus.Infof("nothing to commit to")
		return
	}

	// Initialize the Params in the vanilla mode by default
	sisParams := ctx.SisParams
	if sisParams == nil {
		// happens when using the ReplaceByMiMC options. In that case
		// we pass the default SIS instance to vortex. They are then
		// erased by the `RemoveSIS` options.
		if !ctx.ReplaceSisByMimc {
			panic("unexpected, SisParams are nil but the ReplaceSisByMimc option is unset")
		}
		sisParams = &ringsis.StdParams
	}
	ctx.VortexParams = vortex.NewParams(ctx.BlowUpFactor, ctx.NumCols, ctx.CommittedRowsCount, *sisParams, mimc.NewMiMC)

	// And replace SIS by MiMC if this is deemed useful
	if ctx.ReplaceSisByMimc {
		ctx.VortexParams.RemoveSis(mimc.NewMiMC)
	}
}

// return the number of columns to open
func (ctx *Ctx) NbColsToOpen() int {

	// opportunistic sanity-check : params should be set by now
	if ctx.VortexParams == nil {
		utils.Panic("ilcParams was not set")
	}

	// If the context was created with the relevant option,
	// we return the instructed value
	if ctx.numOpenedCol > 0 {
		return ctx.numOpenedCol
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
	ctx.Items.Alpha = ctx.comp.InsertCoin(
		lastRound+1,
		ctx.LinCombRandCoinName(),
		coin.Field,
	)

	// registers the linear combination claimed by the prover
	ctx.Items.Ualpha = ctx.comp.InsertProof(
		lastRound+1,
		ctx.LinCombName(),
		ctx.NumEncodedCols(),
	)

	// registers the random's verifier column selection
	ctx.Items.Q = ctx.comp.InsertCoin(
		lastRound+2,
		ctx.RandColSelectionName(),
		coin.IntegerVec,
		ctx.NbColsToOpen(),
		ctx.NumEncodedCols(),
	)

	// and registers the opened columns
	numRows := utils.NextPowerOfTwo(ctx.CommittedRowsCount)
	for col := 0; col < ctx.NbColsToOpen(); col++ {
		openedCol := ctx.comp.InsertProof(
			lastRound+2,
			ctx.SelectedColName(col),
			numRows,
		)
		ctx.Items.OpenedColumns = append(ctx.Items.OpenedColumns, openedCol)
	}

	// In case of the Merkle-proof mode, we also registers the
	// column that will contain the Merkle proofs altogether. But
	// first, we need to evaluate its size. The proof size needs to
	// be padded up to a power of two. Otherwise, we can't use PeriodicSampling.
	ctx.Items.MerkleProofs = ctx.comp.InsertProof(
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

// Create a method to decide when to commit to the precomputed
func (ctx *Ctx) IsCommitToPrecomputed() bool {
	return len(ctx.Items.Precomputeds.PrecomputedColums) > ctx.DryTreshold
}

// Turns the precomputed into verifying key messages. A possible improvement
// would be to make them an entire commitment but we estimate that it will not
// be worth it. If the flag `CommitPrecomputed` is set to `true`, this will
// instead register the precomputed columns.
func (ctx *Ctx) processStatusPrecomputed() {

	var (
		comp = ctx.comp
	)

	// This captures the precomputed column.  It is essential to do it this
	// particular moment, because the number of precomputed columns is going to
	// change during the compilation time (in later compilation stage). And
	// we want the precomputed columns defined at the beginning of the current
	// vortex compilation step to be captured only.
	precomputedColNames := comp.Columns.AllPrecomputed()
	if len(precomputedColNames) == 0 {
		ctx.Items.Precomputeds.PrecomputedColums = []ifaces.Column{}
		return
	}

	// Sanity-check. This should be enforced by the splitter compiler already.
	ctx.assertPolynomialHaveSameLength(precomputedColNames)
	precomputedCols := []ifaces.Column{}

	// If there are not enough columns, for a commitment to be meaningful,
	// explicitly sends the column to the verifier so that he can check the
	// evaluation by itself.
	if len(precomputedColNames) < ctx.DryTreshold {
		for _, name := range precomputedColNames {
			comp.Columns.SetStatus(name, column.VerifyingKey)
			pCol := comp.Columns.GetHandle(name)

			_, ok := ctx.PolynomialsTouchedByTheQuery[name]
			if !ok {
				comp.Columns.MarkAsIgnored(name)
				continue
			}

			precomputedCols = append(precomputedCols, pCol)
		}

		// For consistency, with the older version of the code
		ctx.Items.Precomputeds.PrecomputedColums = precomputedCols
		logrus.Infof("We are not committing to the %d precomputeds", len(precomputedColNames))
		return
	}

	for _, name := range precomputedColNames {

		_, ok := ctx.PolynomialsTouchedByTheQuery[name]
		if !ok {
			logrus.Warnf("got an unconstrained column: %v -> marking as ignored", name)
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

	logrus.Infof("Processing %v precomputed columns", len(precomputedCols))

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
	if ctx.SisParams != nil {

		var (
			sisDegree          = ctx.SisParams.OutputSize()
			sisNumLimbs        = ctx.SisParams.NumLimbs()
			sisNumFieldPerPoly = utils.Max(1, sisDegree/sisNumLimbs)
			numShadowRows      = sisNumFieldPerPoly - (len(precomputedCols) % sisNumFieldPerPoly)
		)

		if sisDegree > sisNumLimbs && numShadowRows > 0 {
			for i := 0; i < numShadowRows; i++ {
				// The shift by 20 is to avoid collision with the committed cols
				// at round zero.
				shadowCol := autoAssignedShadowRow(comp, ctx.NumCols, 0, 1<<20+i)
				ctx.ShadowCols[shadowCol.GetColID()] = struct{}{}
				precomputedColNames = append(precomputedColNames, shadowCol.GetColID())
				precomputedCols = append(precomputedCols, shadowCol)
			}
		}

	}

	logrus.Infof("Processed %v precomputed columns", len(precomputedCols))
	ctx.Items.Precomputeds.PrecomputedColums = precomputedCols
}

// Returns the number of committed rounds. Must be called after the
// method compileRound has been executed. Otherwise, it will output zero.
func (ctx *Ctx) NumCommittedRounds() int {
	res := 0

	// MaxCommittedRounds is unset if the function is called before
	// the compileRound method. Careful, the stopping condition is
	// an LE and not a strict LT condition.
	for i := 0; i <= ctx.MaxCommittedRound; i++ {
		if ctx.isDry(i) {
			continue
		}
		res++
	}

	return res
}

// MerkleProofSize Returns the size of the allocated Merkle proof vector
func (ctx *Ctx) MerkleProofSize() int {
	// In case of the Merkle-proof mode, we also registers the
	// column that will contain the Merkle proofs altogether. But
	// first, we need to evaluate its size. The proof size needs to
	// be padded up to a power of two. Otherwise, we can't use PeriodicSampling.
	depth := utils.Log2Ceil(ctx.NumEncodedCols())
	numComs := ctx.NumCommittedRounds()
	// The number of rounds increases by 1 when we commit to the precomputeds
	if ctx.IsCommitToPrecomputed() {
		numComs += 1
	}
	numOpening := ctx.NbColsToOpen()

	if depth*numComs*numOpening == 0 {
		utils.Panic("something was zero : %v, %v, %v", depth, numComs, numOpening)
	}

	return utils.NextPowerOfTwo(depth * numComs * numOpening)
}

// Commit to the precomputed columns
func (ctx *Ctx) commitPrecomputeds() {
	precomputeds := ctx.Items.Precomputeds.PrecomputedColums
	numPrecomputeds := len(precomputeds)

	// This can happens if either the flag `CommitPrecomputeds` is
	// unset or if there is not enough precomputed columns to make
	// it worth committing to them (e.g. less precomputed columns than
	// the dry threshold).
	if numPrecomputeds == 0 {
		logrus.Tracef("skip commit precomputeds, because either the flag `CommitPrecomputeds`" +
			"is unset or there are less columns than dry threshold.")
		return
	}

	// Fetch the assignments (known at compile time since they are precomputeds)
	pols := make([]smartvectors.SmartVector, numPrecomputeds)
	for i, precomputed := range precomputeds {
		if _, ok := ctx.ShadowCols[precomputed.GetColID()]; ok {
			pols[i] = smartvectors.NewConstant(field.Zero(), ctx.NumCols)
			continue
		}
		pols[i] = ctx.comp.Precomputed.MustGet(precomputed.GetColID())
	}

	// Increase the number of committed rows
	ctx.CommittedRowsCount += numPrecomputeds

	// Call Vortex in Merkle mode
	committedMatrix, tree, colHashes := ctx.VortexParams.CommitMerkle(pols)
	ctx.Items.Precomputeds.DhWithMerkle = colHashes
	ctx.Items.Precomputeds.CommittedMatrix = committedMatrix
	ctx.Items.Precomputeds.tree = tree

	// And assign the 1-sized column to contain the root
	var root field.Element
	root.SetBytes(tree.Root[:])
	ctx.Items.Precomputeds.MerkleRoot = ctx.comp.RegisterVerifyingKey(ctx.PrecomputedMerkleRootName(), smartvectors.NewConstant(root, 1))

}
