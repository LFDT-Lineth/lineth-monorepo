package plonkinternal

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/iop"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/expr_handle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/sirupsen/logrus"
)

// PlonkCheck adds a PLONK circuit in the wizard. Namely, the function takes a
// frontend.Circuit parameter, a PLONK witness assigner (i.e. a function that
// returns the PLONK witness to be used as an input for the solver). It
// compiles the circuit and construct a set of column and constraints reflecting
// the satisfiability of the provided PLONK circuit within the current Wizard.
// This is used for the precompiles and for ECDSA verification since these
// use-cases would require a very complex design if we wanted to implement them
// directly into the Wizard instead of Plonk.
//
// The user can provide one or more assigner for the same circuit to mean that
// we want to call the same circuit multiple times. In this case, the function
// optimizes the generated Wizard to commit only once to the preprocessed
// polynomials (qL, qR, etc...). This additionally allows batching certain parts
// of the protocol such as the copy-constraints argument which will be run only
// once over a random linear combination of the witnesses.
//
// The user can provide an identifying string `name` to the function. The name
// will be appended to all generated columns and queries name to carry some
// context to where these queries and columns come from.
func PlonkCheck(
	comp *wizard.CompiledIOP,
	name string,
	Round int,
	circuit frontend.Circuit,
	maxNbInstance int,
	// function to call to get an assignment
	options ...Option,
) *CompilationCtx {

	// Create the ctx
	ctx := createCtx(comp, name, Round, circuit, maxNbInstance, options...)

	// And registers the columns + constraints
	ctx.commitGateColumns()
	ctx.extractPermutationColumns()
	ctx.addCopyConstraint()
	ctx.addGateConstraint()

	if ctx.RangeCheckOption.Enabled {
		ctx.addRangeCheckConstraint()
	}

	if ctx.ExternalHasherOption.Enabled {
		ctx.addHashConstraint()
	}

	if ctx.HasCommitment() {
		comp.RegisterProverAction(Round+1, LROCommitProverAction{
			GenericPlonkProverAction: ctx.GenericPlonkProverAction(),
			ProverStateLock:          &sync.Mutex{},
		})
	}

	comp.RegisterVerifierAction(Round, &CheckingActivators{Cols: ctx.Columns.Activators})

	logrus.
		WithField("nbConstraints", ctx.Plonk.SPR.NbConstraints).
		WithField("MaxNbInstances", maxNbInstance).
		WithField("name", name).
		WithField("hasCommitment", ctx.HasCommitment()).
		Info("compiled Plonk in Wizard circuit")

	return &ctx
}

// This function registers the Plonk gate's columns inside of the wizard. It
// does not add any constraints whatsoever.
func (ctx *CompilationCtx) commitGateColumns() {

	nbRow := ctx.DomainSize()

	// Declare and pre-assign the selector columns
	ctx.Columns.Ql = ctx.comp.InsertPrecomputed(ctx.colIDf("QL"), iopToSV(ctx.Plonk.trace.Ql, nbRow))
	ctx.Columns.Qr = ctx.comp.InsertPrecomputed(ctx.colIDf("QR"), iopToSV(ctx.Plonk.trace.Qr, nbRow))
	ctx.Columns.Qo = ctx.comp.InsertPrecomputed(ctx.colIDf("QO"), iopToSV(ctx.Plonk.trace.Qo, nbRow))
	ctx.Columns.Qm = ctx.comp.InsertPrecomputed(ctx.colIDf("QM"), iopToSV(ctx.Plonk.trace.Qm, nbRow))
	ctx.Columns.Qk = ctx.comp.InsertPrecomputed(ctx.colIDf("QK"), iopToSV(ctx.Plonk.trace.Qk, nbRow))

	// Declare and pre-assign the rangecheck selectors
	if ctx.RangeCheckOption.Enabled && !ctx.RangeCheckOption.wasCancelled {
		PcRcL, PcRcR, PcRcO := ctx.rcGetterToSV()
		ctx.RangeCheckOption.RcL = ctx.comp.InsertPrecomputed(ctx.colIDf("RcL"), smartvectors.RightZeroPadded(PcRcL, nbRow))
		ctx.RangeCheckOption.RcR = ctx.comp.InsertPrecomputed(ctx.colIDf("RcR"), smartvectors.RightZeroPadded(PcRcR, nbRow))
		ctx.RangeCheckOption.RcO = ctx.comp.InsertPrecomputed(ctx.colIDf("RcO"), smartvectors.RightZeroPadded(PcRcO, nbRow))
	}

	ctx.Columns.L = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.R = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.O = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.Activators = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.PI = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.TinyPI = make([]ifaces.Column, ctx.MaxNbInstances)
	ctx.Columns.Cp = make([]ifaces.Column, ctx.MaxNbInstances)

	if ctx.HasCommitment() {
		// Selector for the commitment
		ctx.Columns.Qcp = ctx.comp.InsertPrecomputed(ctx.colIDf("QCP"), iopToSV(ctx.Plonk.trace.Qcp[0], nbRow))

		// First Round, for the committed value and the PI
		for i := 0; i < ctx.MaxNbInstances; i++ {
			if tinyPISize(ctx.Plonk.SPR) > 0 {
				ctx.Columns.TinyPI[i] = ctx.comp.InsertProof(ctx.Round, ctx.colIDf("PI_%v", i), tinyPISize(ctx.Plonk.SPR), true)
				ctx.Columns.PI[i] = verifiercol.NewConcatTinyColumns(ctx.comp, nbRow, fext.Zero(), ctx.Columns.TinyPI[i])
			} else {
				ctx.Columns.PI[i] = verifiercol.NewConstantCol(field.Zero(), nbRow, "")
			}
			ctx.Columns.Cp[i] = ctx.comp.InsertCommit(ctx.Round, ctx.colIDf("Cp_%v", i), nbRow, true)
			ctx.Columns.Activators[i] = ctx.comp.InsertProof(ctx.Round, ctx.colIDf("ACTIVATOR_%v", i), 1, true)
		}

		// Second rounds, after sampling HCP
		ctx.Columns.Hcp = ctx.comp.InsertCoin(ctx.Round+1, coin.Name(ctx.Sprintf("HCP")), coin.FieldExt)
		ctx.Columns.HcpEl = make([]ifaces.Column, fext.ExtensionDegree)
		for i := 0; i < fext.ExtensionDegree; i++ {
			ctx.Columns.HcpEl[i] = ctx.comp.InsertCommit(ctx.Round+1, ctx.colIDf("HCP_BASE_%v", i), nbRow, true)
		}

		// And assigns the LRO polynomials
		for i := 0; i < ctx.MaxNbInstances; i++ {
			ctx.Columns.L[i] = ctx.comp.InsertCommit(ctx.Round+1, ctx.colIDf("L_%v", i), nbRow, true)
			ctx.Columns.R[i] = ctx.comp.InsertCommit(ctx.Round+1, ctx.colIDf("R_%v", i), nbRow, true)
			ctx.Columns.O[i] = ctx.comp.InsertCommit(ctx.Round+1, ctx.colIDf("O_%v", i), nbRow, true)
		}
	} else {
		// Else no additional selector, and just commit to LRO + PI at the same Round
		for i := 0; i < ctx.MaxNbInstances; i++ {
			if tinyPISize(ctx.Plonk.SPR) > 0 {
				ctx.Columns.TinyPI[i] = ctx.comp.InsertProof(ctx.Round, ctx.colIDf("PI_%v", i), tinyPISize(ctx.Plonk.SPR), true)
				ctx.Columns.PI[i] = verifiercol.NewConcatTinyColumns(ctx.comp, nbRow, fext.Zero(), ctx.Columns.TinyPI[i])
			} else {
				ctx.Columns.PI[i] = verifiercol.NewConstantCol(field.Zero(), nbRow, "")
			}
			ctx.Columns.L[i] = ctx.comp.InsertCommit(ctx.Round, ctx.colIDf("L_%v", i), nbRow, true)
			ctx.Columns.R[i] = ctx.comp.InsertCommit(ctx.Round, ctx.colIDf("R_%v", i), nbRow, true)
			ctx.Columns.O[i] = ctx.comp.InsertCommit(ctx.Round, ctx.colIDf("O_%v", i), nbRow, true)
			ctx.Columns.Activators[i] = ctx.comp.InsertColumn(ctx.Round, ctx.colIDf("ACTIVATOR_%v", i), 1, column.Proof, true)
		}
	}
}

// Returns an smart-vector from an iop.Polynomial. If nbRow is specified to
// be greater than the length of Pol, then the function returns a zero-padded
// smartvector.
func iopToSV(pol *iop.Polynomial, nbRow int) smartvectors.SmartVector {
	return smartvectors.RightZeroPadded(pol.Coefficients(), nbRow)
}

// This function constructs the PcRcL and PcRcR vectors. It is used by the
// external range-checking mechanism. Namely, the two constructed vectors are
// binary vectors containing a 1 to indicate that a wire is to be range-checked
// and 0 to indicate that it can be ignored.
//
// For instance, if it PcRcL[56] == 1, it means that the 56-th position of the
// PLONK column xA needs to be range-checked. For PcRcR, it indicates the
// relevant positions in the xB PLONK column.
//
// This function works by calling the RcGetter (see
// [plonk.newExternalRangeChecker] to obtain the result in "[]field.Element"
// form and then it converts it into assignable smartvectors after having
// checked a few hypothesis.
func (ctx *CompilationCtx) rcGetterToSV() (PcRcL, PcRcR, PcRcO []field.Element) {
	v := [3][]field.Element{
		make([]field.Element, ctx.DomainSize()),
		make([]field.Element, ctx.DomainSize()),
		make([]field.Element, ctx.DomainSize()),
	}
	sls := ctx.Plonk.rcGetter()
	for _, ss := range sls {
		v[ss[1]][ss[0]].SetInt64(1)
	}

	return v[0], v[1], v[2]
}

// extractPermutationColumns computes and tracks the values for ctx.Columns.S
func (ctx *CompilationCtx) extractPermutationColumns() {
	for i := range ctx.Columns.S {
		// Directly use the ints from the trace instead of the fresh Plonk ones
		si := ctx.Plonk.trace.S[i*ctx.DomainSize() : (i+1)*ctx.DomainSize()]
		sField := make([]field.Element, len(si))
		for j := range sField {
			sField[j].SetInt64(si[j])
		}

		// Track it, no need to register it since the compiler
		// will do it on its own.
		ctx.Columns.S[i] = smartvectors.NewRegular(sField)
	}
}

// add gate constraint
func (ctx *CompilationCtx) addGateConstraint() {

	for i := 0; i < ctx.MaxNbInstances; i++ {

		// Declare the expression
		exp := sym.Add(
			sym.Mul(ctx.Columns.L[i], ctx.Columns.Ql),
			sym.Mul(ctx.Columns.R[i], ctx.Columns.Qr),
			sym.Mul(ctx.Columns.O[i], ctx.Columns.Qo),
			sym.Mul(ctx.Columns.L[i], ctx.Columns.R[i], ctx.Columns.Qm),
			ctx.Columns.PI[i],
			ctx.Columns.Qk,
		)

		roundLRO := ctx.Round

		// Optionally add a commitment
		if ctx.HasCommitment() {
			// full length of a column
			fullLength := ctx.Columns.PI[i].Size()
			commitmentInfo := ctx.CommitmentInfo()
			hcpPosition := commitmentInfo.CommitmentIndex + ctx.Plonk.SPR.GetNbPublicVariables()
			// the random coin is an extension field element, but gnark circuit
			// expects to have as a vector of base field elements we have
			// already decomposed the random coin into base field elements in
			// [ctx.Columns.HcpEl]. We now assert that the committed value is
			// equal to the random coin.
			cmtOutExp := sym.NewConstant(0)
			for j := range fext.ExtensionDegree {
				cmtOutExp = sym.Add(
					cmtOutExp,
					sym.Mul(
						// selector indicating the row where the commitmend output is stored as random coin.
						// equivalent to using Lagrange
						variables.NewPeriodicSample(fullLength, hcpPosition+j),
						// random coin value (base field element)
						ctx.Columns.HcpEl[j],
					),
				)
			}
			// additionally, we need to assert that the decomposition of
			// extension field element to base field element is correct
			ctx.comp.InsertGlobal(
				roundLRO+1,
				ctx.queryIDf("HCP_DECOMPOSITION_%v", i),
				sym.Sub(
					ctx.Columns.Hcp,
					sym.Mul(ctx.Columns.HcpEl[0], fext.NewFromUint(1, 0, 0, 0)),
					sym.Mul(ctx.Columns.HcpEl[1], fext.NewFromUint(0, 1, 0, 0)),
					sym.Mul(ctx.Columns.HcpEl[2], fext.NewFromUint(0, 0, 1, 0)),
					sym.Mul(ctx.Columns.HcpEl[3], fext.NewFromUint(0, 0, 0, 1)),
				),
			)

			exp = sym.Add(
				exp,
				sym.Mul(ctx.Columns.Cp[i], ctx.Columns.Qcp),
				cmtOutExp,
			)

			// increase the LRO
			roundLRO++
		}
		// And registers the gate expression as a global variable
		ctx.comp.InsertGlobal(
			roundLRO,
			ctx.queryIDf("GATE_CS_INSTANCE_%v", i),
			sym.Mul(
				exp,
				// The conversion into an activator is required for the system
				// to understand that the expression is multiplied by a scalar
				// and not by a wrongly-sized constructed column
				accessors.NewFromPublicColumn(ctx.Columns.Activators[i], 0),
			),
		)
	}
}

// add add the copy constraint
func (ctx *CompilationCtx) addCopyConstraint() {

	// Creates a special handle for the permutation by
	// computing a linear combination of the columns
	var l, r, o ifaces.Column
	roundPermutation := ctx.Columns.L[0].Round()

	if len(ctx.Columns.L) == 1 {
		// then just pass the first column
		l = ctx.Columns.L[0]
		r = ctx.Columns.R[0]
		o = ctx.Columns.O[0]
	} else {
		// other run the permutation only once
		// over a linear combination of the columns
		roundPermutation++
		// declare the coin
		randLin := ctx.comp.InsertCoin(
			roundPermutation,
			coin.Name(ctx.Sprintf("PERMUTATION_RANDLIN")),
			coin.FieldExt,
		)
		// And declare special columns for the linear combination
		l = expr_handle.RandLinCombCol(
			ctx.comp,
			accessors.NewFromCoin(randLin),
			ctx.Columns.L,
			ctx.Sprintf("L_PERMUT_LINCOMB"),
		)
		r = expr_handle.RandLinCombCol(
			ctx.comp,
			accessors.NewFromCoin(randLin),
			ctx.Columns.R,
			ctx.Sprintf("R_PERMUT_LINCOMB"),
		)
		o = expr_handle.RandLinCombCol(
			ctx.comp,
			accessors.NewFromCoin(randLin),
			ctx.Columns.O,
			ctx.Sprintf("O_PERMUT_LINCOMB"),
		)

	}

	// No need to commit to the permutation S =(s1,s2,s3),
	// as it is commited by FixedPermutation
	ctx.comp.InsertFixedPermutation(
		roundPermutation,
		ctx.queryIDf("PLONK_COPY_CS"),
		ctx.Columns.S[:],
		[]ifaces.Column{l, r, o},
		[]ifaces.Column{l, r, o},
	)
}

// CheckingActivators implements the [wizard.VerifierAction] interface and
// checks that the [Activators] columns are correctly assigned
type CheckingActivators struct {
	Cols    []ifaces.Column
	skipped bool `serde:"omit"`
}

var _ wizard.VerifierAction = &CheckingActivators{}

func (ca *CheckingActivators) Run(run wizard.Runtime) error {
	for i := range ca.Cols {

		curr := ca.Cols[i].GetColAssignmentAt(run, 0)
		if !curr.IsOne() && !curr.IsZero() {
			return fmt.Errorf("error the activators must be 0 or 1")
		}

		if i+1 < len(ca.Cols) {
			next := ca.Cols[i+1].GetColAssignmentAt(run, 0)
			if curr.IsZero() && !next.IsZero() {
				return fmt.Errorf("the activators must never go from 0 to 1")
			}
		}
	}

	return nil
}

func (ca *CheckingActivators) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	koalaApi := koalagnark.NewAPI(api)

	for i := range ca.Cols {
		curr := ca.Cols[i].GetColAssignmentGnarkAt(run, 0)
		koalaApi.AssertIsBoolean(curr)

		if i+1 < len(ca.Cols) {
			next := ca.Cols[i+1].GetColAssignmentGnarkAt(run, 0)
			koalaApi.AssertIsEqual(next, koalaApi.Mul(curr, next))
		}
	}
}

func (ca *CheckingActivators) Skip() {
	ca.skipped = true
}

func (ca *CheckingActivators) IsSkipped() bool {
	return ca.skipped
}
