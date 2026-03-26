package wizard

import (
	// "reflect"

	"slices"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/gnarkutil"
)

// CompiledIOP carries a static description of the IOP protocol throughout the
// compilation of the protocol and after the compilation of the protocol. It
// collects the descriptions of the involved columns in protocol, their status
// and their sizes. It also registers all the random challenge coins that the
// verifier of the protocol is expected to send during the verification process.
// Additionally, the CompiledIOP object can register "queries". Queries are an
// indication that something is not proven yet but are expected to be justified
// during the compilation steps. Additionally, the compiled IOP object registers
// the computations of the prover and the verifier at every round of the
// protocol.
//
// CompiledIOP objects should not be directly constructed by the user, which
// should instead implicitly construct it via the [Compile] function and access
// it via the Builder.CompiledIOP object. Namely, the zero value of the
// CompiledIOP does not implement anything useful.
type CompiledIOP struct {

	// Columns registers and stores the Columns (ie: messages for the oracle)
	// of the protocol. This includes the committed vectors, the proof messages,
	// the preprocessed commitments that intervene in the protocol.
	Columns *column.Store

	// QueriesParams registers and stores all the parametrizable queries of the
	// specified protocol. By "parametrizable", we mean the queries for which
	// the prover is required to assign runtime parameters. For instance, for
	// a univariate evaluation query : the prover is required to assign an
	// evaluation point X and and at least one evaluation claim.
	QueriesParams ByRoundRegister[ifaces.QueryID, ifaces.Query]

	// QueriesNoParams registers and stores all queries without parameters.
	// Namely, this is storing the queries for which the prover does not need
	// bring extra information at runtime. An example, is [query.GlobalConstraint]
	// which ensures that an arithmetic expression vanishes over its domain. In
	// this case, as long as the arithmetic expression is defined, there is
	// nothing to add.
	QueriesNoParams ByRoundRegister[ifaces.QueryID, ifaces.Query]

	// Coins registers and stores all the verifier's random challenge that are
	// specified in the protocol. A challenge can be either a single field
	// element, an array of field element or an array of bounded field elements.
	// The challenges can be used to specify sub-protocols and are a very
	// widespread cryptographic tool to build them.
	Coins ByRoundRegister[coin.Name, coin.Info]

	// SubProver stores all the specified steps that needs to be performed by
	// the prover as specified in the protocol. These functions are provided to
	// the user and the compilers and their role is to assign the columns and
	// parametrizable's queries parameters during the prover runtime of the
	// protocol.
	SubProvers collection.VecVec[ProverAction]

	// subVerifier stores all the steps that need to be performed by the verifier
	// explicitly. The role of the verifier function's is to implement all the
	// manual checks that the verifier has to perform. This is useful when a check
	// cannot be represented in term of query but, when possible, queries should
	// always be preferred to express a relation that the witness must satisfy.
	SubVerifiers collection.VecVec[VerifierAction]

	// FiatShamirHooksPreSampling is an action that is run during the FS sampling,
	// before sampling the random coins and thus, before every verifier action in
	// the same round. The action is run just after updating the FS state with the
	// items of the previous round. Thus, it can be used to setup the FS state to
	// a desired value. This can be used to add determinism in the coin generation
	// (very useful for debugging, though completely insecure) or it can be used
	// in the context of the distributed prover where the set value is a combination
	// of the provided values and some other external values.
	FiatShamirHooksPreSampling collection.VecVec[VerifierAction]

	// Precomputed stores the assignments of all the Precomputed and VerifierKey
	// polynomials. It is assigned directly when registering a precomputed
	// column.
	Precomputed collection.Mapping[ifaces.ColID, ifaces.ColAssignment]

	// PcsCtxs stores the compilation context of the last used
	// cryptographic compiler. Specifically, it is aimed to store the last
	// Vortex compilation context (see [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler]) that was used. And
	// its purpose is to provide the Vortex context to the self-recursion
	// compilation context; see [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/selfrecursion]. This allows
	// the self-recursion context to learn about the columns to use and the
	// Vortex parameters.
	PcsCtxs any

	// DummyCompiled that can be set internally by the compilation, when we are
	// using the [github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy.Compile] compilation step. This steps
	// commands that the verifier of the protocol should not be compiled into a
	// circuit. This is needed because `dummy.Compile` turns all the columns of
	// the protocol in columns that are visible to the verifier and all the
	// queries into explcit verifier checks. This can incurs a super-massive
	// amount of constraints and the flag
	DummyCompiled bool

	// SelfRecursionCount counts the number of self-recursions induced in the protocol. Used to
	// derive unique names for when the self-recursion is called several time.
	SelfRecursionCount int

	// FiatShamirSetup stores an initial value to use to bootstrap the Fiat-Shamir
	// transcript. This value is obtained by hashing diverse meta-data of the
	// describing the wizard: a version number, the description of the field,
	// a description of all the columns and all the queries etc...
	//
	// For efficiency reasons, the FiatShamirSetup is derived using SHA2.
	FiatShamirSetup field.Element

	// FunctionalPublic inputs lists the queries representing a public inputs
	// and their identifiers
	PublicInputs []PublicInput

	// ExtraData is a free field in which compilers can store whatever they want.
	ExtraData map[string]any

	// WithStorePointerChecks is a flag that controls whether or not the
	// CompiledIOP should check that its columns and queries are registered in
	// the store.
	WithStorePointerChecks bool
}

// NumRounds returns the total number of prover interactions with the verifier
// that are registered in the protocol. If the protocol is non-interactive it
// will return "1"; "2" if one batch of random coins is registered, etc...
func (c *CompiledIOP) NumRounds() int {
	// If there are no coins, we should still return 1 (at least)
	return utils.Max(1, c.Coins.NumRounds())
}

// ListCommitments returns a list of all the column that are registered in the
// protocol. The columns are returned in a deterministic order: round-by-round
// then by chronological order of declaration.
//
// @alex: this should be renamed ListColumns
func (c *CompiledIOP) ListCommitments() []ifaces.ColID {
	return c.Columns.AllKeys()
}

// InsertCommit registers a new column (as committed) in the protocol at a given
// round and returns the corresponding [ifaces.Column] object which summarizes
// the metadata of the column. The user should provide a unique identifier `name`
// and specify a size for the column.
func (c *CompiledIOP) InsertCommit(round int, name ifaces.ColID, size int) ifaces.Column {
	return c.InsertColumn(round, name, size, column.Committed)
}

// InsertColumn registers a new column in the protocol at a given
// round and returns the corresponding [ifaces.Column] object which summarizes the
// metadata of the column. Compared to [CompiledIOP.InsertCommit], the user can additionally
// provide a custom Status to the column. See [column.Status] for more details.
// Importantly, if the user wants to register either a verifying key column
// (i.e. an offline-computed column public to the verifier) or a precomputed
// column (i.e. a precomputed column that is not public to the verifier and
// meant to be committed to) then the ad-hoc functions
// [CompiledIOP.RegisterVerifyingKey] and [CompiledIOP.InsertPrecomputed] should
// be preferred instead. Otherwise, this will cause an error since using
// these types of status requires the user to explicitly provide an assignment.
//
// Note that the function panics
//   - if the name is the empty string
//   - if the size of the column is not a power of 2
//   - if a column using the same name has already been registered
func (c *CompiledIOP) InsertColumn(round int, name ifaces.ColID, size int, status column.Status) ifaces.Column {
	// Panic if the size is not a power of 2
	if !utils.IsPowerOfTwo(size) {
		utils.Panic("Registering column %v with a non power of two size = %v", name, size)
	}
	// @alex: this has actually caught a few typos. When wrongly setting an
	// incorrect but very large size here, it will generate a disproportionate
	// wizard
	if size > 1<<40 {
		utils.Panic("column %v has size %v", name, size)
	}

	if len(name) == 0 {
		panic("Column with an empty name")
	}

	// This performs all the checks
	return c.Columns.AddToRound(round, name, size, status)
}

/*
Registers a new coin at a given rounds. Returns a [coin.Info] object.

* For normal coins, pass

	_ = c.InsertCoin(<round of sampling>, <stringID of the coin>, coin.Field)

* For IntegerVec coins, pass

	_ = c.InsertCoin(<round of sampling>, <stringID of the coin>, coin.IntegerVec, <#Size of the vec>, <#Bound on the integers>)
*/
func (c *CompiledIOP) InsertCoin(round int, name coin.Name, type_ coin.Type, size ...int) coin.Info {
	// Short-hand to access the compiled object
	info := coin.NewInfo(name, type_, round, size...)
	c.Coins.AddToRound(round, name, info)
	return info
}

// InsertGlobal registers a global constraint (see [query.GlobalConstraint])
// inside of the protocol. The `noBoundCancel` field is used to specify if the
// constraint should be cancelled at the beginning or at the end when the
// constraint is applied over shifted columns. If the constraint is not cancelled,
// then the column will implictly loop-around exactly as if all the columns were
// circular vectors.
//
// The function will panic if
//   - the constraint involves one or more columns that are not registered
//     in the CompiledIOP
//   - the constraint involves columns that do not have all the same size
//   - the constraint is given an `empty` name
//   - the expression is invalid (but it should not be possible for the user
//     to build such invalid expressions)
//   - a constraint with the same name already exists
//   - the definition round is inconsistent with the expression
func (c *CompiledIOP) InsertGlobal(round int, name ifaces.QueryID, expr *symbolic.Expression, noBoundCancel ...bool) query.GlobalConstraint {

	c.checkExpressionInStore(expr)

	// The constructor of the global constraint is assumed to perform all the
	// well-formation checks of the constraint.
	cs := query.NewGlobalConstraint(name, expr, noBoundCancel...)
	metadatas := cs.BoardListVariableMetadata()

	// Test the existence of all variable in the instance
	for _, metadataInterface := range metadatas {
		switch metadata := metadataInterface.(type) {
		case ifaces.Column:
			// The handle mecanism prevents this.
		case coin.Info:
			c.Coins.MustExists(metadata.Name)
		case variables.X, variables.PeriodicSample, ifaces.Accessor:
			// Pass
		default:
			utils.Panic("Not a variable type %T in query %v", metadataInterface, cs.ID)
		}
	}

	// Finally registers the query
	c.QueriesNoParams.AddToRound(round, name, cs)

	return cs
}

// InsertLocal registers a global constraint (see [query.LocalConstraint])
// inside of the protocol. The provided name is used as unique identifier for
// the constraint and allows the caller to provide context so that it is easier
// to understand where the column comes from later on.
//
// The function will panic if
//   - the constraint involves one or more columns (or any item) that is not
//     registered in the receiver CompiledIOP
//   - the constraint involves columns that do not have all the same size
//   - the constraint is given an `empty` name
//   - the expression is invalid (but it should not be possible for the user
//     to build such invalid expressions)
//   - a constraint with the same name already exists
//   - the definition round is inconsistent with the expression
func (c *CompiledIOP) InsertLocal(round int, name ifaces.QueryID, cs_ *symbolic.Expression) query.LocalConstraint {

	c.checkExpressionInStore(cs_)

	cs := query.NewLocalConstraint(name, cs_)
	metadatas := cs.BoardListVariableMetadata()

	// Test the existence of all variable in the instance
	for _, metadataInterface := range metadatas {
		switch metadata := metadataInterface.(type) {
		case ifaces.Column:
			// Existence is guaranteed already
		case coin.Info:
			c.Coins.MustExists(metadata.Name)
		case variables.X, variables.PeriodicSample, ifaces.Accessor:
			// Pass
		default:
			utils.Panic("Not a variable type %T in query %v", metadataInterface, cs.ID)
		}
	}

	// Finally registers the query
	c.QueriesNoParams.AddToRound(round, name, cs)

	return cs
}

// InsertPermutation registers a new permutation [query.Permutation] constraint
// in the CompiledIOP. The caller can provide a name to uniquely identify the
// registered constraint and provide some context regarding its role in the
// currently specified protocol.
//
// The function panics if
// - any of the columns in both `a` and `b` do not have the same size
// - any column in `a` or `b“ is a not registered columns
// - a constraint with the same name already exists in the CompiledIOP
func (c *CompiledIOP) InsertPermutation(round int, name ifaces.QueryID, a, b []ifaces.Column) query.Permutation {

	c.checkAnyInStore(a)
	c.checkAnyInStore(b)

	query_ := query.NewPermutation(name, [][]ifaces.Column{a}, [][]ifaces.Column{b})
	c.QueriesNoParams.AddToRound(round, name, query_)
	return query_
}

// InsertFragmentedPermutation is as [CompiledIOP.InsertPermutation] but for
// fragmented tables. Meanining that permutation operates over the union of
// the rows of multiple tables.
func (c *CompiledIOP) InsertFragmentedPermutation(round int, name ifaces.QueryID, a, b [][]ifaces.Column) query.Permutation {
	query_ := query.NewPermutation(name, a, b)
	c.QueriesNoParams.AddToRound(round, name, query_)
	return query_
}

// InsertFixedPermutation registers a new [query.FixedPermutation] constraint
// in the CompiledIOP. The caller can provide a name to uniquely identify the
// registered constraint and provide some context regarding its role in the
// currently specified protocol.
//
// The function panics if
// - any of the columns in both `a` and `b` do not have the same size
// - any column in `a` or `b“ is a not registered columns
// - a constraint with the same name already exists in the CompiledIOP
func (c *CompiledIOP) InsertFixedPermutation(round int, name ifaces.QueryID, p []ifaces.ColAssignment, a, b []ifaces.Column) query.FixedPermutation {

	c.checkAnyInStore(a)
	c.checkAnyInStore(b)

	query_ := query.NewFixedPermutation(name, p, a, b)
	c.QueriesNoParams.AddToRound(round, name, query_)
	return query_
}

// InsertInclusion creates an inclusion query [query.Inclusion]. Here, `included`
// and `including` are viewed as arrays and the query asserts that `included`
// contains only rows that are contained within `includings`, regardless of the
// multiplicities. The caller must provide a non-empty uniquely-identifying
// name to the column. The name should provide some context to help recognizing
// where the column comes from.
//
// The function will panic if:
// - the columns in `including` do not all have the same size
// - the columns in `included` do not all have the same size
// - a constraint with the same name already exists in the CompiledIOP
func (c *CompiledIOP) InsertInclusion(round int, name ifaces.QueryID, including, included []ifaces.Column) {

	c.checkAnyInStore(including)
	c.checkAnyInStore(included)

	query := query.NewInclusion(name, included, [][]ifaces.Column{including}, nil, nil)
	c.QueriesNoParams.AddToRound(round, name, query)
}

/*
Creates an inclusion query. Both the including and the included tables are filtered
the filters should be columns containing only field elements for 0 and 1
*/
func (c *CompiledIOP) InsertInclusionDoubleConditional(round int, name ifaces.QueryID, including, included []ifaces.Column, includingFilter, includedFilter ifaces.Column) {

	c.checkAnyInStore(including)
	c.checkAnyInStore(included)
	c.checkColumnInStore(includingFilter)
	c.checkColumnInStore(includedFilter)

	query := query.NewInclusion(name, included, [][]ifaces.Column{including}, includedFilter, []ifaces.Column{includingFilter})
	c.QueriesNoParams.AddToRound(round, name, query)
}

/*
Creates an inclusion query. Only the including table is filtered
the filters should be columns containing only field elements for 0 and 1
*/
func (c *CompiledIOP) InsertInclusionConditionalOnIncluding(round int, name ifaces.QueryID, including, included []ifaces.Column, includingFilter ifaces.Column) {

	c.checkAnyInStore(including)
	c.checkAnyInStore(included)
	c.checkColumnInStore(includingFilter)

	query := query.NewInclusion(name, included, [][]ifaces.Column{including}, nil, []ifaces.Column{includingFilter})
	c.QueriesNoParams.AddToRound(round, name, query)
}

/*
Creates an inclusion query. Only the included table is filtered
the filters should be columns containing only field elements for 0 and 1
*/
func (c *CompiledIOP) InsertInclusionConditionalOnIncluded(round int, name ifaces.QueryID, including, included []ifaces.Column, includedFilter ifaces.Column) {

	c.checkAnyInStore(including)
	c.checkAnyInStore(included)
	c.checkColumnInStore(includedFilter)

	query := query.NewInclusion(name, included, [][]ifaces.Column{including}, includedFilter, nil)
	c.QueriesNoParams.AddToRound(round, name, query)
}

// GenericFragmentedConditionalInclusion constructs a generic inclusion query
// where the table can possibly be fragmented in several sub-tables. The user
// set `includedFilter` and/or `includingFilter` to be nil if he does not wish
// to use a filter. For the non-fragmented case, the user can set including to
// have length 1 (on the left-side of the double slice).
//
// In all cases, the provided parameters must be consistent in their length to
// represent a well-formed inclusion query or the function panics.
func (c *CompiledIOP) GenericFragmentedConditionalInclusion(
	round int,
	name ifaces.QueryID,
	including [][]ifaces.Column,
	included []ifaces.Column,
	includingFilter []ifaces.Column,
	includedFilter ifaces.Column,
) {

	c.checkAnyInStore(including)
	c.checkAnyInStore(included)
	c.checkAnyInStore(includingFilter)
	c.checkColumnInStore(includedFilter)

	query := query.NewInclusion(name, included, including, includedFilter, includingFilter)
	c.QueriesNoParams.AddToRound(round, name, query)
}

// InsertPrecomputed registers a new precomputed column that is statically
// assigned offline and which is not visible by the verifier. The created
// column bears the [column.Precomputed] status which tags that the column is
// meant to be committed to by the prover and its commitment is meant to be a
// part of the verifying key.
//
// The caller must provide a uniquely identifying string name which can be used
// to provide context about the purpose of the column. The caller should also
// provide an explicit assignment to the column.
func (c *CompiledIOP) InsertPrecomputed(name ifaces.ColID, v smartvectors.SmartVector) (msg ifaces.Column) {

	// Common : No zero length
	if v.Len() == 0 {
		utils.Panic("when registering %v, VecType with length zero", name)
	}

	// Circuit-breaker : if the precomputed poly had already been inserted we
	// can simply return it.
	//
	// @alex: this is really inconsistent with how the rest of the API work. It
	// should panic. The risk here, is that if we provide two columns that do
	// not have the same content but the same name, then we will end up with
	// a very messed up bug that is hard to track.
	if c.Columns.Exists(name) {
		return c.Columns.GetHandle(name)
	}

	c.Precomputed.InsertNew(name, v)
	return c.Columns.AddToRound(0, name, v.Len(), column.Precomputed)
}

// InsertProof registers a proof message by specifying its size and providing
// it a uniquely identifying name. Proof messages are columns bearing the
// [column.Proof] status. They corresponds to columns that are computed by the
// prover online and that are meant to be directly sent to the verifier at the
// end of the current prover's round.
//
// The name must be non-empty and unique and the size must be a power of 2.
func (c *CompiledIOP) InsertProof(round int, name ifaces.ColID, size int) (msg ifaces.Column) {

	// Common : No zero length
	if size == 0 {
		utils.Panic("when registering %v, VecType with length zero", name)
	}

	return c.Columns.AddToRound(round, name, size, column.Proof)
}

// InsertRange registers [query.Range] in the CompiledIOP. Namely, it ensures
// that all the values taken by `h` are within the range [[0; max]]. The caller
// must provide a non-empty uniquely-identifying name to the column. The name
// should provide some context to help recognizing where the column comes from.
//
// The function panics if:
// - the column `h` does not exists
// - the range is not a power of 2
// - the name is the empty string
// - a query with the same name has already been registered in the Wizard.
func (c *CompiledIOP) InsertRange(round int, name ifaces.QueryID, h ifaces.Column, max int) {

	c.checkColumnInStore(h)

	// @alex: this has actually caught a few typos. When wrongly setting an
	// incorrect but very large value here, the query will tend to always pass
	// and thus the tests will tend to miss it.
	if max > 1<<27 {
		utils.Panic("the range check query %v has an overly large boundary (max=%v)", name, max)
	}

	// sanity-check the bound should be larger than 0
	if max == 0 {
		panic("max is zero : perhaps an overflow")
	}

	/*
		In case the range is applied over a composite handle.
		We apply the range over each natural component of the handle.
	*/
	query := query.NewRange(name, h, max)
	c.QueriesNoParams.AddToRound(round, name, query)
}

// InsertInnerProduct registers a (batch) inner-product query
// ([query.InnerProduct]) between a common vector `a` and multiple vectors `bs`,
// meaning it generates an evaluation query for the inner-products <a, bs[i]>
// all at once. The caller must provide a non-empty uniquely-identifying name to
// the column. The name should provide some context to help recognizing where the
// column comes from.
//
// The function panics if:
// - the name is the empty string
// - a query with the same name has already been registered in the Wizard
// - the provided columns `a` and `bs` do not all have the same size
func (c *CompiledIOP) InsertInnerProduct(round int, name ifaces.QueryID, a ifaces.Column, bs []ifaces.Column) query.InnerProduct {

	c.checkColumnInStore(a)
	c.checkAnyInStore(bs)

	// Also ensures that the query round does not predates the columns rounds
	maxComRound := a.Round()
	for _, b := range bs {
		maxComRound = utils.Max(maxComRound, b.Round())
	}

	if maxComRound > round {
		utils.Panic("The query is declared for round %v, but at least one column is declared for round %v", round, maxComRound)
	}

	query := query.NewInnerProduct(name, a, bs...)
	c.QueriesParams.AddToRound(round, name, query)
	return query
}

// Get an Inner-product query
//
// Deprecated: the user should directly grab it from the `Data` section.
func (run *CompiledIOP) GetInnerProduct(name ifaces.QueryID) query.InnerProduct {
	return run.QueriesParams.Data(name).(query.InnerProduct)
}

// InsertUnivariate declares a new univariate evaluation query [query.UnivariateEval]
// in the current CompiledIOP object. A univariate evaluation query is used to
// get an oracle-evaluation of a set of columns (seen as a polynomial in Lagrange
// basis) on a common evaluation point. The point may be assigned during the
// prover runtime and the evaluation are also assigned by the prover
//
// The function panics if:
// - the name is the empty string
// - a query with the same name has already been registered in the Wizard
func (c *CompiledIOP) InsertUnivariate(round int, name ifaces.QueryID, pols []ifaces.Column) query.UnivariateEval {

	c.checkAnyInStore(pols)

	q := query.NewUnivariateEval(name, pols...)
	// Finally registers the query
	c.QueriesParams.AddToRound(round, name, q)
	return q
}

// InsertLocalOpening registers a new local opening query [query.LocalOpening]
// in the current CompiledIOP. A local opening query requires the prover of the
// protocol to "open" the first position of the vector.
func (c *CompiledIOP) InsertLocalOpening(round int, name ifaces.QueryID, pol ifaces.Column) query.LocalOpening {

	c.checkColumnInStore(pol)

	q := query.NewLocalOpening(name, pol)
	// Finally registers the query
	c.QueriesParams.AddToRound(round, name, q)
	return q
}

// InsertLogDerivativeSum registers a new LogDerivativeSum query [query.LogDerivativeSum].
// It generates a single global summation for many Sigma Columns from Lookup compilation.
// The sigma columns are categorized by [round,size].
func (c *CompiledIOP) InsertLogDerivativeSum(lastRound int, id ifaces.QueryID, in query.LogDerivativeSumInput) query.LogDerivativeSum {

	c.checkAnyInStore(in)

	q := query.NewLogDerivativeSum(lastRound, in, id)
	// Finally registers the query
	c.QueriesParams.AddToRound(lastRound, id, q)
	return q
}

// InsertMiMC declares a MiMC constraints query; a constraint that all the
// entries of new are obtained by running the compression function of MiMC over
// the entries of block and old, row-by-row.
//
// The function returns the registered [query.MiMC] object and will panic if
//   - the columns do not share the same size
//   - the declaration round is anterior to the declaration round of the
//     provided input columns.
//
// The caller may provide a (potentially nil) column as a selector. The selector
// disables the query on rows where the selector is 0.
func (c *CompiledIOP) InsertMiMC(round int, id ifaces.QueryID, block, old, new ifaces.Column, selector ifaces.Column) query.MiMC {

	c.checkColumnInStore(block)
	c.checkColumnInStore(old)
	c.checkColumnInStore(new)

	if selector != nil {
		c.checkColumnInStore(selector)
	}

	q := query.NewMiMC(id, block, old, new, selector)
	c.QueriesNoParams.AddToRound(round, id, q)
	return q
}

// RegistersVerifyingKey registers a column as part of the verifying key of the
// protocol; meaning a column whose assignment is static and which is visible
// to the verifier.
func (c *CompiledIOP) RegisterVerifyingKey(name ifaces.ColID, witness ifaces.ColAssignment) ifaces.Column {
	size := witness.Len()
	if size == 0 {
		utils.Panic("when registering %v, VecType with length zero", name)
	}
	c.Precomputed.InsertNew(name, witness)
	return c.InsertColumn(0, name, size, column.VerifyingKey)
}

// RegisterProverAction registers an action to be accomplished by the prover
// of the protocol at a given round.
func (c *CompiledIOP) RegisterProverAction(round int, action ProverAction) {
	// This is purely to not break the current provers in the middle of the
	// switch.
	c.SubProvers.AppendToInner(round, action)
}

// RegisterVerifierAction registers an action to be accomplished by the verifier
// of the protocol at a given round
func (c *CompiledIOP) RegisterVerifierAction(round int, action VerifierAction) {
	// This is purely to not break the current provers in the middle of the
	// switch.
	c.SubVerifiers.AppendToInner(round, action)
}

// Register a GrandProduct query
func (c *CompiledIOP) InsertGrandProduct(round int, id ifaces.QueryID, in map[int]*query.GrandProductInput) query.GrandProduct {

	if in == nil {
		panic("passed a nil set of inputs")
	}

	q := query.NewGrandProduct(round, in, id)
	// Finally registers the query
	c.QueriesParams.AddToRound(round, q.Name(), q)
	return q
}

/*
A projection query between sets (columnsA,filterA) and (columnsB,filterB) asserts
whether the columnsA filtered by filterA is the same as columnsB filtered by
filterB, preserving the order.

Example:

FilterA = (1,0,0,1,1), ColumnA := (aO,a1,a2,a3,a4)

FiletrB := (0,0,1,0,0,0,0,0,1,1), ColumnB :=(b0,b1,b2,b3,b4,b5,b6,b7,b8,b9)

Thus we have,

ColumnA filtered by FilterA = (a0,a3,a4)

ColumnB filtered by FilterB  = (b2,b8,b9)

The projection query checks if a0 = b2, a3 = b8, a4 = b9

Note that the query imposes that:
  - the number of 1 in the filters are equal
  - the order of filtered elements is preserved

The "in" argument can be either a [query.ProjectionInput] or a
[query.ProjectionMultiAryInput].
*/
func (c *CompiledIOP) InsertProjection(id ifaces.QueryID, in any) query.Projection {

	c.checkAnyInStore(in)

	var q query.Projection

	switch in := in.(type) {

	case query.ProjectionInput:
		round := max(
			column.MaxRound(in.ColumnA...),
			column.MaxRound(in.ColumnB...),
			in.FilterA.Round(),
			in.FilterB.Round())
		q = query.NewProjection(round, id, in)

	case query.ProjectionMultiAryInput:
		round := max(
			column.MaxRound(slices.Concat(in.ColumnsA...)...),
			column.MaxRound(slices.Concat(in.ColumnsB...)...),
			column.MaxRound(in.FiltersA...),
			column.MaxRound(in.FiltersB...))
		q = query.NewProjectionMultiAry(round, id, in)

	default:
		panic("invalid projection input")
	}

	c.QueriesNoParams.AddToRound(q.Round, q.Name(), q)

	return q
}

// AddPublicInput inserts a public-input in the compiled-IOP. The function
// panics if the public-input already exists.
func (c *CompiledIOP) InsertPublicInput(name string, acc ifaces.Accessor) PublicInput {

	c.checkAnyInStore(acc)

	res := PublicInput{
		Name: name,
		Acc:  acc,
	}

	for i := range c.PublicInputs {
		if c.PublicInputs[i].Name == name {
			utils.Panic("public input %v already exists", name)
		}
	}

	c.PublicInputs = append(c.PublicInputs, res)
	return res
}

// GetPublicInput attempts to find a public input with the provided name and
// panics if it fails to do so.
func (c *CompiledIOP) GetPublicInput(name string) PublicInput {
	for _, pi := range c.PublicInputs {
		if pi.Name == name {
			return pi
		}
	}

	pubInputNames := []string{}
	for i := range c.PublicInputs {
		pubInputNames = append(pubInputNames, c.PublicInputs[i].Name)
	}

	utils.Panic("could not find public input %v, the list of the public inputs is: %v", name, pubInputNames)
	return PublicInput{} // unreachable
}

// GetPublicInputAccessor attempts to find a public input with the provided name
// and panics if it fails to do so. The method returns the accessor in case of
// success.
func (c *CompiledIOP) GetPublicInputAccessor(name string) ifaces.Accessor {
	return c.GetPublicInput(name).Acc
}

// InsertPlonkInWizard inserts a [query.PlonkInWizard] in the current compilation
// context. The function panics if the query is improper:
//   - the circuit has secret variables
//   - the nb of public inputs of the circuit is larger than the size of Data and Selector
//   - data and selector do not have the same size
//   - the number of public inputs is a power of two (for technical reasons)
func (c *CompiledIOP) InsertPlonkInWizard(q *query.PlonkInWizard) {

	c.checkAnyInStore(q)

	var (
		round           = q.GetRound()
		nbPub, nbSecret = gnarkutil.CountVariables(q.Circuit)
	)

	if q.Data.Size() != q.Selector.Size() {
		utils.Panic("data and selector must have the same size, data-size=%v selector-size=%v", q.Data.Size(), q.Selector.Size())
	}

	if nbPub > q.Data.Size() {
		utils.Panic("the number of public inputs of the circuit is larger than the size of Data and Selector, nbPub=%v data-size=%v selector-size=%v, name=%v", nbPub, q.Data.Size(), q.Selector.Size(), q.ID)
	}

	if nbSecret > 0 {
		utils.Panic("the circuit has secret variables, found %v", nbSecret)
	}

	c.QueriesNoParams.AddToRound(round, q.ID, q)
}

// InsertHornerQuery inserts a [query.Horner] in the current compilation
// context.
func (c *CompiledIOP) InsertHornerQuery(round int, id ifaces.QueryID, parts []query.HornerPart) query.Horner {

	c.checkAnyInStore(parts)

	q := query.NewHorner(round, id, parts)
	// Finally registers the query
	c.QueriesParams.AddToRound(round, q.Name(), &q)
	return q
}

func (c *CompiledIOP) GetSubVerifiers() collection.VecVec[VerifierAction] {
	return c.SubVerifiers
}
