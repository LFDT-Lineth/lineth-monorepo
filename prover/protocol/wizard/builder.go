package wizard

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

/*
Builder provides the go-to user interface to specify a custom Wizard protocol.
The builder is essentially a wrapper around the [CompiledIOP] struct and
has the additional capability to track the "current" prover-verifier interaction
round.

In particular, Builder provides the utilities to
  - Declare columns
  - Declare random coins
  - Declare queries

@alex: we should deprecate this and directly embed the "round"
tracking capability within the [CompiledIOP] struct. The round-tracking
mechanism does not allow for a smooth way to decompose the user's protocol into
sub-protocols that spans on multiple rounds efficiently as a new round will be
created everytime the user declares a new Coin.
*/
type Builder struct {
	*CompiledIOP
	// Indicate the current round
	currRound int
	/*
		Flag indicating whether the FS state is fresh. If
		`true`, the next call to `RegisterRandomCoin` will
		start a new round and reset it to `false`. Registering
		a new `column` or a new `ParametrizableQuery`
	*/
	fsStateIsDirty bool
}

/*
Function to specify the definition of an IOP
*/
type DefineFunc func(build *Builder)

/*
Compile an IOP from a protocol definition
*/
func Compile(define DefineFunc, compilers ...func(*CompiledIOP)) *CompiledIOP {
	builder := newBuilder()
	define(&builder)
	comp := builder.CompiledIOP
	return ContinueCompilation(comp, compilers...)
}

// ContinueCompilation continues a set of compilation steps over a initial CompiledIOP object.
func ContinueCompilation(rootComp *CompiledIOP, compilers ...func(*CompiledIOP)) *CompiledIOP {
	/*
		For sanity, we need to ensure the protocol is well formed. All
		registers should have the same number of rounds. The simplest to
		iron this out after the define function. We still make sure than
		no more rounds are allocated anywhere.
	*/
	comp := rootComp
	numRounds := comp.NumRounds()

	comp.EqualizeRounds(numRounds)

	for _, compiler := range compilers {
		compiler(comp)
		numRounds := comp.NumRounds()
		comp.EqualizeRounds(numRounds)
	}

	if comp.SubProvers.Len() < comp.NumRounds() {
		utils.Panic("There are coin sampling rounds that are not followed by an action of the prover. numRoundProver=%v numRoundCoins=%v",
			comp.SubProvers.Len(), comp.NumRounds(),
		)
	}

	return comp
}

// NewCompiledIOP initializes a CompiledIOP object.
func NewCompiledIOP() *CompiledIOP {
	CompiledIOP := &CompiledIOP{
		Columns:         column.NewStore(),
		QueriesParams:   NewRegister[ifaces.QueryID, ifaces.Query](),
		QueriesNoParams: NewRegister[ifaces.QueryID, ifaces.Query](),
		Coins:           NewRegister[coin.Name, coin.Info](),
		Precomputed:     collection.NewMapping[ifaces.ColID, ifaces.ColAssignment](),
		ExtraData:       make(map[string]interface{}),
	}
	return CompiledIOP
}

/*
Creates a new builder for a new IOP
*/
func newBuilder() Builder {
	return Builder{
		CompiledIOP:    NewCompiledIOP(),
		currRound:      0,
		fsStateIsDirty: true,
	}
}

/*
Registers a new column in the protocol
*/
func (b *Builder) RegisterCommit(name ifaces.ColID, size int) ifaces.Column {
	b.fsStateIsDirty = true
	return b.CompiledIOP.InsertCommit(b.currRound, name, size, true)
}

// RegisterCommitExt registers a new column in the protocol and specified the
// column should contain field extensions.
func (b *Builder) RegisterCommitExt(name ifaces.ColID, size int) ifaces.Column {
	b.fsStateIsDirty = true
	return b.CompiledIOP.InsertCommit(b.currRound, name, size, false)
}

/*
Registers a precomputed column in the protocol
*/
func (b *Builder) RegisterPrecomputed(name ifaces.ColID, v smartvectors.SmartVector) ifaces.Column {
	b.fsStateIsDirty = true
	return b.CompiledIOP.InsertPrecomputed(name, v)
}

/*
Asserts there will be a Fiat-Shamir hash

(for integer vec coin only, the caller must pass a slice of length 2 such that
- size[0] contains the number of integers and
- size[1] contains the upperBound.
*/
func (b *Builder) RegisterRandomCoin(name coin.Name, type_ coin.Type, size ...int) coin.Info {
	/*
		The fact that the fsStateIsDirty indicates that something
		was sent to the verifier since the last message. Thus it
	*/
	if b.fsStateIsDirty {
		b.currRound++
		b.fsStateIsDirty = false
	}

	/*
		Sanity-check : Not that it is necessarily an error it is a
		little strange to see a random coin happening ex-nihilo
		since it does not bind to anything.
	*/
	if b.currRound == 0 {
		utils.Panic("Random coin ex-nihilo. Probably an error.")
	}

	// And insert the random coin
	return b.CompiledIOP.InsertCoin(b.currRound, name, type_, size...)
}

/*
Create a univariate query for a list of already registered polynomials.
The witnesses here are assumed to be in COEFFICIENT FORM. It is important
to note, that this function assumes that, `X`, the evaluation point is
**unique** and **not known yet** (it could be a random coin challenge
for instance). If you want to register a query for which the evaluation point
is already known, you should use `FixedPointUnivariateEval` instead. If you
would like to do a multi-evaluation instead, you need to register several
queries
*/
func (b *Builder) UnivariateEval(name ifaces.QueryID, pols ...ifaces.Column) {
	// Mark the state as dirty
	b.fsStateIsDirty = true
	b.InsertUnivariate(b.currRound, name, pols)

}

/*
Creates an inclusion query. Here, `included` and `including` are viewed
as a arrays and the query asserts that `included` contains only rows
that are contained within `includings`, regardless of the multiplicity.
*/
func (b *Builder) Inclusion(name ifaces.QueryID, including, included []ifaces.Column) {
	b.InsertInclusion(b.currRound, name, including, included)
}

/*
An inclusion query that adds two filters on the including and included arrays
The filters should be columns that contain only field elements for 0 and 1.
*/
func (b *Builder) InclusionDoubleConditional(name ifaces.QueryID, including, included []ifaces.Column, includingFilter, includedFilter ifaces.Column) {
	b.InsertInclusionDoubleConditional(b.currRound, name, including, included, includingFilter, includedFilter)
}

/*
An inclusion query that adds a filter on the including array
The filter should be a column that contains only field elements for 0 and 1.
*/
func (b *Builder) InclusionConditionalOnIncluding(name ifaces.QueryID, including, included []ifaces.Column, includingFilter ifaces.Column) {
	b.InsertInclusionConditionalOnIncluding(b.currRound, name, including, included, includingFilter)
}

/*
An inclusion query that adds a filter on the included array
The filter should be a column that contains only field elements for 0 and 1.
*/
func (b *Builder) InclusionConditionalOnIncluded(name ifaces.QueryID, including, included []ifaces.Column, includedFilter ifaces.Column) {
	b.InsertInclusionConditionalOnIncluded(b.currRound, name, including, included, includedFilter)
}

/*
Creates an permutation query. The query views `a` and `b_` to be lists of
columns and asserts that `a` and `b_` have the same rows (possibly in
a different order) but with the same multiplicity.
*/
func (b *Builder) Permutation(name ifaces.QueryID, a, b_ []ifaces.Column) {
	b.CompiledIOP.InsertPermutation(b.currRound, name, a, b_)
}

/*
Creates a fixed-permutation query. Were 'a' is the fixedpermutation of 'b' for
a given-permutation p: p(a)=b, p can be deifed only by 'b' over a defult vector 'a'.
*/
func (b *Builder) FixedPermutation(name ifaces.QueryID, p []ifaces.ColAssignment, a, b_ []ifaces.Column) {
	b.CompiledIOP.InsertFixedPermutation(b.currRound, name, p, a, b_)
}

/*
Create an GlobalConstraint query, returns the global constraint
*/
func (b *Builder) GlobalConstraint(name ifaces.QueryID, cs_ *symbolic.Expression) query.GlobalConstraint {
	// Finally registers the query
	// This will perform all the checks
	cs_.AssertValid()
	return b.InsertGlobal(b.currRound, name, cs_)
}

/*
Create an LocalConstraint query
*/
func (b *Builder) LocalConstraint(name ifaces.QueryID, cs_ *symbolic.Expression) query.LocalConstraint {
	// Finally registers the query
	// This will perform all the checks
	cs_.AssertValid()
	return b.InsertLocal(b.currRound, name, cs_)
}

/*
Create a Range query
*/
func (b *Builder) Range(name ifaces.QueryID, h ifaces.Column, max int) {
	b.InsertRange(b.currRound, name, h, max)
}

/*
Create an inner-product query
*/
func (b *Builder) InnerProduct(name ifaces.QueryID, a ifaces.Column, bs ...ifaces.Column) query.InnerProduct {
	return b.InsertInnerProduct(b.currRound, name, a, bs)
}

/*
Create a local opening query
*/
func (b *Builder) LocalOpening(name ifaces.QueryID, pol ifaces.Column) query.LocalOpening {
	return b.InsertLocalOpening(b.currRound, name, pol)
}

/*
Equalizes the length of all the structure so that they all have the same
numbers of rounds
*/
func (comp *CompiledIOP) EqualizeRounds(numRounds int) {

	helpMsg := "If you are seeing this message it's probably because you insert queries one round too late."

	/*
		Check and reserve the coins
	*/
	if comp.Coins.NumRounds() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the coins. %v", numRounds, comp.Coins.NumRounds(), helpMsg)
	}
	comp.Coins.ReserveFor(numRounds)

	/*
		Check and reserve for the columns
	*/
	if comp.Columns.NumRounds() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the column. %v", numRounds, comp.Columns.NumRounds(), helpMsg)
	}
	comp.Columns.ReserveFor(numRounds)

	/*
		Check and reserve for queries that don't take runtime parameters
	*/
	if comp.QueriesNoParams.NumRounds() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the QueriesNoParams. %v", numRounds, comp.QueriesNoParams.NumRounds(), helpMsg)
	}
	comp.QueriesNoParams.ReserveFor(numRounds)

	/*
		Check and reserve for the queries that takes runtime parameters
	*/
	if comp.QueriesParams.NumRounds() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the QueriesParams. %v", numRounds, comp.QueriesParams.NumRounds(), helpMsg)
	}
	comp.QueriesParams.ReserveFor(numRounds)

	/*
		Check and reserve for the provers
	*/
	if comp.SubProvers.Len() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the prover. %v", numRounds, comp.SubProvers.Len(), helpMsg)
	}
	comp.SubProvers.Reserve(numRounds)

	/*
		Check and reserve for the verifiers
	*/
	if comp.SubVerifiers.Len() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the verifier. %v", numRounds, comp.SubVerifiers.Len(), helpMsg)
	}
	comp.SubVerifiers.Reserve(numRounds)

	/*
		Check and reserve for the FiatShamirHooksPreSampling
	*/
	if comp.FiatShamirHooksPreSampling.Len() > numRounds {
		utils.Panic("Bug : numRounds is %v but %v rounds are registered for the FiatShamirHooksPreSampling. %v", numRounds, comp.FiatShamirHooksPreSampling.Len(), helpMsg)
	}
	comp.FiatShamirHooksPreSampling.Reserve(numRounds)
}
