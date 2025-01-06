package fullrecursion

import (
	"strconv"

	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// FullRecursion "recurses" the wizard protocol by wrapping all the verifier
// steps in a Plonk-in-Wizard context as well as all the Proof columns. The
// Vortex PCS verification is done via self-recursion.
func FullRecursion(withoutGkr bool) func(comp *wizard.CompiledIOP) {

	return func(comp *wizard.CompiledIOP) {
		var (
			ctx   = captureCtx(comp)
			c     = allocateGnarkCircuit(comp, ctx)
			numPI = len(c.ctx.NonEmptyMerkleRootPositions) +
				len(c.Pubs) +
				len(c.Ys) +
				3 // (1.) for X (2.) for the initial FS state (3.) for the final state
			funcPiOffset = 3 + len(ctx.NonEmptyMerkleRootPositions) + len(ctx.PolyQuery.Pols)
		)

		selfrecursion.SelfRecurse(comp)

		piw := plonk.PlonkCheck(comp, "full-recursion-"+strconv.Itoa(comp.SelfRecursionCount), ctx.LastRound, c, 1)

		ctx.PlonkInWizard.PI = piw.ConcatenatedTinyPIs(utils.NextPowerOfTwo(numPI))
		ctx.PlonkInWizard.ProverAction = piw.GetPlonkProverAction()

		for i := 0; i < numPI; i++ {

			var (
				pi = ctx.PlonkInWizard.PI
				lo = comp.InsertLocalOpening(
					ctx.PlonkInWizard.PI.Round(),
					ifaces.QueryIDf("%v_LO_%v", pi.String(), i),
					column.Shift(pi, i),
				)
			)

			ctx.LocalOpenings = append(ctx.LocalOpenings, lo)
		}

		for i := range comp.PublicInputs {
			comp.PublicInputs[i].Acc = accessors.NewLocalOpeningAccessor(
				ctx.LocalOpenings[funcPiOffset+i],
				ctx.PlonkInWizard.PI.Round(),
			)
		}

		comp.FiatShamirHooks.AppendToInner(ctx.LastRound, &ResetFsActions{fullRecursionCtx: *ctx})
		comp.RegisterProverAction(ctx.LastRound, CircuitAssignment(*ctx))
		comp.RegisterProverAction(ctx.LastRound, ReplacementAssignment(*ctx))
		comp.RegisterProverAction(ctx.PlonkInWizard.PI.Round(), LocalOpeningAssignment(*ctx))
		comp.RegisterVerifierAction(ctx.PlonkInWizard.PI.Round(), &ConsistencyCheck{fullRecursionCtx: *ctx})
	}
}

// fullRecursionCtx holds compilation context informations about the wizard
// protocol being compiled by a FullRecursion routine.
type fullRecursionCtx struct {
	// A pointer to the compiled-IOP over which the compilation step has run
	Comp *wizard.CompiledIOP
	// The Vortex compilation context
	PcsCtx                      *vortex.Ctx
	PublicInputs                []wizard.PublicInput
	PolyQuery                   query.UnivariateEval
	PolyQueryReplacement        query.UnivariateEval
	MerkleRootsReplacement      []ifaces.Column
	NonEmptyMerkleRootPositions []int
	FirstRound, LastRound       int
	QueryParams                 [][]ifaces.Query
	Columns                     [][]ifaces.Column
	VerifierActions             [][]wizard.VerifierAction
	Coins                       [][]coin.Info
	FsHooks                     [][]wizard.VerifierAction
	PlonkInWizard               struct {
		ProverAction plonk.PlonkInWizardProverAction
		PI           ifaces.Column
	}
	LocalOpenings []query.LocalOpening
}

// captureCtx scans the content of comp to store the compilation infos of the
// CompiledIOP at the beginning of the compilation.
func captureCtx(comp *wizard.CompiledIOP) *fullRecursionCtx {

	var (
		polyQuery = comp.PcsCtxs.(*vortex.Ctx).Query
		lastRound = comp.QueriesParams.Round(polyQuery.QueryID)
		ctx       = &fullRecursionCtx{
			Comp:         comp,
			PcsCtx:       comp.PcsCtxs.(*vortex.Ctx),
			PolyQuery:    polyQuery,
			LastRound:    lastRound,
			FirstRound:   lastRound,
			PublicInputs: append([]wizard.PublicInput{}, comp.PublicInputs...),
		}
	)

	for round := 0; round <= lastRound; round++ {

		ctx.QueryParams = append(ctx.QueryParams, []ifaces.Query{})
		ctx.Columns = append(ctx.Columns, []ifaces.Column{})
		ctx.VerifierActions = append(ctx.VerifierActions, []wizard.VerifierAction{})
		ctx.Coins = append(ctx.Coins, []coin.Info{})
		ctx.FsHooks = append(ctx.FsHooks, []wizard.VerifierAction{})

		for _, colName := range comp.Columns.AllKeysAt(round) {

			// filter the columns by status
			var (
				status = comp.Columns.Status(colName)
				col    = comp.Columns.GetHandle(colName)
			)

			if !status.IsPublic() {
				// the column is not public so it is not part of the proof
				continue
			}

			if status == column.VerifyingKey {
				// these are constant columns
				continue
			}

			ctx.FirstRound = min(ctx.FirstRound, round)
			ctx.Columns[round] = append(ctx.Columns[round], col)
			comp.Columns.IgnoreButKeepInProverTranscript(colName)
		}

		for _, qName := range comp.QueriesParams.AllKeysAt(round) {

			if comp.QueriesParams.IsSkippedFromVerifierTranscript(qName) {
				continue
			}

			// Not that we do not filter the already compiled queries
			qInfo := comp.QueriesParams.Data(qName)
			ctx.QueryParams[round] = append(ctx.QueryParams[round], qInfo)
			comp.QueriesParams.MarkAsSkippedFromVerifierTranscript(qName)
		}

		for _, cname := range comp.Coins.AllKeysAt(round) {

			if comp.Coins.IsSkippedFromVerifierTranscript(cname) {
				continue
			}

			coin := comp.Coins.Data(cname)
			ctx.Coins[round] = append(ctx.Coins[round], coin)
			comp.Coins.MarkAsSkippedFromVerifierTranscript(cname)
		}

		verifierActions := comp.SubVerifiers.Inner()

		for i := range verifierActions[round] {

			va := verifierActions[round][i]
			if va.IsSkipped() {
				continue
			}

			ctx.VerifierActions[round] = append(ctx.VerifierActions[round], va)
			va.Skip()
		}

		if comp.FiatShamirHooks.Len() > round {
			resetFs := comp.FiatShamirHooks.Inner()[round]
			for i := range resetFs {

				fsHook := resetFs[i]
				if fsHook.IsSkipped() {
					continue
				}

				ctx.FsHooks[round] = append(ctx.VerifierActions[round], fsHook)
				fsHook.Skip()
			}
		}
	}

	comp.QueriesParams.MarkAsSkippedFromProverTranscript(polyQuery.QueryID)

	ctx.PcsCtx.IsSelfrecursed = true

	pcsCtxReplacement := *ctx.PcsCtx
	pcsCtxReplacement.Items.MerkleRoots = make([]ifaces.Column, len(pcsCtxReplacement.Items.MerkleRoots))

	for i := range pcsCtxReplacement.Items.MerkleRoots {

		if ctx.PcsCtx.Items.MerkleRoots[i] == nil {
			continue
		}

		ctx.NonEmptyMerkleRootPositions = append(ctx.NonEmptyMerkleRootPositions, i)
		pcsCtxReplacement.Items.MerkleRoots[i] = comp.InsertProof(
			ctx.LastRound,
			ctx.PcsCtx.Items.MerkleRoots[i].GetColID()+"_REPLACEMENT",
			1,
		)
	}

	ctx.MerkleRootsReplacement = pcsCtxReplacement.Items.MerkleRoots

	comp.PcsCtxs = &pcsCtxReplacement
	newPolyQuery := comp.InsertUnivariate(
		lastRound,
		polyQuery.QueryID+"_REPLACEMENT",
		polyQuery.Pols,
	)

	comp.QueriesParams.MarkAsIgnored(newPolyQuery.QueryID)

	ctx.PolyQueryReplacement = newPolyQuery
	pcsCtxReplacement.Query = ctx.PolyQueryReplacement

	return ctx
}
