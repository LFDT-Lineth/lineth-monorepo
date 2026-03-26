package wizard

import (
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

// Proof generically represents a proof obtained from the wizard. This object does not
// implement any logic and only serves as a registry for all the prover messages
// that are assigned by the prover runtime and that are necessary to run
// the verifier. It includes the assignment of all the columns that are visible
// to the verifier; meaning all columns bearing the tag [column.Proof] and the
// query parameters [ifaces.QueryParams] provided by the prover runtime.
//
// The proof can be constructed using the [Prove] function and can be
// used as an input to the [Verify] function. It can also be used to
// assign a [VerifierCircuit] in order to recursively compose
// the proof within a gnark circuit.
//
// The struct does not implement any serialization logic.
type Proof struct {
	// Messages collection of the prover's message sent to the verifier.
	Messages collection.Mapping[ifaces.ColID, ifaces.ColAssignment]

	// QueriesParams stores all the query parameters (i.e) the messages of the
	// oracle to the verifier.
	QueriesParams collection.Mapping[ifaces.QueryID, ifaces.QueryParams]
}

// Runtime is a generic interface extending the [ifaces.Runtime] interface
// with all methods of [wizard.VerifierRuntime]. This is used to allow the
// writing of adapters for the verifier runtime.
type Runtime interface {
	ifaces.Runtime
	GetSpec() *CompiledIOP
	GetPublicInput(name string) fext.GenericFieldElem
	GetGrandProductParams(name ifaces.QueryID) query.GrandProductParams
	GetHornerParams(name ifaces.QueryID) query.HornerParams
	GetLogDerivSumParams(name ifaces.QueryID) query.LogDerivSumParams
	GetLocalPointEvalParams(name ifaces.QueryID) query.LocalOpeningParams
	GetInnerProductParams(name ifaces.QueryID) query.InnerProductParams
	GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval
	GetUnivariateParams(name ifaces.QueryID) query.UnivariateEvalParams
	GetQuery(name ifaces.QueryID) ifaces.Query
	Fs() fiatshamir.FS
	InsertCoin(name coin.Name, value any)
	GetState(name string) (any, bool)
	SetState(name string, value any)
}

// VerifierStep specifies a single step of verifier for a single subprotocol.
// This can be used to specify verifier checks involving user-provided
// columns for relations that cannot be automatically enforced via a
// [ifaces.Query]
type VerifierStep func(a Runtime) error

// VerifierRuntime runtime collects all data that visible or computed by the
// verifier of the wizard protocol. This includes the prover's messages, the
// [column.VerifyingKey] tagged columns.
//
// The struct is not intended to be constructed by the user and is internally
// constructed by the [Verify] function. The user should instead
// restricts its usage of the function within [VerifierStep] functions that are
// provided to either the [CompiledIOP] or the [Verify] function.
type VerifierRuntime struct {

	// Spec points to the static description of the underlying protocol
	Spec *CompiledIOP

	// Collection of the prover's message sent to the verifier.
	Columns collection.Mapping[ifaces.ColID, ifaces.ColAssignment]

	// Coins stores all the random coins issued during the protocol
	Coins collection.Mapping[coin.Name, interface{}]

	// Stores all the query parameters (i.e) the messages of the oracle to the
	// verifier.
	QueriesParams collection.Mapping[ifaces.QueryID, ifaces.QueryParams]

	// KoalaFS stores the Fiat-Shamir State, you probably don't want to use it
	// directly unless you know what you are doing. Just know that if you use
	// it to update the KoalaFS hash, this can potentially result in the prover and
	// the verifer end up having different state or the same message being
	// included a second time. Use it externally at your own risks.
	KoalaFS fiatshamir.FS
	BLSFS   fiatshamir.FS
	IsBLS   bool

	// State stores arbitrary data that can be used by the verifier. This
	// can be used to communicate values between verifier states.
	State map[string]interface{}
}

// Verify verifies a wizard proof. The caller specifies a [CompiledIOP] that
// describes the protocol to run and a proof to verify. The function returns
// `nil` to indicate that the proof passed and an error to indicate the proof
// was invalid.
func Verify(c *CompiledIOP, proof Proof, IsBLS ...bool) error {
	isBLSValue := false
	if IsBLS != nil {
		isBLSValue = IsBLS[0]
	}
	_, err := VerifyWithRuntime(c, proof, isBLSValue)
	return err
}

// VerifyWithRuntime runs the verifier of the protocol and returns the result
// and the runtime of the verifier.
func VerifyWithRuntime(c *CompiledIOP, proof Proof, IsBLS bool) (*VerifierRuntime, error) {
	return verifyWithRuntimeUntilRound(c, proof, c.NumRounds(), IsBLS)
}

// VerifyUntilRound runs the verifier up to a specified round
func VerifyUntilRound(c *CompiledIOP, proof Proof, stopRound int, IsBLS bool) error {
	_, err := verifyWithRuntimeUntilRound(c, proof, stopRound, IsBLS)
	return err
}

// verifyWithRuntimeUntilRound runs the verifier of 'comp' up to (and excluding)
// the provided round "stopRound". By "excluding", we mean that the function
// won't run the round "stopRound". If stopRound is higher than the number of
// rounds in comp, the function runs the whole protocol.
func verifyWithRuntimeUntilRound(comp *CompiledIOP, proof Proof, stopRound int, IsBLS bool) (run *VerifierRuntime, err error) {

	var (
		runtime = comp.createVerifier(proof, IsBLS)
		errs    = []error{}
	)

	stopRound = min(stopRound, comp.NumRounds())

	for round := 0; round < stopRound; round++ {

		runtime.GenerateCoinsFromRound(round)

		verifierSteps := runtime.Spec.SubVerifiers.MustGet(round)
		for _, step := range verifierSteps {
			if err := step.Run(&runtime); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return &runtime, utils.WrapErrsAlphabetically(errs)
	}

	return &runtime, nil
}

// createVerifier is an internal constructor for a new empty [VerifierRuntime] runtime. It
// prepopulates the [VerifierRuntime.Columns] and [VerifierRuntime.QueriesParams]
// with the one that are in the proof. It also populates its verifier steps from
// the `VerifierFuncGen` in the `c`. The user also passes the list of prover
// messages. It is internally called by the [Verify] function.
func (c *CompiledIOP) createVerifier(proof Proof, IsBLS bool) VerifierRuntime {

	/*
		Instantiate an empty assigment for the verifier
	*/
	runtime := VerifierRuntime{
		Spec:          c,
		Coins:         collection.NewMapping[coin.Name, interface{}](),
		Columns:       proof.Messages.Clone(),
		IsBLS:         IsBLS,
		QueriesParams: proof.QueriesParams.Clone(),
		State:         make(map[string]interface{}),
	}

	// Create a new fresh FS state and bootstrap it
	if IsBLS {
		fs := fiatshamir.NewFSBls12377()
		runtime.BLSFS = fs
		runtime.BLSFS.Update(c.FiatShamirSetup[:]...)
	} else {
		fs := fiatshamir.NewFSKoalabear()
		runtime.KoalaFS = fs
		runtime.KoalaFS.Update(c.FiatShamirSetup[:]...)
	}

	/*
		Insert the verifying key into the messages
	*/
	for _, name := range c.Columns.AllVerifyingKey() {
		val := c.Precomputed.MustGet(name)
		runtime.Columns.InsertNew(name, val)
	}

	return runtime
}

// GetPublicInput extracts the value of a public input from the proof.
func (proof Proof) GetPublicInput(comp *CompiledIOP, name string, IsBLS bool) fext.GenericFieldElem {

	publicInputsAccessor := comp.GetPublicInputAccessor(name)

	switch a := publicInputsAccessor.(type) {
	case *accessors.FromConstAccessor:
		if a.IsBase() {
			return fext.GenericFieldElem{Base: a.Base, IsBase: true}
		} else {
			panic("Requested a base element from a public input that is a field extension")
		}
	case *accessors.FromPublicColumn:
		if a.Col.Status() == column.Proof {
			return fext.GenericFieldElem{Base: proof.Messages.MustGet(a.Col.ID).Get(a.Pos), IsBase: true}
		}
	case *accessors.FromLocalOpeningYAccessor:
		return fext.GenericFieldElem{Base: proof.QueriesParams.MustGet(a.Q.ID).(query.LocalOpeningParams).BaseY, IsBase: true}
	}

	// This generically returns the value of a public input by extracting
	// it from the runtime of the verifier. This is inefficient because it
	// needs to run the verifier to extract the value. So this behaviour
	// should be used only for types of [ifaces.Accessor] who need it.
	//
	// These are not directly visible from the proof. Thus we need to
	// run the verifier and extract them from the runtime.
	verifierRuntime, _ := VerifyWithRuntime(comp, proof, IsBLS)
	return verifierRuntime.GetPublicInput(name)
}

// GenerateCoinsFromRound generates all the random coins for the given round.
// It does so by updating the FS with all the prover messages from round-1
// and then generating all the coins for the current round.
func (run *VerifierRuntime) GenerateCoinsFromRound(currRound int) {

	if currRound > 0 && !run.Spec.DummyCompiled {

		/*
			Make sure that all messages have been written and use them
			to update the FS state.  Note that we do not need to update
			FS using the last round of the prover because he is always
			the last one to "talk" in the protocol.
		*/
		msgsToFS := run.Spec.Columns.AllKeysProofAt(currRound - 1)
		for _, msgName := range msgsToFS {

			if run.Spec.Columns.IsExplicitlyExcludedFromProverFS(msgName) {
				continue
			}

			if run.Spec.Precomputed.Exists(msgName) {
				continue
			}

			instance := run.GetColumn(msgName)
			if run.IsBLS {
				run.BLSFS.UpdateSV(instance)
			} else {
				run.KoalaFS.UpdateSV(instance)
			}
		}

		/*
			Also include the prover's allegations for all evaluations
		*/
		queries := run.Spec.QueriesParams.AllKeysAt(currRound - 1)
		for _, qName := range queries {
			if run.Spec.QueriesParams.IsSkippedFromVerifierTranscript(qName) {
				continue
			}

			params := run.QueriesParams.MustGet(qName)
			if run.IsBLS {
				params.UpdateFS(run.BLSFS)
			} else {
				params.UpdateFS(run.KoalaFS)
			}
		}
	}

	if run.Spec.FiatShamirHooksPreSampling.Len() > currRound {
		fsHooks := run.Spec.FiatShamirHooksPreSampling.MustGet(currRound)
		for i := range fsHooks {
			fsHooks[i].Run(run)
		}
	}

	var seed field.Octuplet
	if run.IsBLS {
		seed = run.BLSFS.State()
	} else {
		seed = run.KoalaFS.State()
	}

	/*
		Then assigns the coins for the new round. As the round incrementation
		is made lazily, we expect that there is a next round.
	*/
	toCompute := run.Spec.Coins.AllKeysAt(currRound)
	for _, myCoin := range toCompute {
		if run.Spec.Coins.IsSkippedFromVerifierTranscript(myCoin) {
			continue
		}

		info := run.Spec.Coins.Data(myCoin)
		var value interface{}
		if run.IsBLS {
			value = info.Sample(run.BLSFS, seed)
		} else {
			value = info.Sample(run.KoalaFS, seed)
		}
		run.Coins.InsertNew(myCoin, value)
	}
}

// GetRandomCoinField returns a field random element. The coin should be issued
// at the same round as it was registered. The same coin can't be retrieved more
// than once. The coin should also have been registered as a field element
// before doing this call. Will also trigger the "goNextRound" logic if
// appropriate.
func (run *VerifierRuntime) GetRandomCoinFieldExt(name coin.Name) fext.Element {
	infos := run.Spec.Coins.Data(name)
	if infos.Type != coin.FieldExt && infos.Type != coin.FieldFromSeed {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, coin.FieldExt)
	}
	// If this panics, it means we generates the coins wrongly
	return run.Coins.MustGet(name).(fext.Element)
}

// GetRandomCoinIntegerVec returns a pre-sampled integer vec random coin. The
// coin should be issued at the same round as it was registered. The same coin
// can't be retrieved more than once. The coin should also have been registered
// as an integer vec before doing this call. Will also trigger the
// "goNextRound" logic if appropriate.
func (run *VerifierRuntime) GetRandomCoinIntegerVec(name coin.Name) []int {
	/*
		Early check, ensures the coin has been registered at all
		and that it has the correct type
	*/
	infos := run.Spec.Coins.Data(name)
	if infos.Type != coin.IntegerVec {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, coin.IntegerVec)
	}
	// If this panics, it means we generates the coins wrongly
	return run.Coins.MustGet(name).([]int)
}

// GetUnivariateParams returns the parameters of a univariate evaluation (i.e:
// x, the evaluation point) and y, the alleged polynomial opening. This is
// intended to resolve parameters that have been provided by the proof.
func (run *VerifierRuntime) GetUnivariateParams(name ifaces.QueryID) query.UnivariateEvalParams {
	return run.QueriesParams.MustGet(name).(query.UnivariateEvalParams)
}

/*
Returns the number of rounds in the assignment.
Deprecated: get it from the CompiledIOP instead
*/
func (run *VerifierRuntime) NumRounds() int {
	/*
		Getting it from the spec is the safest as it is already
		tested. We could fit more assertions here nonetheless.
	*/
	return run.Spec.NumRounds()
}

/*
GetUnivariateEval returns a registered [query.UnivariateEval]. Panic if not found.
Deprecated: get it from the CompiledIOP instead
*/
func (run *VerifierRuntime) GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval {
	return run.Spec.QueriesParams.Data(name).(query.UnivariateEval)
}

// GetColumn returns a column by name. The status of the columns must be
// either proof or public input and the column must be visible to the verifier
// and consequently be available in the proof.
func (run *VerifierRuntime) GetColumn(name ifaces.ColID) ifaces.ColAssignment {

	msgStatus := run.Spec.Columns.Status(name)

	// Sanity-check : the verifier may only access public columns.
	// In case it was not it would be caught by the MustGet below
	// but it's cleaner if the panic happens before.
	if !msgStatus.IsPublic() {
		utils.Panic("the verifier attempted to get message : %v (status %v)", name, msgStatus.String())
	}

	msgIFace := run.Columns.MustGet(name)

	// Just a sanity-check to ensure the message has the right size
	expectedSize := run.Spec.Columns.GetSize(name)
	if msgIFace.Len() != expectedSize {
		utils.Panic("bad dimension %v, spec expected %v, column-name %v", msgIFace.Len(), expectedSize, name)
	}

	return msgIFace
}

// GetInnerProductParams returns the parameters of an inner-product query
// [query.InnerProduct] provided by the proof. The function will panic if the
// query does not exist or if the parameters are not available in the proof.
func (run *VerifierRuntime) GetInnerProductParams(name ifaces.QueryID) query.InnerProductParams {
	return run.QueriesParams.MustGet(name).(query.InnerProductParams)
}

// GetLocalPointEvalParams returns the parameters of a [query.LocalOpening]
// query  (i.e: y, the alleged opening of the query's column at the first
// position.
func (run *VerifierRuntime) GetLocalPointEvalParams(name ifaces.QueryID) query.LocalOpeningParams {
	return run.QueriesParams.MustGet(name).(query.LocalOpeningParams)
}

// GetLogDerivSumParams returns the parameters of a [query.LogDerivativeSum]
func (run *VerifierRuntime) GetLogDerivSumParams(name ifaces.QueryID) query.LogDerivSumParams {
	return run.QueriesParams.MustGet(name).(query.LogDerivSumParams)
}

// GetGrandProductParams returns the parameters of a [query.GrandProduct]
func (run *VerifierRuntime) GetGrandProductParams(name ifaces.QueryID) query.GrandProductParams {
	return run.QueriesParams.MustGet(name).(query.GrandProductParams)
}

// GetHornerParams returns the parameters of a [query.Honer] query.
func (run *VerifierRuntime) GetHornerParams(name ifaces.QueryID) query.HornerParams {
	return run.QueriesParams.MustGet(name).(query.HornerParams)
}

/*
CopyColumnInto implements `column.GetWitness`
Copies the witness into a slice

Deprecated: this is deadcode
*/
func (run VerifierRuntime) CopyColumnInto(name ifaces.ColID, buff *ifaces.ColAssignment) {
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

// GetColumnAt returns the value of a verifier [ifaces.Column] at a specified
// position. This is needed to implement the [column.GetWitness] interface and
// it will only work if the requested column is part of the proof the verifier
// is running on.
func (run VerifierRuntime) GetColumnAt(name ifaces.ColID, pos int) field.Element {
	/*
		Make sure the column is registered. If the name is the one specified
		does not correcpond to a natural column, this will panic. And this is
		expected behaviour.
	*/
	run.Spec.Columns.MustHaveName(name)
	wit := run.Columns.MustGet(name)

	if pos >= wit.Len() || pos < 0 {
		utils.Panic("asked pos %v for vector of size %v", pos, wit.Len())
	}

	return wit.Get(pos)
}

func (run *VerifierRuntime) GetColumnAtBase(name ifaces.ColID, pos int) (field.Element, error) {
	run.Spec.Columns.MustHaveName(name)
	wit := run.Columns.MustGet(name)

	if pos >= wit.Len() || pos < 0 {
		utils.Panic("asked pos %v for vector of size %v", pos, wit)
	}

	if _, err := wit.GetBase(0); err == nil {
		return wit.GetBase(pos)
	} else {
		return field.Zero(), err
	}

}
func (run *VerifierRuntime) GetColumnAtExt(name ifaces.ColID, pos int) fext.Element {
	run.Spec.Columns.MustHaveName(name)
	wit := run.Columns.MustGet(name)

	if pos >= wit.Len() || pos < 0 {
		utils.Panic("asked pos %v for vector of size %v", pos, wit)
	}
	return wit.GetExt(pos)
}

// GetParams extracts the parameters of a query. Will panic if no
// parameters are found
//
// Deprecated: there are already methods to return parameters with an explicit
// type.
func (run *VerifierRuntime) GetParams(name ifaces.QueryID) ifaces.QueryParams {
	return run.QueriesParams.MustGet(name)
}

// GetPublicInput returns a public input from its name
func (run *VerifierRuntime) GetPublicInput(name string) (res fext.GenericFieldElem) {
	allPubs := run.Spec.PublicInputs
	for i := range allPubs {
		if allPubs[i].Name == name {
			if allPubs[i].Acc.IsBase() {
				field, err := allPubs[i].Acc.GetValBase(run)
				if err != nil {
					utils.Panic("error getting public input %v: %v", name, err)
				}
				res.Base = field
				res.IsBase = true
			} else {
				res.Ext = allPubs[i].Acc.GetValExt(run)
				res.IsBase = false
			}
			return res
		}
	}
	utils.Panic("could not find public input nb %v", name)
	return fext.GenericFieldElem{}
}

// Fs returns the Fiat-Shamir state
func (run *VerifierRuntime) Fs() fiatshamir.FS {
	if run.IsBLS {
		return run.BLSFS
	} else {
		return run.KoalaFS
	}
}

// GetSpec returns the compiled IOP
func (run *VerifierRuntime) GetSpec() *CompiledIOP {
	return run.Spec
}

// InsertCoin inserts a coin into the runtime. It should not be
// used by usual verifier action but is useful when implementing
// recursion utilities.
func (run *VerifierRuntime) InsertCoin(name coin.Name, value any) {
	run.Coins.InsertNew(name, value)
}

// GetState returns an arbitrary value stored in the runtime
func (run *VerifierRuntime) GetState(name string) (any, bool) {
	res, ok := run.State[name]
	return res, ok
}

// SetState sets an arbitrary value in the runtime
func (run *VerifierRuntime) SetState(name string, value any) {
	run.State[name] = value
}

// GetQuery returns a query from its name
func (run *VerifierRuntime) GetQuery(name ifaces.QueryID) ifaces.Query {

	if run.Spec.QueriesParams.Exists(name) {
		return run.Spec.QueriesParams.Data(name)
	}

	if run.Spec.QueriesNoParams.Exists(name) {
		return run.Spec.QueriesNoParams.Data(name)
	}

	utils.Panic("could not find query nb %v", name)
	return nil
}
