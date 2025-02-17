package wizard

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

// ProverStep represents an operation to be performed by the prover of a
// wizard protocol. It can be provided by the user or by an internal compiled
// to the protocol specification [CompiledIOP] by appending it to the field
// [CompiledIOP.SubProvers].
//
//	CompiledIOP.SubProvers.AppendToInner(round, proverStep)
//
// The ProverStep function may interact with the prover runtime to resolve
// the values of an already assigned item: ([ifaces.Colssignment], coin,
// [ifaces.QueryParams], ...).
//
// The ProverStep function that we pass as the `highLevelProver` argument of
// [Prove] function has the particularity that it is allowed to span
// over multiple interaction-rounds between the prover and the verifier. This
// is a behavior that we intend to deprecate and it should not be used by the
// prover as this tends to create convolutions in the runtime of the prover.
type ProverStep func(assi *ProverRuntime)

// ProverRuntime collects the assignment of all the items with which the prover
// interacts by the prover of the protocol. This includes the prover's
// messages, items that are computed solely by the prover, the witness but also
// the random coins that are sampled by the verifier. The object is implicitly
// constructed by the [Prove] function and it should not be explicitly
// constructed by the user.
//
// Instead, the user should interact with the prover runtime within a
// [ProverStep] function that he provides to the CompiledIOP that he is
// building. Example:
//
//	// Function that the user provide to specify his protocol
//	func myDefineFunction(builder wizard.Builder) {
//
//		// Registers a column "A" as a column to commit to
//		a := build.RegisterCommit("A", 16)
//
//		// Potentially add constraints over the column
//		...
//	}
//
//	// The above define function specifies a protocol involving a column
//	// named "A". If we want to concretely run our protocol, we also need
//	// to provide a way to assign concrete values to the witness of the
//	// protocol.
//	func myProverFunction(run wizard.ProverRuntime) {
//		a := smartvector.ForTest(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16)
//		run.AssignColumn("A", a)
//	}
//
// ProverRuntime also bears the logic to track the current round of interaction
// between the prover and the verifier.
type ProverRuntime struct {

	// Spec is the underlying [CompiledIOP] of the underlying protocol the prover
	// is running.
	Spec *CompiledIOP

	// Columns stores all the column's ([ifaces.Column]) witnesses assigned so
	// far by the ProverRuntime. Columns that are assigned using
	// [ProverRuntime.AssignColumn] method are stored there. For most use-cases,
	// it is preferable to use[ifaces.Columns.GetColAssignment] instead of
	// fetching the assignmentdirectly from the ProverRuntime. The reason is
	// that, the column the caller is trying to fetch may be a "derivative
	// column" or another type of special column whose assignment is not directly
	// available within the prover's runtime.
	//
	// Please consider that this field could become a private field.
	Columns collection.Mapping[ifaces.ColID, ifaces.ColAssignment]

	// QueriesParams accumulates all the query parameters of the queries assigned so far. See
	// [ifaces.QueryParams]. The query parameters that are stored there
	// corresponds to the queries stored in [ProverRuntime.Spec.QueriesParams]
	QueriesParams collection.Mapping[ifaces.QueryID, ifaces.QueryParams]

	// Coins stores all the values of all random Coins that are generated internally
	// as the ProverRuntime unfolds the prover steps round after rounds.
	//
	// The user should not directly access this field and fall back to using the
	// dedicated methods [ProverRuntime.GetRandomCoinField] or
	// [ProverRuntime.GetRandomCoinIntegerVec].
	Coins collection.Mapping[coin.Name, interface{}]

	// State serves as an "any-purpose" data-storage for stateful proving. It allows
	// ProverSteps to persist data that can be accessed in later prover steps
	// without having to store it in a column. For convenience, the user should
	// take care of deleting the entry to free memory when he knows that the
	// field will not be accessed again while proving.
	//
	// The State is used internally by the [github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex] and the
	// [github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion] compilers as a communication channel.
	State collection.Mapping[string, interface{}]

	// currRound indicates the current round the prover is processing and it is incremented
	// every time that the [ProverRuntime.goNextRound] method is called by
	// the [Prove] function.
	currRound int

	// FS stores the Fiat-Shamir State, you probably don't want to use it
	// directly unless you know what you are doing. Just know that if you use
	// it to update the FS hash, this can potentially result in the prover and
	// the verifer end up having different state or the same message being
	// included a second time. Use it externally at your own risks.
	FS *fiatshamir.State

	// lock is global lock so that the assignment maps are thread safes
	lock *sync.Mutex

	// FiatShamirHistory tracks the fiat-shamir state at the beginning of every
	// round. The first entry is the initial state, the final entry is the final
	// state.
	FiatShamirHistory [][2][]field.Element
}

// Prove is the top-level function that runs the Prover on the user's side. It
// is responsible for instantiating a fresh and new ProverRuntime and running
// the user's and compiler's [ProverStep] in order and calling the Fiat-Shamir
// state to generate the randomness between every rounds.
//
// The caller can specify a `highLevelProver` function that implements the
// allocation of the columns and parameters defined in the [Compile]
// via the `define` parameter of the [Compile] function used to construct
// the provided [CompiledIOP] object `c`. In this case, and only in this case,
// the `highLevelProver` function is allowed to span over multiple interaction
// rounds between the prover and the verifier of the protocol. When this
// happens, the underlying [ProverRuntime] object is able to automatically
// follow and detect when the `highLevelProver` function is entering in a new
// round of the protocol.
//
// However, we plan to deprecate this behavior and plan to require the user to
// concretely break down the high-level prover round-by-round as this
// auto-detection adds little value and adds a lot of convolution especially
// when the specified protocol is complicated and involves multiple multi-rounds
// sub-protocols that runs independently.
func Prove(c *CompiledIOP, highLevelprover ProverStep) Proof {
	runtime := c.createProver()
	/*
		Run the user provided assignment function. We can't expect it
		to run all the rounds, because the compilation could have added
		extra-rounds.
	*/
	highLevelprover(&runtime)

	/*
		Then, run the compiled prover steps
	*/
	runtime.runProverSteps()
	for runtime.currRound+1 < runtime.NumRounds() {
		runtime.goNextRound()
		runtime.runProverSteps()
	}

	/*
		Pass all the prover message columns as part of the proof
	*/
	messages := collection.NewMapping[ifaces.ColID, ifaces.ColAssignment]()

	for _, name := range runtime.Spec.Columns.AllKeysProof() {
		messageValue := runtime.Columns.MustGet(name)
		messages.InsertNew(name, messageValue)
	}

	// And also the public inputs
	for _, name := range runtime.Spec.Columns.AllKeysPublicInput() {
		messageValue := runtime.Columns.MustGet(name)
		messages.InsertNew(name, messageValue)
	}

	return Proof{
		Messages:      messages,
		QueriesParams: runtime.QueriesParams,
	}
}

// NumRounds returns the total number of rounds in the corresponding WizardIOP.
//
// Deprecated: this method does not bring anything useful as its already easy
// to get this value from the Spec
func (run *ProverRuntime) NumRounds() int {
	/*
		Getting it from the spec is the safest as it is already
		tested. We could fit more assertions here nonetheless.
	*/
	return run.Spec.NumRounds()
}

// createProver is the internal function that is used by the [Prove]
// function to instantiate and fresh and new [ProverRuntime].
func (c *CompiledIOP) createProver() ProverRuntime {

	// Create a new fresh FS state and bootstrap it
	fs := fiatshamir.NewMiMCFiatShamir()
	fs.Update(c.fiatShamirSetup)

	// Instantiates an empty Assignment (but link it to the CompiledIOP)
	runtime := ProverRuntime{
		Spec:              c,
		Columns:           collection.NewMapping[ifaces.ColID, ifaces.ColAssignment](),
		QueriesParams:     collection.NewMapping[ifaces.QueryID, ifaces.QueryParams](),
		Coins:             collection.NewMapping[coin.Name, interface{}](),
		State:             collection.NewMapping[string, interface{}](),
		FS:                fs,
		currRound:         0,
		lock:              &sync.Mutex{},
		FiatShamirHistory: make([][2][]field.Element, c.NumRounds()),
	}

	runtime.FiatShamirHistory[0] = [2][]field.Element{
		fs.State(),
		fs.State(),
	}

	// Pass the precomputed polynomials
	for key, val := range c.Precomputed.InnerMap() {
		runtime.Columns.InsertNew(key, val)
	}

	return runtime
}

// GetColumn implements `ifaces.Runtime`. Returns a column witness, that has been
// previously stored. It is a deep-copy operation. And thus, it guarantees that
// the stored witness cannot be accidentally mutated by the caller as a side
// effect.
//
// Something to note however, is that the function will panic if the
// the provided name does not exists explictly in the [ProverRuntime.Columns]
// database and this will be the case if the attempts to recover a column such
// as a [column.Shifted] or any other type of derivative columns. While theses
// columns are absolutely legal they are not stored explicitly in the runtime
// and they must be accessed through the [ifaces.Column.GetColAssignment]
// method instead which will work for any type of column. The user should use
// the latter as a go-to way to access an assigned column. The reason this
// function is exported is to make it accessible to the other functions of the
// [github.com/consensys/linea-monorepo/prover/protocol/column] package.
//
// Namely, the function will panic if:
//   - `name` relates to a column that does not exists or to a column that is
//     not explictly an assigned column.
//   - `name` relates to a column that does exists but whose assignment is
//     not readily available when the function is called.
func (run ProverRuntime) GetColumn(name ifaces.ColID) ifaces.ColAssignment {

	// global prover's lock before accessing the witnesses
	run.lock.Lock()
	defer run.lock.Unlock()

	/*
		Make sure the column is registered. If the name is the one specified
		does not correcpond to a natural column, this will panic. And this is
		expected behaviour.
	*/
	run.Spec.Columns.MustHaveName(name)
	res := run.Columns.MustGet(name)
	return res
}

// CopyColumnInto implements `column.GetWitness`. Copies the witness into a slice
// Deprecated: this is deadcode
func (run ProverRuntime) CopyColumnInto(name ifaces.ColID, buff *ifaces.ColAssignment) {

	// global prover's lock before accessing the witnesses
	run.lock.Lock()
	defer run.lock.Unlock()

	/*
		Make sure the column is registered. If the name is the one specified
		does not correcpond to a natural column, this will panic. And this is
		expected behaviour.
	*/
	run.Spec.Columns.MustHaveName(name)
	toCopy := run.Columns.MustGet(name)

	if toCopy.Len() != (*buff).Len() {
		utils.Panic("buffer has the wrong length %v, witness has length %v", (*buff).Len(), toCopy.Len())
	}

	smartvectors.Copy(buff, toCopy)
}

// GetColumnAt does the same as [GetColumn] but only returns a single position
// instead of returning the whole vector; i.e. it returns the assignment of
// an explictly assigned column at a requested position.
//
// The same cautiousness as for [ProverRuntime.AssignColumn] applies to this
// function. Namely, this function will only work if the requested column is
// explicitly an assigned column (meaning not a derive column).
func (run ProverRuntime) GetColumnAt(name ifaces.ColID, pos int) field.Element {

	// global prover's lock before accessing the witnesses
	run.lock.Lock()
	defer run.lock.Unlock()

	/*
		Make sure the column is registered. If the name is the one specified
		does not correcpond to a natural column, this will panic. And this is
		expected behaviour.
	*/
	run.Spec.Columns.MustHaveName(name)
	wit := run.Columns.MustGet(name)

	if pos >= wit.Len() || pos < 0 {
		utils.Panic("asked pos %v for vector of size %v", pos, wit)
	}

	return wit.Get(pos)
}

// GetRandomCoinField returns a field element random. The coin should be issued
// at the same round as it was registered. The same coin can't be retrieved more
// than once. The coin should also have been registered as a field element
// before doing this call. Will also trigger the "goNextRound" logic if
// appropriate.
func (run *ProverRuntime) GetRandomCoinField(name coin.Name) field.Element {
	return run.getRandomCoinGeneric(name, coin.Field).(field.Element)
}

// GetRandomCoinIntegerVec returns a pre-sampled integer vec random coin. The
// coin should be issued at the same round as it was registered. The same coin
// can't be retrieved more than once. The coin should also have been registered
// as an integer vec before doing this call. Will also trigger the
// "goNextRound" logic if appropriate.
func (run *ProverRuntime) GetRandomCoinIntegerVec(name coin.Name) []int {
	return run.getRandomCoinGeneric(name, coin.IntegerVec).([]int)
}

// AssignColumn assigns a value to a column specified in the underlying
// CompiledIOP. For an external user, it should be used only on columns
// explicitly created via the [Builder.RegisterCommit] or
// [CompiledIOP.InsertColumn], [CompiledIOP.InsertCommit] or
// [CompiledIOP.InsertProof] or even [CompiledIOP.InsertPublicInput].
//
// The function will panic if
//   - an empty column name is provided
//   - the column is not explictly registered in the CompiledIOP (e.g. if it is
//     a derive column or the underlying type is found in
//     [github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol] for instance).
//   - the assignment does not have the correct size
//   - the column assignment occurs at the wrong round. If this error happens,
//     it is likely that the [ifaces.Column] was created in the wrong round to
//     begin with.
func (run *ProverRuntime) AssignColumn(name ifaces.ColID, witness ifaces.ColAssignment) {

	// global prover's lock before accessing the witnesses. This makes the
	// function thread-safe
	run.lock.Lock()
	defer run.lock.Unlock()

	// Sanity-check : the handle should not be empty
	if len(name) == 0 {
		panic("given an empty name")
	}

	// @alex: this check is redundant with the following check since GetHandle
	// would panic as it won't be able to find the column if it is
	// "VerifierDefined". The advantage of this check is that it provides a
	// more accurate error message if this occurs.
	if run.Spec.Columns.Status(name) == column.VerifierDefined {
		utils.Panic("tried assign a verifier defined column : %v", name)
	}

	// Sanity-check: Make sure, it is done at the right round
	handle := run.Spec.Columns.GetHandle(name)
	ifaces.MustBeInRound(handle, run.currRound)

	if witness.Len() != handle.Size() {
		utils.Panic("Bad length for %v, expected %v got %v\n", handle, handle.Size(), witness.Len())
	}

	// This is supposed to be redundant with the checks we make when
	// registering a column. So, if it fails here it likely should have failed
	// earlier. Thus, the check is there purely for defensive purposes.
	if !utils.IsPowerOfTwo(witness.Len()) {
		utils.Panic("Witness with non-power of two sizes, should have been caught earlier")
	}

	// This reduction is a trade-off between runtime and memory. It
	// costs CPU but can save a significant amount of memory.
	witness = smartvectors.TryReduceSize(witness)

	// Adds it to the assignments
	run.Columns.InsertNew(handle.GetColID(), witness)
}

// getRandomCoinGeneric is an internal utility function that we use when
// resolving the value of a random coin. When called in the context of the
// `highLevelProver` argument function of [Prove], the function
// encompasses the necessary logic for understanding when to move to the next
// round.
//
// We plan on removing this "go-to-next-round" auto-detection logic in the
// future as this convolutes the code for little benefits.
func (run *ProverRuntime) getRandomCoinGeneric(name coin.Name, requestedType coin.Type) interface{} {
	/*
		Early check, ensures the coin has been registered at all
		and that it has the correct type
	*/
	infos := run.Spec.Coins.Data(name)
	if infos.Type != requestedType {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, requestedType)
	}

	// Check if maybe we need to go to the next round
	foundAtRound := run.Spec.Coins.Round(name)

	switch {
	case foundAtRound <= run.currRound:
		/*
			Regular case : we are fetching the value of a past or current round
			and the coin is already available
		*/
	case foundAtRound == run.currRound+1:
		/*
			The user wants a coin for the next round. This signals that the user is
			moving to the next round. At this stage the "compiled steps" of the current
			rounds still have not been run. This needs to be done before we leave the
			current round. After that, we can generate the coins for the new round.
		*/
		run.runProverSteps()
		/*
			Then, we can try to transitionate to the next rounds. We assert all witnesses,
			params, messages etc.. from the past round have been set. Then, we can generate
			the coins the user is requesting.
		*/
		run.goNextRound()

	case foundAtRound > run.currRound+1:
		/*
			Certainly a bug
		*/
		utils.Panic("Requested coin %v (registered at round %v) but we are at round %v", name, foundAtRound, run.currRound)
	}

	/*
		Note, if we are at round 0. We are guaranteed that this will
		panic (and this is expected, but it does not make sense to query)
		a random coin ex-nihilo.
	*/
	return run.Coins.MustGet(name)
}

// Internal function handling the transition to the next round. This function
// is called when the prover is entering into the next rounds. It proceeds by
// first ensuring that all the columns that were defined in the current round
// have been assigned, otherwise `goNextRound` will panic. Then, it passes all
// the columns visible to the verifier into the Fiat-Shamir state of the
// protocol (i.e. all the columns with either the tag [column.Proof] or
// [column.PublicInput] or the [ifaces.QueryParams] object that have been
// assigned during the current round. Then the function proceeds by entering in
// the next round and sampling all the random coins for the new round with the
// Fiat-Shamir state that we just obtained by appending all the columns and
// parameters. This makes all the new coins available in the prover runtime.
func (run *ProverRuntime) goNextRound() {

	initialState := run.FS.State()

	/*
		Make sure all issued random coin have been "consumed" by all the prover
		steps, in the round we are closing. An error occuring here is more likely
		an error in the compiler than an error from the user because it is not
		responsible for setting the coin. Thus, this is more a sanity check.
	*/
	toBeConsumed := run.Spec.Coins.AllKeysAt(run.currRound)
	run.Coins.MustExists(toBeConsumed...)

	/*
		We do not make this check for the columns, the reason is that we delete
		the columns that we do not use anymore.
	*/

	/*
		Then, make sure all the query parameters have been set
		during the rounds we are closing
	*/
	toBeParametrized := run.Spec.QueriesParams.AllKeysAt(run.currRound)
	run.QueriesParams.MustExists(toBeParametrized...)

	if !run.Spec.DummyCompiled {

		/*
			Make sure that all messages have been written and use them
			to update the FS state.  Note that we do not need to update
			FS using the last round of the prover because he is always
			the last one to "talk" in the protocol.
		*/
		msgsToFS := run.Spec.Columns.AllKeysProofsOrIgnoredButKeptInProverTranscript(run.currRound)
		for _, msgName := range msgsToFS {
			instance := run.GetMessage(msgName)
			run.FS.UpdateSV(instance)
		}

		/*
			Make sure that all messages have been written and use them
			to update the FS state.  Note that we do not need to update
			FS using the last round of the prover because he is always
			the last one to "talk" in the protocol.
		*/
		msgsToFS = run.Spec.Columns.AllKeysPublicInputAt(run.currRound)
		for _, msgName := range msgsToFS {
			instance := run.GetMessage(msgName)
			run.FS.UpdateSV(instance)
		}

		/*
			Also include the prover's allegations for all evaluations
		*/
		paramsToFS := run.Spec.QueriesParams.AllKeysAt(run.currRound)
		for _, qName := range paramsToFS {
			if run.Spec.QueriesParams.IsSkippedFromProverTranscript(qName) {
				continue
			}

			// Implicitly, this will panic whenever we start supporting
			// a new type of query params
			params := run.QueriesParams.MustGet(qName)
			params.UpdateFS(run.FS)
		}
	}

	// Increment the number of rounds
	run.currRound++

	/*
		Then assigns the coins for the new round. As the round
		incrementation is made lazily, we expect that there is
		a next round.
	*/
	toCompute := run.Spec.Coins.AllKeysAt(run.currRound)
	for _, coin := range toCompute {
		info := run.Spec.Coins.Data(coin)
		value := info.Sample(run.FS)
		run.Coins.InsertNew(coin, value)
	}

	finalState := run.FS.State()

	run.FiatShamirHistory[run.currRound] = [2][]field.Element{
		initialState,
		finalState,
	}
}

// runProverSteps runs all the [ProverStep] specified in the underlying
// [CompiledIOP] object for the current round.
func (run *ProverRuntime) runProverSteps() {
	// Run all the assigners
	subProverSteps := run.Spec.SubProvers.MustGet(run.currRound)
	for _, step := range subProverSteps {
		step(run)
	}
}

// GetMessage gets a message sent to the verifier
// Deprecated: use [ProverRuntime.GetColumn] instead
func (run *ProverRuntime) GetMessage(name ifaces.ColID) ifaces.ColAssignment {
	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Sanity-check, this panics if the column does not exists
	return run.Columns.MustGet(name)
}

// GetInnerProduct returns an inner-product query from the underlying CompiledIOP.
// Deprecated: directly use CompiledIOP.Spec.GetInnerProduct() instead, which
// does exactly the same thing.
func (run *ProverRuntime) GetInnerProduct(name ifaces.QueryID) query.InnerProduct {
	return run.Spec.GetInnerProduct(name)
}

// GetInnerProductParams returns pre-assigned parameters for the specified
// [query.InnerProduct] query. The caller specifies the query by its name and
// the method returns the query's parameters. As a reminders, the parameters of
// the query means the result of the inner-products.
//
// The function will panic of the parameters are not available or if the
// parameters have the wrong type: not an [query.InnerProductParams].
func (run *ProverRuntime) GetInnerProductParams(name ifaces.QueryID) query.InnerProductParams {
	return run.QueriesParams.MustGet(name).(query.InnerProductParams)
}

// AssignInnerProduct assigns the result of an inner-product query in the
// prover runtime. The function will panic if
//   - the wrong number of `ys` value is provided. It should match the length
//     of `bs` that was provided when registering the query.
//   - no query with the name `name` are found in the [CompiledIOP] object.
//   - parameters for this query have already been assigned
//   - the assignment round is not the correct one
func (run *ProverRuntime) AssignInnerProduct(name ifaces.QueryID, ys ...field.Element) query.InnerProductParams {
	q := run.GetInnerProduct(name)
	if len(q.Bs) != len(ys) {
		utils.Panic("Inner-product query %v has %v bs but assigned for %v", name, len(q.Bs), len(ys))
	}

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	param := query.NewInnerProductParams(ys...)
	run.QueriesParams.InsertNew(name, param)
	return param
}

// AssignUnivariate assigns the evaluation point and the evaluation result
// and claimed values for a univariate evaluation bearing `name` as an ID.
//
// The function will panic if:
//   - the wrong number of `ys` value is provided. It should match the length
//     of `bs` that was provided when registering the query.
//   - no query with the name `name` are found in the [CompiledIOP] object.
//   - parameters for this query have already been assigned
//   - the assignment round is not the correct one
func (run *ProverRuntime) AssignUnivariate(name ifaces.QueryID, x field.Element, ys ...field.Element) {

	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	// Check the length of ys
	q := run.Spec.QueriesParams.Data(name).(query.UnivariateEval)
	if len(q.Pols) != len(ys) {
		utils.Panic("Query expected ys = %v but got %v", len(q.Pols), len(ys))
	}
	// Adds it to the assignments
	params := query.NewUnivariateEvalParams(x, ys...)
	run.QueriesParams.InsertNew(name, params)
}

// GetUnivariateEval get univariate eval metadata. Panic if not found.
// Deprecated: fallback to run.Spec.GetUnivariateEval instead which does exactly
// the same thing.
func (run *ProverRuntime) GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval {
	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()
	return run.Spec.QueriesParams.Data(name).(query.UnivariateEval)
}

// GetUnivariateParams returns the parameters of a univariate evaluation (i.e:
// x, the evaluation point) and y, the alleged polynomial opening. This is
// intended to resolve parameters that have been already assigned in a previous
// step of the prover runtime.
func (run *ProverRuntime) GetUnivariateParams(name ifaces.QueryID) query.UnivariateEvalParams {
	// Global prover's lock for accessing params
	run.lock.Lock()
	defer run.lock.Unlock()
	return run.QueriesParams.MustGet(name).(query.UnivariateEvalParams)
}

// AssignLocalPoint assign evaluation point and claimed values for a local point
// opening. The function will panic if:
//   - the parameters were already assigned
//   - the specified query is not registered
//   - the assignment round is incorrect
func (run *ProverRuntime) AssignLocalPoint(name ifaces.QueryID, y field.Element) {

	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	// Adds it to the assignments
	params := query.NewLocalOpeningParams(y)
	run.QueriesParams.InsertNew(name, params)
}

// GetLocalPointEval gets the metadata of a [query.LocalOpening] query. Panic if not found.
// Deprecated, use `comp.Spec.GetLocalPointEval` instead since it does exactly
// the same thing.
func (run *ProverRuntime) GetLocalPointEval(name ifaces.QueryID) query.LocalOpening {
	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()
	return run.Spec.QueriesParams.Data(name).(query.LocalOpening)
}

// GetLocalPointEvalParams returns the parameters of a univariate evaluation
// (i.e: x, the evaluation point) and y, the alleged polynomial opening.
func (run *ProverRuntime) GetLocalPointEvalParams(name ifaces.QueryID) query.LocalOpeningParams {

	// Global prover's lock for accessing params
	run.lock.Lock()
	defer run.lock.Unlock()

	return run.QueriesParams.MustGet(name).(query.LocalOpeningParams)
}

// GetParams generically extracts the parameters of a query. Will panic if no
// parameters are found
func (run *ProverRuntime) GetParams(name ifaces.QueryID) ifaces.QueryParams {
	return run.QueriesParams.MustGet(name)
}
