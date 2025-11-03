package wizard

import (
	"encoding/json"
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

// GnarkRuntime is the interface implemented by the struct [VerifierCircuit]
// and is used to interact with the GnarkVerifierStep.
type GnarkRuntime interface {
	ifaces.GnarkRuntime
	GetSpec() *CompiledIOP
	GetPublicInput(api frontend.API, name string) frontend.Variable
	GetGrandProductParams(name ifaces.QueryID) query.GnarkGrandProductParams
	GetHornerParams(name ifaces.QueryID) query.GnarkHornerParams
	GetLogDerivSumParams(name ifaces.QueryID) query.GnarkLogDerivSumParams
	GetLocalPointEvalParams(name ifaces.QueryID) query.GnarkLocalOpeningParams
	GetInnerProductParams(name ifaces.QueryID) query.GnarkInnerProductParams
	GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval
	GetUnivariateParams(name ifaces.QueryID) query.GnarkUnivariateEvalParams
	Fs() *fiatshamir.GnarkFiatShamir
	GetHasherFactory() mimc.HasherFactory
	InsertCoin(name coin.Name, value interface{})
	GetState(name string) (any, bool)
	SetState(name string, value any)
	GetQuery(name ifaces.QueryID) ifaces.Query
}

// GnarkVerifierStep functions that can be registered in the CompiledIOP by the
// successive compilation steps. They correspond to "precompiled" verification
// steps.
type GnarkVerifierStep func(frontend.API, GnarkRuntime)

// VerifierCircuitAnalytic collects analytic datas on a verifier circuit
type VerifierCircuitAnalytic struct {
	NumCols            int
	NumUnivariate      int
	NumInnerProduct    int
	NumLogDerivative   int
	NumGrandProduct    int
	NumHorner          int
	NumLocalOpenings   int
	WeightCols         int
	WeightUnivariate   int
	WeightInnerProduct int
	WeightHorner       int
	Details            []string
}

// VerifierCircuit the [VerifierRuntime] in a gnark circuit. The complete
// implementation follows this mirror logic.
//
// The sub-circuit employs GKR for MiMC in order to improve the performances
// of the MiMC hashes that occurs during the verifier runtime.
type VerifierCircuit struct {

	// Spec points to the inner CompiledIOP and carries all the static
	// informations related to the circuit.
	Spec *CompiledIOP `gnark:"-"`

	// Maps a query's name to a position in the arrays below. The reason we
	// use this data-structure is because the [VerifierRuntime] offers
	// key-value access to the internal parameters of the struct and we
	// cannot have maps of [frontend.Variable] in a gnark circuit (because we
	// need a deterministic storage so that we are sure that the wires stay at
	// the same position). The way we solve the problem is by storing the
	// columns and parameters in slices and keeping track of their positions
	// in a map that is not accessed by the gnark compiler. This way we
	// can ensure determinism and are still able to do key-value access in a
	// slightly more convoluted way
	ColumnsIDs collection.Mapping[ifaces.ColID, int] `gnark:"-"`
	// Same for univariate query
	UnivariateParamsIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for inner-product query
	InnerProductIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for local-opening query
	LocalOpeningIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for logDerivativeSum query
	LogDerivSumIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for grand-product query
	GrandProductIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for Horner query
	HornerIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`

	// Columns stores the gnark witness part corresponding to the columns
	// provided in the proof and in the VerifyingKey.
	Columns [][]frontend.Variable `gnark:",secret"`
	// UnivariateParams stores an assignment for each [query.UnivariateParams]
	// from the proof. This is part of the witness of the gnark circuit.
	UnivariateParams []query.GnarkUnivariateEvalParams `gnark:",secret"`
	// InnerProductParams stores an assignment for each [query.InnerProductParams]
	// from the proof. It is part of the witness of the gnark circuit.
	InnerProductParams []query.GnarkInnerProductParams `gnark:",secret"`
	// LocalOpeningParams stores an assignment for each [query.LocalOpeningParams]
	// from the proof. It is part of the witness of the gnark circuit.
	LocalOpeningParams []query.GnarkLocalOpeningParams `gnark:",secret"`
	// LogDerivSumParams stores an assignment for each [query.LogDerivSumParams]
	// from the proof. It is part of the witness of the gnark circuit.
	LogDerivSumParams []query.GnarkLogDerivSumParams `gnark:",secret"`
	// GrandProductParams stores an assignment for each [query.GrandProductParams]
	// from the proof. It is part of the witness of the gnark circuit.
	GrandProductParams []query.GnarkGrandProductParams `gnark:",secret"`
	// HornerParams stores an assignment for each [query.Horner] from
	// the proof. It is part of the witness of the gnark circuit.
	HornerParams []query.GnarkHornerParams `gnark:",secret"`

	// FS is the Fiat-Shamir state, mirroring [VerifierRuntime.FS]. The same
	// cautionnary rules apply to it; e.g. don't use it externally when
	// possible.
	FS *fiatshamir.GnarkFiatShamir `gnark:"-"`

	// Coins stores all the coins sampled by the verifier circuit. It is not
	// part of the witness since the coins are constructed from the assigned
	// proof. We still track them here to mirror how the [VerifierRuntime]
	// works.
	Coins collection.Mapping[coin.Name, interface{}] `gnark:"-"`

	// HasherFactory is a custom hasher that we use for all the MiMC hashing
	// in the circuit. It is used for efficiently computing the Fiat-Shamir
	// hashes but also the MiMC Vortex column hashes that we use for the
	// last round of the self-recursion.
	HasherFactory mimc.HasherFactory `gnark:"-"`

	// State is a generic-purpose data store that the verifier steps can use to
	// communicate with each other across rounds.
	State map[string]interface{} `gnark:"-"`

	// NumRound is the last round of the proof.
	NumRound int
}

// NewVerifierCircuit creates an empty wizard verifier circuit.
// Initializes the underlying structs and collections.
func NewVerifierCircuit(comp *CompiledIOP, numRound int) *VerifierCircuit {

	return &VerifierCircuit{
		Spec: comp,

		ColumnsIDs:          collection.NewMapping[ifaces.ColID, int](),
		UnivariateParamsIDs: collection.NewMapping[ifaces.QueryID, int](),
		LocalOpeningIDs:     collection.NewMapping[ifaces.QueryID, int](),
		InnerProductIDs:     collection.NewMapping[ifaces.QueryID, int](),
		LogDerivSumIDs:      collection.NewMapping[ifaces.QueryID, int](),
		GrandProductIDs:     collection.NewMapping[ifaces.QueryID, int](),
		HornerIDs:           collection.NewMapping[ifaces.QueryID, int](),

		Columns:            [][]frontend.Variable{},
		UnivariateParams:   make([]query.GnarkUnivariateEvalParams, 0),
		InnerProductParams: make([]query.GnarkInnerProductParams, 0),
		LocalOpeningParams: make([]query.GnarkLocalOpeningParams, 0),
		LogDerivSumParams:  make([]query.GnarkLogDerivSumParams, 0),
		HornerParams:       make([]query.GnarkHornerParams, 0),
		Coins:              collection.NewMapping[coin.Name, interface{}](),

		NumRound: numRound,
	}
}

// AllocateWizardCircuit allocates the inner-slices of the verifier struct
// from a precompiled IOP. It is necessary to run this function before
// calling the [frontend.Compile] function as this will pre-allocate all
// the witness fields of the circuit and will allow the gnark compiler to
// understand how big is the witness of the circuit.
func AllocateWizardCircuit(comp *CompiledIOP, numRound int) *VerifierCircuit {

	if numRound == 0 {
		numRound = comp.NumRounds()
	}

	res := NewVerifierCircuit(comp, numRound)

	for _, colName := range comp.Columns.AllKeys() {

		col := comp.Columns.GetHandle(colName)

		if col.Round() >= numRound {
			// the column will not be accessed by the verifier.
			continue
		}

		// filter the columns by status
		status := comp.Columns.Status(colName)
		if !status.IsPublic() {
			// the column is not public so it is not part of the proof
			continue
		}

		if status == column.VerifyingKey {
			// these are constant columns
			continue
		}

		// Allocates the column in the circuit and indexes it
		res.AllocColumn(colName, col.Size())
	}

	/*
		Allocate the queries params also. Note that AllKeys does give a
		deterministic order iteration and that's why we do not iterate
		on the map directly.
	*/
	for _, qName := range comp.QueriesParams.AllKeys() {

		// Note that we do not filter out the "already compiled" queries
		// here.
		qInfoIface := comp.QueriesParams.Data(qName)

		if comp.QueriesParams.Round(qName) >= numRound {
			continue
		}

		switch qInfo := qInfoIface.(type) {
		case query.UnivariateEval:
			res.AllocUnivariateEval(qName, qInfo)
		case query.InnerProduct:
			res.AllocInnerProduct(qName, qInfo)
		case query.LocalOpening:
			res.AllocLocalOpening(qName, qInfo)
		case query.LogDerivativeSum:
			res.AllocLogDerivativeSum(qName, qInfo)
		case query.GrandProduct:
			res.AllocGrandProduct(qName, qInfo)
		case *query.Horner:
			res.AllocHorner(qName, qInfo)
		}
	}

	return res
}

// AssignVerifierCircuit assigns values to the wizard verifier
// circuit from a proof. The result of this function can be used to construct a
// gnark assignment circuit involving the verification of Wizard proof.
func AssignVerifierCircuit(comp *CompiledIOP, proof Proof, numRound int) *VerifierCircuit {

	if numRound == 0 {
		numRound = comp.NumRounds()
	}

	res := NewVerifierCircuit(comp, numRound)

	// Assigns the messages. Note that the iteration order is made
	// consistent with `AllocateWizardCircuit`
	for i, colName := range comp.Columns.AllKeys() {

		col := comp.Columns.GetHandle(colName)

		if col.Round() >= res.NumRound {
			// the column will not be accessed by the verifier.
			continue
		}

		// filter the columns by status
		status := comp.Columns.Status(colName)
		if !status.IsPublic() {
			// the column is not public so it is not part of the proof
			continue
		}

		if status == column.VerifyingKey {
			// this are constant columns
			continue
		}

		logrus.Tracef("VERIFIER CIRCUIT : registering column %v (as %v) in circuit (#%v)", colName, status.String(), i)
		msgDataIFace := proof.Messages.MustGet(colName)
		msgData := msgDataIFace

		// Perform the conversion to frontend.Variable, element by element
		assignedMsg := smartvectors.IntoGnarkAssignment(msgData)
		res.ColumnsIDs.InsertNew(colName, len(res.Columns))
		res.Columns = append(res.Columns, assignedMsg)
	}

	// Assigns the query parameters. Note that the iteration order is
	// made deterministic to match the iteration order of the
	for _, qName := range comp.QueriesParams.AllKeys() {

		if comp.QueriesParams.Round(qName) >= res.NumRound {
			continue
		}

		// Note that we do not filter out the "already compiled" queries
		// here.
		paramsIface := proof.QueriesParams.MustGet(qName)

		switch params := paramsIface.(type) {

		case query.UnivariateEvalParams:
			res.AssignUnivariateEval(qName, params)
		case query.InnerProductParams:
			res.AssignInnerProduct(qName, params)
		case query.LocalOpeningParams:
			res.AssignLocalOpening(qName, params)
		case query.LogDerivSumParams:
			res.AssignLogDerivativeSum(qName, params)
		case query.GrandProductParams:
			res.AssignGrandProduct(qName, params)
		case query.HornerParams:
			res.AssignHorner(qName, params)
		default:
			utils.Panic("unknow type %T", params)
		}
	}

	return res
}

// Verify generates the constraints to assess the correctness of a wizard
// transcript. This function has to be called in the context of a
// [frontend.Define] function. Its work mirrors the [Verify] function.
func (c *VerifierCircuit) Verify(api frontend.API) {

	// Note: the function handles the case where c.HasherFactory == nil.
	// It will instead use a standard MiMC hasher that does not use
	// GKR instead.
	c.FS = fiatshamir.NewGnarkFiatShamir(api, c.HasherFactory)
	c.FS.Update(c.Spec.FiatShamirSetup)

	for round, roundSteps := range c.Spec.SubVerifiers.GetInner() {

		if round >= c.NumRound {
			break
		}

		c.GenerateCoinsForRound(api, round)

		for _, step := range roundSteps {
			step.RunGnark(api, c)
		}
	}
}

// GenerateCoinsForRound runs the FS coin generator for round=currRound,
// it will update the FS state with the assets of currRound-1 and then
// it generates all the coins for the request round.
func (c *VerifierCircuit) GenerateCoinsForRound(api frontend.API, currRound int) {

	if currRound > 0 && !c.Spec.DummyCompiled {

		// Make sure that all messages have been written and use them
		// to update the FS state. Note that we do not need to update
		// FS using the last round of the prover because he is always
		// the last one to "talk" in the protocol.
		toUpdateFS := c.Spec.Columns.AllKeysProofAt(currRound - 1)
		for _, msg := range toUpdateFS {

			if c.Spec.Columns.IsExplicitlyExcludedFromProverFS(msg) {
				continue
			}

			msgContent := c.GetColumn(msg)
			c.FS.UpdateVec(msgContent)
		}

		/*
			Also include the prover's allegations for all evaluations
		*/
		queries := c.Spec.QueriesParams.AllKeysAt(currRound - 1)
		for _, qName := range queries {
			if c.Spec.QueriesParams.IsSkippedFromVerifierTranscript(qName) {
				continue
			}

			params := c.GetParams(qName)
			params.UpdateFS(c.FS)
		}
	}

	if c.Spec.FiatShamirHooksPreSampling.Len() > currRound {
		fsHooks := c.Spec.FiatShamirHooksPreSampling.MustGet(currRound)
		for i := range fsHooks {
			fsHooks[i].RunGnark(api, c)
		}
	}

	seed := c.FS.State()

	// Then assigns the coins for the new round.
	toCompute := c.Spec.Coins.AllKeysAt(currRound)
	for _, coinName := range toCompute {
		if c.Spec.Coins.IsSkippedFromVerifierTranscript(coinName) {
			continue
		}

		cn := c.Spec.Coins.Data(coinName)
		value := cn.SampleGnark(c.FS, seed[0])
		c.Coins.InsertNew(coinName, value)
	}
}

// GetRandomCoinField returns the preassigned value of a random coin as
// [frontend.Variable]. The implementation implicitly checks that the field
// element is of the right type. It mirrors [VerifierRuntime.GetRandomCoinField]
func (c *VerifierCircuit) GetRandomCoinField(name coin.Name) frontend.Variable {
	/*
		Early check, ensures the coin has been registered at all
		and that it has the correct type
	*/
	infos := c.Spec.Coins.Data(name)
	if infos.Type != coin.Field && infos.Type != coin.FieldFromSeed {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, coin.Field)
	}
	// If this panics, it means we generate the coins wrongly
	return c.Coins.MustGet(name).(frontend.Variable)
}

// GetRandomCoinIntegerVec returns a pre-sampled integer vec random coin as an
// array of [frontend.Variable]. The implementation implicitly checks that the
// requested coin does indeed have the type [coin.IntegerVec] and panics if not.
// The function mirror [VerifierRuntime.GetRandomCoinIntegerVec].
func (c *VerifierCircuit) GetRandomCoinIntegerVec(name coin.Name) []frontend.Variable {
	/*
		Early check, ensures the coin has been registered at all
		and that it has the correct type
	*/
	infos := c.Spec.Coins.Data(name)
	if infos.Type != coin.IntegerVec {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, coin.IntegerVec)
	}
	// If this panics, it means we generates the coins wrongly
	return c.Coins.MustGet(name).([]frontend.Variable)
}

// GetUnivariateParams returns the parameters of a univariate evaluation (i.e:
// x, the evaluation point) and y, the alleged polynomial opening. It mirrors
// [VerifierRuntime.GetUnivariateParams].
func (c *VerifierCircuit) GetUnivariateParams(name ifaces.QueryID) query.GnarkUnivariateEvalParams {
	qID := c.UnivariateParamsIDs.MustGet(name)
	params := c.UnivariateParams[qID]

	// Sanity-checks
	info := c.GetUnivariateEval(name)
	if len(info.Pols) != len(params.Ys) {
		utils.Panic("(for %v) inconsistent lengths %v %v", name, len(info.Pols), len(params.Ys))
	}
	return params
}

// GetUnivariateEval univariate eval metadata of the requested query. Panic if
// not found.
func (c *VerifierCircuit) GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval {
	return c.Spec.QueriesParams.Data(name).(query.UnivariateEval)
}

// GetInnerProductParams returns pre-assigned parameters for the requested
// [query.InnerProduct] query from the proof. It mirrors the work of
// [VerifierRuntime.GetInnerProductParams]
func (c *VerifierCircuit) GetInnerProductParams(name ifaces.QueryID) query.GnarkInnerProductParams {
	qID := c.InnerProductIDs.MustGet(name)
	params := c.InnerProductParams[qID]

	// Sanity-checks
	info := c.Spec.QueriesParams.Data(name).(query.InnerProduct)
	if len(info.Bs) != len(params.Ys) {
		utils.Panic("(for %v) inconsistent lengths %v %v", name, len(info.Bs), len(params.Ys))
	}
	return params
}

// GetLocalPointEvalParams returns the parameters for the requested
// [query.LocalPointOpening] query. Its work mirrors the function
// [VerifierRuntime.GetLocalOpeningParams]
func (c *VerifierCircuit) GetLocalPointEvalParams(name ifaces.QueryID) query.GnarkLocalOpeningParams {
	qID := c.LocalOpeningIDs.MustGet(name)
	return c.LocalOpeningParams[qID]
}

// GetLogDerivSumParams returns the parameters for the requested
// [query.LogDerivativeSum] query. Its work mirrors the function
// [VerifierRuntime.GetLogDerivSumParams]
func (c *VerifierCircuit) GetLogDerivSumParams(name ifaces.QueryID) query.GnarkLogDerivSumParams {
	qID := c.LogDerivSumIDs.MustGet(name)
	return c.LogDerivSumParams[qID]
}

// GetGrandProductParams returns the parameters for the requested
// [query.GrandProduct] query. Its work mirrors the function
// [VerifierRuntime.GetGrandProductParams]
func (c *VerifierCircuit) GetGrandProductParams(name ifaces.QueryID) query.GnarkGrandProductParams {
	qID := c.GrandProductIDs.MustGet(name)
	return c.GrandProductParams[qID]
}

// GetHornerPArams returns the parameters for the requested
// [query.Horner] query. Its work mirrors the function [VerifierRuntime.GetHornerParams]
func (c *VerifierCircuit) GetHornerParams(name ifaces.QueryID) query.GnarkHornerParams {
	qID := c.HornerIDs.MustGet(name)
	return c.HornerParams[qID]
}

// GetColumns returns the gnark assignment of a column in a gnark circuit. It
// mirrors the function [VerifierRuntime.GetColumn]
func (c *VerifierCircuit) GetColumn(name ifaces.ColID) []frontend.Variable {

	// case where the column is part of the verification key
	if c.Spec.Columns.Status(name) == column.VerifyingKey {
		val := smartvectors.IntoRegVec(c.Spec.Precomputed.MustGet(name))
		res := make([]frontend.Variable, len(val))
		// Return the column as an array of constants
		for i := range val {
			res[i] = val[i].String()
		}
		return res
	}

	msgID := c.ColumnsIDs.MustGet(name)
	wrappedMsg := c.Columns[msgID]

	size := c.Spec.Columns.GetSize(name)
	if len(wrappedMsg) != size {
		utils.Panic("bad dimension %v, spec expected %v", len(wrappedMsg), size)
	}

	return wrappedMsg
}

// GetColumnAt returns the gnark assignment of a column at a requested point in
// a gnark circuit. It mirrors the function [VerifierRuntime.GetColumnAt]
func (c *VerifierCircuit) GetColumnAt(name ifaces.ColID, pos int) frontend.Variable {
	return c.GetColumn(name)[pos]
}

// GetParams returns a query parameters as a generic interface
func (c *VerifierCircuit) GetParams(id ifaces.QueryID) ifaces.GnarkQueryParams {
	switch t := c.Spec.QueriesParams.Data(id).(type) {
	case query.UnivariateEval:
		return c.GetUnivariateParams(id)
	case query.LocalOpening:
		return c.GetLocalPointEvalParams(id)
	case query.LogDerivativeSum:
		return c.GetLogDerivSumParams(id)
	case query.InnerProduct:
		return c.GetInnerProductParams(id)
	case query.GrandProduct:
		return c.GetGrandProductParams(id)
	case *query.LogDerivativeSum:
		return c.GetLogDerivSumParams(id)
	case *query.Horner:
		return c.GetHornerParams(id)
	default:
		utils.Panic("unexpected type : %T", t)
	}
	panic("unreachable")
}

// AllocColumn inserts a column in the Wizard verifier circuit and is meant
// to be called at allocation time.
func (c *VerifierCircuit) AllocColumn(id ifaces.ColID, size int) []frontend.Variable {
	column := make([]frontend.Variable, size)
	c.ColumnsIDs.InsertNew(id, len(c.Columns))
	c.Columns = append(c.Columns, column)
	return column
}

// AssignColumn assigns a column in the Wizard verifier circuit
func (c *VerifierCircuit) AssignColumn(id ifaces.ColID, sv smartvectors.SmartVector) {
	column := smartvectors.IntoGnarkAssignment(sv)
	c.ColumnsIDs.InsertNew(id, len(c.Columns))
	c.Columns = append(c.Columns, column)
}

// AllocUnivariableEval inserts a slot for a univariate query opening in the
// witness of the verifier circuit.
func (c *VerifierCircuit) AllocUnivariateEval(qName ifaces.QueryID, qInfo query.UnivariateEval) {
	// Note that nil is the default value for frontend.Variable
	c.UnivariateParamsIDs.InsertNew(qName, len(c.UnivariateParams))
	c.UnivariateParams = append(c.UnivariateParams, qInfo.GnarkAllocate())
}

// AllocInnerProduct inserts a slot for an inner-product query opening in the
// witness of the verifier circuit.
func (c *VerifierCircuit) AllocInnerProduct(qName ifaces.QueryID, qInfo query.InnerProduct) {
	// Note that nil is the default value for frontend.Variable
	c.InnerProductIDs.InsertNew(qName, len(c.InnerProductParams))
	c.InnerProductParams = append(c.InnerProductParams, qInfo.GnarkAllocate())
}

// AllocLocalOpening inserts a slot for a local position opening in the witness
// of the verifier circuit.
func (c *VerifierCircuit) AllocLocalOpening(qName ifaces.QueryID, qInfo query.LocalOpening) {
	// Note that nil is the default value for frontend.Variable
	c.LocalOpeningIDs.InsertNew(qName, len(c.LocalOpeningParams))
	c.LocalOpeningParams = append(c.LocalOpeningParams, query.GnarkLocalOpeningParams{})
}

// AllocLogDerivativeSum inserts a slot for a log-derivative sum in the witness
// of the verifier circuit.
func (c *VerifierCircuit) AllocLogDerivativeSum(qName ifaces.QueryID, qInfo query.LogDerivativeSum) {
	c.LogDerivSumIDs.InsertNew(qName, len(c.LogDerivSumParams))
	c.LogDerivSumParams = append(c.LogDerivSumParams, query.GnarkLogDerivSumParams{})
}

// AllocGrandProduct inserts a slot for a log-derivative sum in the witness
// of the verifier circuit.
func (c *VerifierCircuit) AllocGrandProduct(qName ifaces.QueryID, qInfo query.GrandProduct) {
	c.GrandProductIDs.InsertNew(qName, len(c.GrandProductParams))
	c.GrandProductParams = append(c.GrandProductParams, query.GnarkGrandProductParams{})
}

// AllocHorner inserts a slot for a log-derivative sum in the witness
// of the verifier circuit.
func (c *VerifierCircuit) AllocHorner(qName ifaces.QueryID, qInfo *query.Horner) {
	c.HornerIDs.InsertNew(qName, len(c.HornerParams))
	c.HornerParams = append(c.HornerParams, query.GnarkHornerParams{
		Parts: make([]query.HornerParamsPartGnark, len(qInfo.Parts)),
	})
}

// AssignUnivariableEval assigns the parameters of a [query.UnivariateEval]
// in the witness of the verifier circuit.
func (c *VerifierCircuit) AssignUnivariateEval(qName ifaces.QueryID, params query.UnivariateEvalParams) {
	// Note that nil is the default value for frontend.Variable
	c.UnivariateParamsIDs.InsertNew(qName, len(c.UnivariateParams))
	c.UnivariateParams = append(c.UnivariateParams, params.GnarkAssign())
}

// AssignInnerProduct assigns the parameters of an [query.InnerProduct]
// in the the witnesss of the verifier circuit.
func (c *VerifierCircuit) AssignInnerProduct(qName ifaces.QueryID, params query.InnerProductParams) {
	// Note that nil is the default value for frontend.Variable
	c.InnerProductIDs.InsertNew(qName, len(c.InnerProductParams))
	c.InnerProductParams = append(c.InnerProductParams, params.GnarkAssign())
}

// AssignLocalOpening assigns the parameters of a [query.LocalOpening] into
// the witness of the verifier circuit.
func (c *VerifierCircuit) AssignLocalOpening(qName ifaces.QueryID, params query.LocalOpeningParams) {
	// Note that nil is the default value for frontend.Variable
	c.LocalOpeningIDs.InsertNew(qName, len(c.LocalOpeningParams))
	c.LocalOpeningParams = append(c.LocalOpeningParams, params.GnarkAssign())
}

// AssignLogDerivativeSum assigns the parameters of a [query.LogDerivativeSum]
// into the witness of the verifier circuit.
func (c *VerifierCircuit) AssignLogDerivativeSum(qName ifaces.QueryID, params query.LogDerivSumParams) {
	// Note that nil is the default value for frontend.Variable
	c.LogDerivSumIDs.InsertNew(qName, len(c.LogDerivSumParams))
	c.LogDerivSumParams = append(c.LogDerivSumParams, query.GnarkLogDerivSumParams{Sum: params.Sum})
}

// AssignGrandProduct assigns the parameters of a [query.GrandProduct]
// into the witness of the verifier circuit.
func (c *VerifierCircuit) AssignGrandProduct(qName ifaces.QueryID, params query.GrandProductParams) {
	// Note that nil is the default value for frontend.Variable
	c.GrandProductIDs.InsertNew(qName, len(c.GrandProductParams))
	c.GrandProductParams = append(c.GrandProductParams, query.GnarkGrandProductParams{Prod: params.Y})
}

// AssignHorner assigns the parameters of a [query.Horner] into the witness
// of the verifier circuit.
func (c *VerifierCircuit) AssignHorner(qName ifaces.QueryID, params query.HornerParams) {
	// Note that nil is the default value for frontend.Variable
	c.HornerIDs.InsertNew(qName, len(c.HornerParams))
	parts := make([]query.HornerParamsPartGnark, len(params.Parts))
	for i := range params.Parts {
		parts[i].N0 = params.Parts[i].N0
		parts[i].N1 = params.Parts[i].N1
	}
	c.HornerParams = append(c.HornerParams, query.GnarkHornerParams{
		FinalResult: params.FinalResult,
		Parts:       parts,
	})
}

// GetPublicInput returns a public input value from its name
func (c *VerifierCircuit) GetPublicInput(api frontend.API, name string) frontend.Variable {
	allPubs := c.Spec.PublicInputs
	for i := range allPubs {
		if allPubs[i].Name == name {
			return allPubs[i].Acc.GetFrontendVariable(api, c)
		}
	}

	// At this point, the public input has not been found so we will panic, but
	// before that we consolidate the list of the public input names.
	allPubNames := []string{}
	for i := range c.Spec.PublicInputs {
		allPubNames = append(allPubNames, c.Spec.PublicInputs[i].Name)
	}

	utils.Panic("could not find public input nb %v, list of public inputs: %v", name, allPubNames)
	return field.Element{}
}

// Fs returns the Fiat-Shamir state of the verifier circuit
func (c *VerifierCircuit) Fs() *fiatshamir.GnarkFiatShamir {
	return c.FS
}

// SetFs sets the Fiat-Shamir state of the verifier circuit
func (c *VerifierCircuit) SetFs(fs *fiatshamir.GnarkFiatShamir) {
	c.FS = fs
}

// GetHasherFactory returns the hasher factory of the verifier circuit; nil
// if none is set.
func (c *VerifierCircuit) GetHasherFactory() mimc.HasherFactory {
	return c.HasherFactory
}

// SetHasherFactory sets the hasher factory of the verifier circuit
func (c *VerifierCircuit) SetHasherFactory(hf mimc.HasherFactory) {
	c.HasherFactory = hf
}

// GetSpec returns the compiled IOP of the verifier circuit
func (c *VerifierCircuit) GetSpec() *CompiledIOP {
	return c.Spec
}

// InsertCoin inserts a coin in the verifier circuit. This has
// a use for implementing recursive application.
func (c *VerifierCircuit) InsertCoin(name coin.Name, value interface{}) {
	c.Coins.InsertNew(name, value)
}

// GetState returns the value of a state variable in the verifier circuit
func (c *VerifierCircuit) GetState(name string) (any, bool) {
	res, ok := c.State[name]
	return res, ok
}

// SetState sets the value of a state variable in the verifier circuit
func (c *VerifierCircuit) SetState(name string, value any) {
	c.State[name] = value
}

// GetQuery returns a query from its name
func (c *VerifierCircuit) GetQuery(name ifaces.QueryID) ifaces.Query {
	if c.Spec.QueriesParams.Exists(name) {
		return c.Spec.QueriesParams.Data(name)
	}
	if c.Spec.QueriesNoParams.Exists(name) {
		return c.Spec.QueriesNoParams.Data(name)
	}
	utils.Panic("could not find query nb %v", name)
	return nil
}

// Analyze returns a cell count for each type of query and/or column
func (c *VerifierCircuit) Analyze() *VerifierCircuitAnalytic {

	res := &VerifierCircuitAnalytic{}

	for i := range c.Columns {
		res.NumCols++
		res.WeightCols += len(c.Columns[i])
	}

	for i := range c.HornerParams {
		res.NumHorner++
		res.WeightHorner += 2*len(c.HornerParams[i].Parts) + 1
	}

	for i := range c.InnerProductParams {
		res.NumInnerProduct++
		res.WeightInnerProduct += len(c.InnerProductParams[i].Ys)
	}

	for i := range c.UnivariateParams {
		res.NumUnivariate++
		res.WeightUnivariate += len(c.UnivariateParams[i].Ys)
	}

	res.NumGrandProduct += len(c.GrandProductParams)
	res.NumLogDerivative += len(c.LogDerivSumParams)
	res.NumLocalOpenings += len(c.LocalOpeningParams)

	return res
}

// WithDetails adds details for every column into the verifier analytic. The
// function returns a pointer to the receiver of the call.
func (a *VerifierCircuitAnalytic) WithDetails(c *VerifierCircuit) *VerifierCircuitAnalytic {

	comp := c.GetSpec()

	for _, colName := range comp.Columns.AllKeys() {
		id, ok := c.ColumnsIDs.TryGet(colName)
		if !ok {
			continue
		}

		value := c.Columns[id]
		a.addDetail(fmt.Sprintf("[Column] size-circuit=%v name=%v", len(value), colName))
	}

	for _, queryName := range comp.QueriesParams.AllKeys() {

		q := c.Spec.QueriesParams.Data(queryName)
		switch q.(type) {

		case query.InnerProduct:
			id, ok := c.InnerProductIDs.TryGet(queryName)
			if !ok {
				continue
			}
			value := c.InnerProductParams[id]
			a.addDetail(fmt.Sprintf("[InnerProduct] size-circuit=%v name=%v", len(value.Ys), queryName))

		case query.GrandProduct:
			_, ok := c.GrandProductIDs.TryGet(queryName)
			if !ok {
				continue
			}
			a.addDetail(fmt.Sprintf("[GrandProduct] size-circuit=%v name=%v", 1, queryName))

		case *query.LocalOpening:
			_, ok := c.LocalOpeningIDs.TryGet(queryName)
			if !ok {
				continue
			}
			a.addDetail(fmt.Sprintf("[LocalOpening] size-circuit=%v name=%v", 1, queryName))

		case *query.Horner:
			id, ok := c.HornerIDs.TryGet(queryName)
			if !ok {
				continue
			}
			value := c.HornerParams[id]
			a.addDetail(fmt.Sprintf("[Horner] size-circuit=%v name=%v", len(value.Parts), queryName))

		case *query.LogDerivativeSum:
			_, ok := c.LogDerivSumIDs.TryGet(queryName)
			if !ok {
				continue
			}
			a.addDetail(fmt.Sprintf("[LogDerivativeSum] size-circuit=%v name=%v", 1, queryName))

		case query.UnivariateEval:
			id, ok := c.UnivariateParamsIDs.TryGet(queryName)
			if !ok {
				continue
			}
			value := c.UnivariateParams[id]
			a.addDetail(fmt.Sprintf("[UnivariateEval] size-circuit=%v name=%v", len(value.Ys), queryName))
		}
	}

	return a
}

// addDetail adds a detail to the verifier analytic
func (a *VerifierCircuitAnalytic) addDetail(detail string) {
	a.Details = append(a.Details, detail)
}

func (a *VerifierCircuitAnalytic) JsonString() string {
	b, e := json.Marshal(a)
	if e != nil {
		panic(e)
	}
	return string(b)
}
