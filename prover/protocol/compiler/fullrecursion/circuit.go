package fullrecursion

import (
	"fmt"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

type gnarkCircuit struct {
	InitialFsState frontend.Variable   `gnark:",public"`
	FinalFsState   frontend.Variable   `gnark:",public"`
	Commitments    []frontend.Variable `gnark:",public"`
	X              frontend.Variable   `gnark:",public"`
	Ys             []frontend.Variable `gnark:",public"`
	Pubs           []frontend.Variable `gnark:",public"`
	WizardVerifier *wizard.WizardVerifierCircuit
	comp           *wizard.CompiledIOP `gnark:"-"`
	ctx            *fullRecursionCtx   `gnark:"-"`
	withoutGkr     bool                `gnark:"-"`
}

func allocateGnarkCircuit(comp *wizard.CompiledIOP, ctx *fullRecursionCtx) *gnarkCircuit {

	var (
		wizardVerifier = wizard.NewWizardVerifierCircuit()
	)

	for round := range ctx.Columns {
		for _, col := range ctx.Columns[round] {
			wizardVerifier.AllocColumn(col.GetColID(), col.Size())
		}
	}

	for round := range ctx.QueryParams {
		for _, qInfoIface := range ctx.QueryParams[round] {
			switch qInfo := qInfoIface.(type) {
			case query.UnivariateEval:
				wizardVerifier.AllocUnivariateEval(qInfo.QueryID, qInfo)
			case query.InnerProduct:
				wizardVerifier.AllocInnerProduct(qInfo.ID, qInfo)
			case query.LocalOpening:
				wizardVerifier.AllocLocalOpening(qInfo.ID, qInfo)
			}
		}
	}

	wizardVerifier.Spec = comp

	return &gnarkCircuit{
		ctx:            ctx,
		comp:           comp,
		WizardVerifier: wizardVerifier,
		Commitments:    make([]frontend.Variable, len(ctx.NonEmptyMerkleRootPositions)),
		Ys:             make([]frontend.Variable, len(ctx.PolyQuery.Pols)),
		Pubs:           make([]frontend.Variable, len(comp.PublicInputs)),
	}

}

func (c *gnarkCircuit) Define(api frontend.API) error {

	w := c.WizardVerifier

	if c.withoutGkr {
		w.FS = fiatshamir.NewGnarkFiatShamir(api, nil)
	} else {
		w.HasherFactory = gkrmimc.NewHasherFactory(api)
		w.FS = fiatshamir.NewGnarkFiatShamir(api, w.HasherFactory)
	}

	w.FiatShamirHistory = make([][2][]frontend.Variable, c.comp.NumRounds())

	c.generateAllRandomCoins(api)

	for round := 0; round <= c.ctx.LastRound; round++ {
		roundSteps := c.ctx.VerifierActions[round]
		for _, step := range roundSteps {
			step.RunGnark(api, w)
		}
	}

	for i := range c.Pubs {
		api.AssertIsEqual(c.Pubs[i], c.ctx.PublicInputs[i].Acc.GetFrontendVariable(api, w))
	}

	polyParams := w.GetUnivariateParams(c.ctx.PolyQuery.Name())

	api.AssertIsEqual(c.X, polyParams.X)

	for i := range polyParams.Ys {
		api.AssertIsEqual(c.Ys[i], polyParams.Ys[i])
	}

	for i := range c.Commitments {
		pos := c.ctx.NonEmptyMerkleRootPositions[i]
		api.AssertIsEqual(
			c.Commitments[i],
			w.GetColumn(c.ctx.PcsCtx.Items.MerkleRoots[pos].GetColID())[0],
		)
	}

	return nil
}

// generateAllRandomCoins is as [VerifierRuntime.generateAllRandomCoins]. Note
// that the function does create constraints via the hasher factory that is
// inside of `c.FS`.
func (c *gnarkCircuit) generateAllRandomCoins(api frontend.API) {

	var (
		ctx = c.ctx
		w   = c.WizardVerifier
	)

	w.FS.SetState([]frontend.Variable{c.InitialFsState})

	for currRound := 0; currRound <= c.ctx.LastRound; currRound++ {

		initialState := w.FS.State()

		if currRound > 0 {

			toUpdateFS := ctx.Columns[currRound-1]
			for _, msg := range toUpdateFS {
				val := w.GetColumn(msg.GetColID())
				w.FS.UpdateVec(val)
			}

			queries := ctx.QueryParams[currRound-1]
			for _, q := range queries {
				params := w.GetParams(q.Name())
				params.UpdateFS(w.FS)
			}
		}

		for _, info := range ctx.Coins[currRound] {
			switch info.Type {
			case coin.Field:
				value := w.FS.RandomField()
				w.Coins.InsertNew(info.Name, value)
			case coin.IntegerVec:
				value := w.FS.RandomManyIntegers(info.Size, info.UpperBound)
				w.Coins.InsertNew(info.Name, value)
			}
		}

		for _, fsHook := range ctx.FsHooks[currRound] {
			fsHook.RunGnark(api, w)
		}

		w.FiatShamirHistory[currRound] = [2][]frontend.Variable{
			initialState,
			w.FS.State(),
		}
	}

	api.AssertIsEqual(w.FS.State()[0], c.FinalFsState)
}

// AssignGnarkCircuit returns an assignment for the gnark circuit
func AssignGnarkCircuit(ctx *fullRecursionCtx, comp *wizard.CompiledIOP, run *wizard.ProverRuntime) *gnarkCircuit {

	var (
		wizardVerifier = wizard.NewWizardVerifierCircuit()
	)

	for round := range ctx.Columns {
		for _, col := range ctx.Columns[round] {
			wizardVerifier.AssignColumn(col.GetColID(), col.GetColAssignment(run))
		}
	}

	for round := range ctx.QueryParams {
		for _, qInfoIface := range ctx.QueryParams[round] {
			switch qInfo := qInfoIface.(type) {
			case query.UnivariateEval:
				params := run.GetUnivariateParams(qInfo.QueryID)
				wizardVerifier.AssignUnivariateEval(qInfo.QueryID, params)
			case query.InnerProduct:
				params := run.GetInnerProductParams(qInfo.ID)
				wizardVerifier.AssignInnerProduct(qInfo.ID, params)
			case query.LocalOpening:
				params := run.GetLocalPointEvalParams(qInfo.ID)
				wizardVerifier.AssignLocalOpening(qInfo.ID, params)
			}
		}
	}

	c := &gnarkCircuit{
		ctx:            ctx,
		comp:           comp,
		WizardVerifier: wizardVerifier,
		Pubs:           make([]frontend.Variable, len(comp.PublicInputs)),
		Commitments:    make([]frontend.Variable, len(ctx.NonEmptyMerkleRootPositions)),
		// It is important we start from the begining because of the case where
		// we stack several FullRecursion. In that case, the FsHooks are going
		// to automatically set the FsState to the correct value at first round.
		InitialFsState: run.FiatShamirHistory[1][0][0],
		FinalFsState:   run.FiatShamirHistory[ctx.LastRound][1][0],
	}

	polyParams := run.GetUnivariateParams(ctx.PolyQuery.QueryID).GnarkAssign()
	c.X = polyParams.X
	c.Ys = polyParams.Ys

	for i := range c.Pubs {
		c.Pubs[i] = comp.PublicInputs[i].Acc.GetVal(run)
	}

	for i := range c.Commitments {
		pos := ctx.NonEmptyMerkleRootPositions[i]
		c.Commitments[i] = ctx.PcsCtx.Items.MerkleRoots[pos].GetColAssignmentAt(run, 0)
	}

	return c
}

// WitnessAssign is an implementation of the [plonk.WitnessAssigner] and is used to
// generate the assignment of the fullRecursion circuit.
type WitnessAssigner fullRecursionCtx

func (w WitnessAssigner) NumEffWitnesses(_ *wizard.ProverRuntime) int {
	return 1
}

func (w WitnessAssigner) Assign(run *wizard.ProverRuntime, i int) (private, public witness.Witness, err error) {

	if i > 0 {
		panic("only a single witness for the full-recursion")
	}

	var (
		ctx        = fullRecursionCtx(w)
		assignment = AssignGnarkCircuit(&ctx, w.Comp, run)
	)

	witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
	if err != nil {
		return nil, nil, fmt.Errorf("new witness: %W", err)
	}

	pubWitness, err := witness.Public()
	if err != nil {
		return nil, nil, fmt.Errorf("public witness: %w", err)
	}

	return witness, pubWitness, nil
}
