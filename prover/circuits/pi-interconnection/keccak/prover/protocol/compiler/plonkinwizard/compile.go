package plonkinwizard

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	plonkinternal "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/internal/plonkinternal"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Context stores the compilation Context for a single PlonkInWizard query
type Context struct {
	// Q is the query handled by the current compilation context
	Q *query.PlonkInWizard
	// TinyPIs are the columns containing the public inputs of the plonk
	// instances
	TinyPIs []ifaces.Column
	// Activators are the binary length-1 columns that are used to indicate the
	// activation of a particular plonk instance.
	Activators []ifaces.Column
	// PlonkProverAction is the prover action running the plonk circuit solver
	PlonkProverAction plonkinternal.PlonkInWizardProverAction
	// NbPublicVariable stores the number of public variables in the Plonk
	// circuit.
	NbPublicVariable int
	// SelOpenings are the local constraints responsible for
	// checking the activators are well-set w.r.t to the circuit mask
	SelOpenings []query.LocalOpening
	// CircuitMask is a precomputed binary column indicating with 1's
	// the rows where [query.PlonkInWizard.Data] corresponds to "potentially" actual public
	// inputs. It is used to ensure that the selector goes to zero on
	// the right position.
	CircuitMask ifaces.Column
	// StackedCircuitData is the column storing the concatenation of all the
	// public inputs.
	StackedCircuitData dedicated.StackedColumn
	// MinimalRound is the smallest round in which all the compilation assets.
	MinimalRound int
}

// Compile is the default compiler for PlonkInWizard queries. For 99% of the
// use-cases, this will be the one you need. It works by instantiating Plonk
// columns to represent the circuit within the current wizard.
func Compile(comp *wizard.CompiledIOP) {
	compile(comp, 0)
}

// CompileWithMinimalRound applies the same compilation routine as [Compile]
// but it additionally guarantees that all the compilation assets are not added
// BEFORE round `minimalRound`. This is used for the limitless prover to ensure
// that the first round of the GL module only contains columns that are passed
// to the LPP module. If we added column in that round, they would pollute the
// commitment and it would prevent the LPP commitment mechanism from working.
//
// Passing zero yields the exact same result as [Compile].
func CompileWithMinimalRound(minimalRound int) func(comp *wizard.CompiledIOP) {
	return func(comp *wizard.CompiledIOP) {
		compile(comp, minimalRound)
	}
}

func compile(comp *wizard.CompiledIOP, minimalRound int) {

	qNames := comp.QueriesNoParams.AllKeys()
	for i := range qNames {

		// Skip if it was already compiled
		if comp.QueriesNoParams.IsIgnored(qNames[i]) {
			continue
		}

		q, isType := comp.QueriesNoParams.Data(qNames[i]).(*query.PlonkInWizard)

		// Skip if this is not a PlonkInWizard query
		if !isType {
			continue
		}

		compileQuery(comp, q, minimalRound)

		comp.QueriesNoParams.MarkAsIgnored(q.Name())
	}
}

func compileQuery(comp *wizard.CompiledIOP, q *query.PlonkInWizard, minimalRound int) {

	plonkOptions := make([]plonkinternal.Option, len(q.PlonkOptions))
	for i := range plonkOptions {
		// Note: today, there is only one type of PlonkOption but in the
		// the future we might have more.
		plonkOptions[i] = plonkinternal.WithRangecheck(
			q.PlonkOptions[i].RangeCheckNbBits,
			q.PlonkOptions[i].RangeCheckNbLimbs,
			q.PlonkOptions[i].RangeCheckAddGateForRangeCheck,
		)
	}

	var (
		round          = max(q.Data.Round(), q.Selector.Round(), minimalRound)
		maxNbInstances = q.GetMaxNbCircuitInstances()
		plonkCtx       = plonkinternal.PlonkCheck(comp, string(q.ID), round, q.Circuit, maxNbInstances, plonkOptions...)
		ctx            = &Context{
			Q:                 q,
			MinimalRound:      round,
			TinyPIs:           plonkCtx.Columns.TinyPI,
			Activators:        plonkCtx.Columns.Activators,
			PlonkProverAction: plonkCtx.GetPlonkProverAction(),
			NbPublicVariable:  plonkCtx.Plonk.SPR.GetNbPublicVariables(),
		}
	)

	ctx.StackedCircuitData = *dedicated.StackColumn(comp, ctx.TinyPIs)

	checkActivators(comp, ctx)
	checkPublicInputs(comp, ctx)

	comp.RegisterProverAction(round, &CircAssignment{Context: ctx})
}

// checkPublicInputs adds the constraints ensuring that the public inputs are
// consistent with the one of the PlonkCtx.
func checkPublicInputs(comp *wizard.CompiledIOP, ctx *Context) {
	comp.InsertGlobal(
		max(ctx.MinimalRound, ctx.Q.GetRound()),
		ifaces.QueryIDf("%v_PUBLIC_INPUTS", ctx.Q.ID),
		sym.Sub(ctx.Q.Data, ctx.StackedCircuitData.Column),
	)
}

// checkActivators adds the constraints checking the activators are well-set w.r.t
// to the circuit mask column. See [compilationCtx.Columns.Activators].
func checkActivators(comp *wizard.CompiledIOP, ctx *Context) {

	var (
		openings   = make([]query.LocalOpening, ctx.Q.GetMaxNbCircuitInstances())
		mask       = ctx.Q.Selector
		offset     = utils.NextPowerOfTwo(ctx.Q.GetNbPublicInputs())
		activators = ctx.Activators
		round      = max(activators[0].Round(), ctx.MinimalRound)
	)

	for i := range openings {
		openings[i] = comp.InsertLocalOpening(
			round,
			ifaces.QueryIDf("%v_ACTIVATOR_LOCAL_OP_%v", ctx.Q.ID, i),
			column.Shift(mask, i*offset),
		)
	}

	comp.RegisterProverAction(round, &AssignSelOpening{Context: ctx})
	comp.RegisterVerifierAction(round, &CheckActivatorAndMask{Context: ctx})
	ctx.SelOpenings = openings
}
