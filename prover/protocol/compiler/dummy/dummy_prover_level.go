package dummy

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/sirupsen/logrus"
)

// alreadyDoneInPreviousDummyProverAction is a key in [comp.ExtraData] to mark
// queries or verifier-actions that have already been marked in the previous
// [DummyProverAction].
const alreadyDoneInPreviousDummyProverAction = "AlreadyDoneInPreviousDummyProverAction"

// Option is an option for the compiler
type Option func(*OptionSet)

// WithMsg tells the [CompileAtProverLvl] to use the given message
// in the panic messages in case it fails. This is handy to differentiate
// which invokation of the compiler failed.
func WithMsg(msg string) Option {
	return func(o *OptionSet) {
		o.Msg = msg
	}
}

// OptionSet is a set of options that can be passed to the compiler.
type OptionSet struct {
	// Msg is an identifier shown in the panic message of the [CompileAtProverLvl]
	// to help identifying which invokation of the compiler failed.
	Msg string
}

// CompileAtProverLvl instantiate the oracle as the prover. Meaning that the
// prover is responsible for checking all the queries and the verifier does not
// see any compiled IOP.
//
// This is useful for quick "manual" testing and debugging. One perk is that
// unlike [CompileAtVerifierLvl] the FS is not pressured as we don't push many
// column in plain-text to the verifier. The drawback is that since it happens
// at prover level, the "errors" result in panics. This makes it not very
// suitable for established unit-tests where we want to analyze the errors.
func CompileAtProverLvl(opts ...Option) func(*wizard.CompiledIOP) {
	os := &OptionSet{}
	for _, opt := range opts {
		opt(os)
	}
	return func(comp *wizard.CompiledIOP) {
		compileAtProverLvl(comp, os)
	}
}

// DoneOperation is an operation done during a round. It used to help avoid the
// CompilerAtProverLvl to recompile the same verification steps when called
// multiple times.
//
// The reason this structure exists (it used to be a locally named structure in
// the body of the DummyAtProverLvl compiler) is that we need it to be named
// publicly so that the serializer can handle it.
type DoneOperation struct {
	Type       byte // 0 for params, 1 for no-params, 2 for verifier-action
	Round      int  // round in which the operation was done
	PosInRound int  // # of the operation in the round
}

// DoneOperationSet wraps a set of [DoneOperation]s.
//
// The reason this structure exists (it used to be a locally named structure in
// the body of the DummyAtProverLvl compiler) is that we need it to be named
// publicly so that the serializer can handle it.
//
// Also, it used to be a map[DoneOperation]struct{} in the body of the compiler
// but the serializer has issues serializing that.
type DoneOperationSet struct {
	S map[DoneOperation]int
}

func compileAtProverLvl(comp *wizard.CompiledIOP, os *OptionSet) {

	/*
		Registers all declared commitments and query parameters
		as messages in the same round. This steps is only relevant
		for the compiledIOP. We elaborate on how to update the provers
		and verifiers to account for this. Additionally, we take the queries
		as we compile them from the `CompiledIOP`.
	*/
	numRounds := comp.NumRounds()

	for round := 0; round < numRounds; round++ {

		if _, foundMap := comp.ExtraData[alreadyDoneInPreviousDummyProverAction]; !foundMap {
			alreadyDonePlain := DoneOperationSet{
				S: map[DoneOperation]int{},
			}
			alreadyDone := &alreadyDonePlain
			comp.ExtraData[alreadyDoneInPreviousDummyProverAction] = alreadyDone
		}

		alreadyDone := comp.ExtraData[alreadyDoneInPreviousDummyProverAction].(*DoneOperationSet)

		// The filter returns true, as long as the query has not been marked as
		// already compiled. This is to avoid them being compiled a second time.
		queriesParamsToCompile := []ifaces.QueryID{}
		queriesNoParamsToCompile := []ifaces.QueryID{}
		verifierActions := []wizard.VerifierAction{}

		for i, qName := range comp.QueriesParams.AllKeysAt(round) {
			di := DoneOperation{Type: 0, Round: round, PosInRound: i}
			if comp.QueriesParams.IsIgnored(qName) {
				continue
			}
			if _, found := alreadyDone.S[di]; found {
				continue
			}
			alreadyDone.S[di] = 1
			queriesParamsToCompile = append(queriesParamsToCompile, qName)
		}

		for i, qName := range comp.QueriesNoParams.AllKeysAt(round) {
			di := DoneOperation{Type: 1, Round: round, PosInRound: i}
			if comp.QueriesNoParams.IsIgnored(qName) {
				continue
			}
			if _, found := alreadyDone.S[di]; found {
				continue
			}
			alreadyDone.S[di] = 1
			queriesNoParamsToCompile = append(queriesNoParamsToCompile, qName)
		}

		for i := range comp.SubVerifiers.GetOrEmpty(round) {
			di := DoneOperation{Type: 2, Round: round, PosInRound: i}
			if _, found := alreadyDone.S[di]; found {
				continue
			}
			alreadyDone.S[di] = 1
			verifierActions = append(verifierActions, comp.SubVerifiers.GetOrEmpty(round)[i])
		}

		if len(queriesNoParamsToCompile)+len(queriesParamsToCompile)+len(verifierActions) == 0 {
			continue
		}

		// One step per round as this can catch problems before posteriorally-
		// defined prover-steps are run.
		comp.RegisterProverAction(round, &DummyProverAction{
			Comp:                     comp,
			QueriesParamsToCompile:   queriesParamsToCompile,
			QueriesNoParamsToCompile: queriesNoParamsToCompile,
			VerifierActions:          verifierActions,
			Os:                       os,
		})
	}
}

// DummyProverAction is the action to verify queries at the prover level.
// It implements the [wizard.ProverAction] interface.
type DummyProverAction struct {
	Comp                     *wizard.CompiledIOP
	QueriesParamsToCompile   []ifaces.QueryID
	QueriesNoParamsToCompile []ifaces.QueryID
	VerifierActions          []wizard.VerifierAction
	Os                       *OptionSet
}

// Run executes the dummy verification by checking all queries.
func (a *DummyProverAction) Run(run *wizard.ProverRuntime) {

	if a.Os == nil {
		logrus.Infof("started to run the dummy verifier for step")
	} else if len(a.Os.Msg) > 0 {
		logrus.Infof("started to run the dummy verifier for step %v", a.Os.Msg)
	}

	var finalErr error
	lock := sync.Mutex{}

	countDone := uint64(0)
	countTotal := uint32(len(a.QueriesParamsToCompile) + len(a.QueriesNoParamsToCompile))
	bumpCounterDone := func() {
		loaded := atomic.AddUint64(&countDone, 1)
		if loaded%1000 == 0 {
			logrus.Infof("finished to run the dummy verifier for step %v, progress=%v/%v", a.Os.Msg, loaded, countTotal)
		}
	}

	// Test all the query with parameters
	parallel.Execute(len(a.QueriesParamsToCompile), func(start, stop int) {

		for i := start; i < stop; i++ {
			name := a.QueriesParamsToCompile[i]
			lock.Lock()
			q := a.Comp.QueriesParams.Data(name)
			lock.Unlock()
			if err := q.Check(run); err != nil {
				err = fmt.Errorf("%v\nfailed %v - %v", finalErr, name, err)
				lock.Lock()
				finalErr = errors.Join(finalErr, err)
				lock.Unlock()
				logrus.Debugf("query %v failed\n", name)
			} else {
				logrus.Debugf("query %v passed\n", name)
			}
			bumpCounterDone()
		}
	})

	// Test the queries without parameters
	parallel.Execute(len(a.QueriesNoParamsToCompile), func(start, stop int) {

		for i := start; i < stop; i++ {
			name := a.QueriesNoParamsToCompile[i]
			lock.Lock()
			q := a.Comp.QueriesNoParams.Data(name)
			lock.Unlock()
			if err := q.Check(run); err != nil {
				err = fmt.Errorf("verifier step failed %v - %v", name, err)
				lock.Lock()
				finalErr = errors.Join(finalErr, err)
				lock.Unlock()
			} else {
				logrus.Debugf("query %v passed\n", name)
			}
			bumpCounterDone()
		}
	})

	// Run the verifier actions
	parallel.Execute(len(a.VerifierActions), func(start, stop int) {

		for i := start; i < stop; i++ {
			if err := a.VerifierActions[i].Run(run); err != nil {
				err = fmt.Errorf("verifier step failed %T - %v", a.VerifierActions[i], err)
				lock.Lock()
				finalErr = errors.Join(finalErr, err)
				lock.Unlock()
				bumpCounterDone()
			}
		}
	})

	if finalErr != nil {
		utils.Panic("dummy.Compile brought errors: msg=%v: err=%v", a.Os.Msg, finalErr.Error())
	}
}
