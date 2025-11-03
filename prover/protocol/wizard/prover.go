package wizard

import (
	"fmt"
	"path"

	"sync"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/sirupsen/logrus"
)

// ProverRuntimeOption represents options that can be provided to
// methods of the [wizard.ProverRuntime] struct. These are used to
// enable/disable some of the optimization that are done by the prover
// internally.
type ProverRuntimeOption uint64

const (
	// DisableAssignmentSizeReduction is used to disable the
	// the routine that tries to reduce the space taken by a column.
	DisableAssignmentSizeReduction ProverRuntimeOption = 1 << iota
)

// This is a compilation check to ensure that the [wizard.ProverRuntime]
// implements the [wizard.Runtime] interface.
var _ Runtime = &ProverRuntime{}

// MainProverStep represents an operation to be performed by the prover of a
// wizard protocol. It can be provided by the user or by an internal compiled
// to the protocol specification [CompiledIOP] by appending it to the field
// [CompiledIOP.SubProvers].
//
//	CompiledIOP.SubProvers.AppendToInner(round, proverStep)
//
// The MainProverStep function may interact with the prover runtime to resolve
// the values of an already assigned item: ([ifaces.Colssignment], coin,
// [ifaces.QueryParams], ...).
//
// The MainProverStep function that we pass as the `highLevelProver` argument of
// [Prove] function has the particularity that it is allowed to span
// over multiple interaction-rounds between the prover and the verifier. This
// is a behavior that we intend to deprecate and it should not be used by the
// prover as this tends to create convolutions in the runtime of the prover.

type MainProverStep func(assi *ProverRuntime)

// ProverRuntime collects the assignment of all the items with which the prover
// interacts by the prover of the protocol. This includes the prover's
// messages, items that are computed solely by the prover, the witness but also
// the random coins that are sampled by the verifier. The object is implicitly
// constructed by the [Prove] function and it should not be explicitly
// constructed by the user.
//
// Instead, the user should interact with the prover runtime within a
// [MainProverStep] function that he provides to the CompiledIOP that he is
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
	// MainProverSteps to persist data that can be accessed in later prover steps
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

	PerformanceMonitor *config.PerformanceMonitor

	// PerformanceLogs stores performance metrics for each major operation
	PerformanceLogs []*profiling.PerformanceLog

	// High-level prover function
	HighLevelProver MainProverStep
}

// Prove is the top-level function that runs the Prover on the user's side. It
// is responsible for instantiating a fresh and new ProverRuntime and running
// the user's and compiler's [MainProverStep] in order and calling the Fiat-Shamir
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
func Prove(c *CompiledIOP, highLevelprover MainProverStep) Proof {
	run := RunProver(c, highLevelprover)

	// Write the performance logs to the csv file is the performance monitor is active
	if run.PerformanceMonitor.Active {
		csvFilePath := path.Join(run.PerformanceMonitor.ProfileDir, "runtime_performance_logs.csv")
		perfLogs := profiling.PerfLogs(run.PerformanceLogs)
		if err := perfLogs.WritePerformanceLogsToCSV(csvFilePath); err != nil {
			utils.Panic("error writing performance logs to CSV: %v", err.Error())
		}
	}

	return run.ExtractProof()
}

// RunProver initializes a [ProverRuntime], runs the prover and returns the final
// runtime. It does not returns the [Proof] however.
func RunProver(c *CompiledIOP, highLevelprover MainProverStep) *ProverRuntime {
	return RunProverUntilRound(c, highLevelprover, c.NumRounds())
}

// RunProverUntilRound runs the prover until the specified round
// We wrap highLevelProver with a struct that implements the prover action interface
func RunProverUntilRound(c *CompiledIOP, highLevelProver MainProverStep, round int) *ProverRuntime {
	runtime := c.createProver()
	runtime.HighLevelProver = highLevelProver

	// Execute the high-level prover as a ProverAction
	if runtime.HighLevelProver != nil {
		runtime.exec("high-level-prover", proverStepWrapper{step: highLevelProver})
	}

	// Run sub-prover steps for the initial round
	runtime.exec(fmt.Sprintf("prover-steps-round%d", runtime.currRound), runtime.runProverSteps)

	for runtime.currRound+1 < round {
		runtime.exec(fmt.Sprintf("next-after-round-%d", runtime.currRound), runtime.goNextRound)
		runtime.exec(fmt.Sprintf("prover-steps-round-%d", runtime.currRound), runtime.runProverSteps)
	}

	return &runtime
}

// Resume resumes a [ProverRuntime] from a checkpoint till the end (the last
// round) and returns a pointer to self.
func (runtime *ProverRuntime) Resume() *ProverRuntime {
	round := runtime.Spec.NumRounds()
	for runtime.currRound+1 < round {
		runtime.exec(fmt.Sprintf("next-after-round-%d", runtime.currRound), runtime.goNextRound)
		runtime.exec(fmt.Sprintf("prover-steps-round-%d", runtime.currRound), runtime.runProverSteps)
	}
	return runtime
}

// ExtractProof extracts the proof from a [ProverRuntime]. If the runtime has
// been obtained via a [RunProverUntilRound], then it may be the case that
// some columns have not been assigned at all. Those won't be included in the
// returned proof.
func (run *ProverRuntime) ExtractProof() Proof {
	messages := collection.NewMapping[ifaces.ColID, ifaces.ColAssignment]()
	for _, name := range run.Spec.Columns.AllKeysProof() {
		cols := run.Spec.Columns.GetHandle(name)
		if run.currRound < cols.Round() {
			continue
		}
		messageValue := run.Columns.MustGet(name)
		messages.InsertNew(name, messageValue)
	}

	queriesParams := collection.NewMapping[ifaces.QueryID, ifaces.QueryParams]()
	for round := 0; round <= run.currRound; round++ {
		for _, name := range run.Spec.QueriesParams.AllKeysAt(round) {
			queriesParams.InsertNew(name, run.QueriesParams.MustGet(name))
		}
	}

	return Proof{
		Messages:      messages,
		QueriesParams: queriesParams,
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
	fs.Update(c.FiatShamirSetup)

	// Instantiates an empty Assignment (but link it to the CompiledIOP)
	runtime := ProverRuntime{
		Spec:               c,
		Columns:            collection.NewMapping[ifaces.ColID, ifaces.ColAssignment](),
		QueriesParams:      collection.NewMapping[ifaces.QueryID, ifaces.QueryParams](),
		Coins:              collection.NewMapping[coin.Name, interface{}](),
		State:              collection.NewMapping[string, interface{}](),
		FS:                 fs,
		currRound:          0,
		lock:               &sync.Mutex{},
		FiatShamirHistory:  make([][2][]field.Element, c.NumRounds()),
		PerformanceMonitor: profiling.GetMonitorParams(),
	}

	runtime.FiatShamirHistory[0] = [2][]field.Element{
		fs.State(),
		fs.State(),
	}

	// Pass the precomputed polynomials
	for key, val := range c.Precomputed.GetInnerMap() {
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

// HasColumn returns whether the column is assigned. The function panics if the
// provided column name does not exists
func (run ProverRuntime) HasColumn(name ifaces.ColID) bool {

	// global prover's lock before accessing the witnesses
	run.lock.Lock()
	defer run.lock.Unlock()

	/*
		Make sure the column is registered. If the name is the one specified
		does not correcpond to a natural column, this will panic. And this is
		expected behaviour.
	*/
	run.Spec.Columns.MustHaveName(name)
	return run.Columns.Exists(name)
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
	mycoin := run.Spec.Coins.Data(name)

	if mycoin.Type == 0 {
		return run.getRandomCoinGeneric(name, coin.Field).(field.Element)
	}
	return run.getRandomCoinGeneric(name, coin.FieldFromSeed).(field.Element)
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
func (run *ProverRuntime) AssignColumn(name ifaces.ColID, witness ifaces.ColAssignment, opts ...ProverRuntimeOption) {

	var opts_ ProverRuntimeOption
	for i := range opts {
		opts_ |= opts[i]
	}

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
	handle := run.Spec.Columns.GetHandle(name).(column.Natural)
	// if round is empty, we expect it to assign the column at the current round,
	// otherwise it assigns it in the round the column was declared.
	// This is useful when we have for loop over rounds.
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

	// If the column is generated after the first round, there is no need
	// optimizing the assignment.
	if run.currRound > 0 {
		// Adds it to the assignments
		run.Columns.InsertNew(handle.GetColID(), witness)
		return
	}

	start, stop := smartvectors.CoWindowRange(witness)

	var (
		hasRightPaddedRange  = stop < witness.Len()
		hasLeftPaddedRange   = start > 0
		hasRightPaddedPragma = pragmas.IsRightPadded(handle)
		hasLeftPaddedPragma  = pragmas.IsLeftPadded(handle)
		hasFullColumnPragma  = pragmas.IsFullColumn(handle)
	)

	switch {
	case hasLeftPaddedPragma:

		if !hasLeftPaddedRange {
			// logrus.Warnf("Left-padded column with non-left-padded witness: %v, start: %v, stop: %v", name, start, stop)
			// This conversion to regular ensures that the witness won't be
			// stored as a right-padded column. The size reduction might later
			// find a padding opportunity in the right direction. The conversion
			// is ineffective in case the column is a regular column (which
			// might be caught by the condition)
			witness = smartvectors.NewRegular(witness.IntoRegVecSaveAlloc())
		}

		// This reduction is a trade-off between runtime and memory. It costs CPU
		// but can save a significant amount of memory.
		if opts_&DisableAssignmentSizeReduction == 0 {
			witness, _ = smartvectors.TryReduceSizeLeft(witness)
		}

	case hasRightPaddedPragma:

		if !hasRightPaddedRange {
			// logrus.Warnf("Right-padded column with non-right-padded witness: %v, start: %v, stop: %v", name, start, stop)
			// This conversion to regular ensures that the witness won't be
			// stored as a left-padded column. The size reduction might later
			// find a padding opportunity in the right direction. The conversion
			// is ineffective in case the column is a regular column (which
			// might be caught by the condition)
			witness = smartvectors.NewRegular(witness.IntoRegVecSaveAlloc())
		}

		// This reduction is a trade-off between runtime and memory. It costs CPU
		// but can save a significant amount of memory.
		if opts_&DisableAssignmentSizeReduction == 0 {
			witness, _ = smartvectors.TryReduceSizeRight(witness)
		}

	case hasFullColumnPragma:

		if hasLeftPaddedRange || hasRightPaddedRange {
			logrus.Errorf("Full column with non-full witness: %v, start: %v, stop: %v", name, start, stop)
		}

		witness = smartvectors.NewRegular(witness.IntoRegVecSaveAlloc())

	default:

		// This reduction is a trade-off between runtime and memory. It costs CPU
		// but can save a significant amount of memory.
		if opts_&DisableAssignmentSizeReduction == 0 {
			witness, _ = smartvectors.TryReduceSizeRight(witness)
		}
	}

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

	if !run.Spec.DummyCompiled {

		/*
			Make sure that all messages have been written and use them
			to update the FS state.  Note that we do not need to update
			FS using the last round of the prover because he is always
			the last one to "talk" in the protocol.
		*/
		msgsToFS := run.Spec.Columns.AllKeysInProverTranscript(run.currRound)
		for _, msgName := range msgsToFS {

			if run.Spec.Columns.IsExplicitlyExcludedFromProverFS(msgName) {
				continue
			}

			if run.Spec.Precomputed.Exists(msgName) {
				continue
			}

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

	if run.Spec.FiatShamirHooksPreSampling.Len() > run.currRound {
		fsHooks := run.Spec.FiatShamirHooksPreSampling.MustGet(run.currRound)
		for i := range fsHooks {
			// if fsHooks[i].IsSkipped() {
			// 	continue
			// }

			fsHooks[i].Run(run)
		}
	}

	seed := run.FS.State()[0]

	// Then assigns the coins for the new round. As the round
	// incrementation is made lazily, we expect that there is
	// a next round.
	toCompute := run.Spec.Coins.AllKeysAt(run.currRound)

	for _, myCoin := range toCompute {
		if run.Spec.Coins.IsSkippedFromProverTranscript(myCoin) {
			continue
		}

		info := run.Spec.Coins.Data(myCoin)
		value := info.Sample(run.FS, seed)
		run.Coins.InsertNew(myCoin, value)
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
	for idx, step := range subProverSteps {

		// Profile individual prover steps
		namePrefix := fmt.Sprintf("prover-round%d-step%d", run.currRound, idx)
		run.exec(namePrefix, step)
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

// AssignLogDerivSum assign the claimed values for a logDeriveSum
// The function will panic if:
//   - the parameters were already assigned
//   - the specified query is not registered
//   - the assignment round is incorrect
func (run *ProverRuntime) AssignLogDerivSum(name ifaces.QueryID, y field.Element) {

	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	// Adds it to the assignments
	params := query.NewLogDerivSumParams(y)
	run.QueriesParams.InsertNew(name, params)
}

// AssignGrandProduct assign the claimed values for a grand product
// The function will panic if:
//   - the parameters were already assigned
//   - the specified query is not registered
//   - the assignment round is incorrect
func (run *ProverRuntime) AssignGrandProduct(name ifaces.QueryID, y field.Element) {

	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	// Adds it to the assignments
	params := query.NewGrandProductParams(y)
	run.QueriesParams.InsertNew(name, params)
}

// GetLogDeriveSum gets the metadata of a [query.LogDerivativeSum] query. Panic if not found.
func (run *ProverRuntime) GetLogDeriveSum(name ifaces.QueryID) query.LogDerivativeSum {
	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()
	return run.Spec.QueriesParams.Data(name).(query.LogDerivativeSum)
}

// GetLogDerivSumParams returns the parameters of [query.LogDerivativeSum]
func (run *ProverRuntime) GetLogDerivSumParams(name ifaces.QueryID) query.LogDerivSumParams {

	// Global prover's lock for accessing params
	run.lock.Lock()
	defer run.lock.Unlock()

	return run.QueriesParams.MustGet(name).(query.LogDerivSumParams)
}

// GetGrandProductParams returns the parameters of a [query.Honer] query.
func (run *ProverRuntime) GetGrandProductParams(name ifaces.QueryID) query.GrandProductParams {
	return run.QueriesParams.MustGet(name).(query.GrandProductParams)
}

// GetParams generically extracts the parameters of a query. Will panic if no
// parameters are found
func (run *ProverRuntime) GetParams(name ifaces.QueryID) ifaces.QueryParams {
	return run.QueriesParams.MustGet(name)
}

// AssignHornerParams assignes the parameters of a [query.Honer] query.
func (run *ProverRuntime) AssignHornerParams(name ifaces.QueryID, params query.HornerParams) {
	// Global prover locks for accessing the maps
	run.lock.Lock()
	defer run.lock.Unlock()

	// Make sure, it is done at the right round
	run.Spec.QueriesParams.MustBeInRound(run.currRound, name)

	// Adds it to the assignments
	run.QueriesParams.InsertNew(name, params)
}

// GetHornerParams returns the parameters of a [query.Honer] query.
func (run *ProverRuntime) GetHornerParams(name ifaces.QueryID) query.HornerParams {
	// Global prover's lock for accessing params
	run.lock.Lock()
	defer run.lock.Unlock()
	return run.QueriesParams.MustGet(name).(query.HornerParams)
}

// Fs returns the Fiat-Shamir state
func (run *ProverRuntime) Fs() *fiatshamir.State {
	return run.FS
}

// FsHistory returns the Fiat-Shamir state history
func (run *ProverRuntime) FsHistory() [][2][]field.Element {
	return run.FiatShamirHistory
}

// GetPublicInputs return the value of a public-input from its name
func (run *ProverRuntime) GetPublicInput(name string) field.Element {
	allPubs := run.Spec.PublicInputs
	for i := range allPubs {
		if allPubs[i].Name == name {
			return allPubs[i].Acc.GetVal(run)
		}
	}
	utils.Panic("could not find public input nb %v", name)
	return field.Element{}
}

// HasPublicInput returns true if the public input with the provided name exists
func (run *ProverRuntime) HasPublicInput(name string) bool {
	allPubs := run.Spec.PublicInputs
	for i := range allPubs {
		if allPubs[i].Name == name {
			return true
		}
	}
	return false
}

// GetQuery returns a query from its name
func (run *ProverRuntime) GetQuery(name ifaces.QueryID) ifaces.Query {

	if run.Spec.QueriesParams.Exists(name) {
		return run.Spec.QueriesParams.Data(name)
	}

	if run.Spec.QueriesNoParams.Exists(name) {
		return run.Spec.QueriesNoParams.Data(name)
	}

	utils.Panic("could not find query nb %v", name)
	return nil
}

// GetSpec returns the underlying compiled IOP
func (run *ProverRuntime) GetSpec() *CompiledIOP {
	return run.Spec
}

// GetState returns an arbitrary value stored in the runtime
func (run *ProverRuntime) GetState(name string) (any, bool) {
	res, ok := run.State.TryGet(name)
	return res, ok
}

// SetState sets an arbitrary value in the runtime
func (run *ProverRuntime) SetState(name string, value any) {
	run.State.InsertNew(name, value)
}

// InsertCoin is there so that [ProverRuntime] implements the [ifaces.Runtime]
// but the function panicks if called.
func (run *ProverRuntime) InsertCoin(name coin.Name, value any) {
	utils.Panic("InsertCoin is not implemented")
}

// exec: executes the `action` with the performance monitor if active
func (runtime *ProverRuntime) exec(name string, action any) {

	// Define helper excute function
	execute := func() {
		switch a := action.(type) {
		case func():
			a()
		case ProverAction:
			a.Run(runtime)
		default:
			utils.Panic("wizard.exec: unsupported action type: got %T; expected one of: func(), ProverAction", action)
		}
	}

	// If PerformanceMonitor is inactive, just execute the action and return
	if !runtime.PerformanceMonitor.Active {
		execute()
		return
	}

	// Determine if profiling is needed based on action type and profile setting
	shouldProfile := false
	switch runtime.PerformanceMonitor.Profile {
	case "all":
		shouldProfile = true
	case "prover-rounds":
		shouldProfile = actionIsPlainFunc(action)
	case "prover-steps":
		shouldProfile = actionIsProverAction(action)
	}

	if shouldProfile {
		runtime.profileAction(name, execute)
	} else {
		execute()
	}
}

// profileAction profiles the given action and logs the performance metrics.
func (runtime *ProverRuntime) profileAction(name string, action func()) {
	profilingPath := path.Join(runtime.PerformanceMonitor.ProfileDir, name)
	monitor, err := profiling.StartPerformanceMonitor(name, runtime.PerformanceMonitor.SampleDuration, profilingPath)
	if err != nil {
		panic("error setting up performance monitor for " + name)
	}

	action()

	perfLog, err := monitor.Stop()
	if err != nil {
		logrus.Panicf("error:%s encountered while retrieving performance log for:%s", err.Error(), name)
	}

	// perfLog.PrintMetrics()
	runtime.PerformanceLogs = append(runtime.PerformanceLogs, perfLog)
}

// actionIsProverRound checks if the action is a plain function such as nextRound or runProverSteps
func actionIsPlainFunc(action any) bool {
	_, ok := action.(func())
	return ok
}

// actionIsProverAction checks if the action is an individual ProverStep in a specific round or highlevelProver.
func actionIsProverAction(action any) bool {
	_, ok := action.(ProverAction)
	return ok
}
