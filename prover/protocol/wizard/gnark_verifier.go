package wizard

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/sirupsen/logrus"
)

// GnarkVerifierStep functions that can be registered in the CompiledIOP by the successive
// compilation steps. They correspond to "precompiled" verification steps.
type GnarkVerifierStep func(frontend.API, *WizardVerifierCircuit)

// WizardVerifierCircuit the [VerifierRuntime] in a gnark circuit. The complete
// implementation follows this mirror logic.
//
// The sub-circuit employs GKR for MiMC in order to improve the performances
// of the MiMC hashes that occurs during the verifier runtime.
type WizardVerifierCircuit struct {

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
	columnsIDs collection.Mapping[ifaces.ColID, int] `gnark:"-"`
	// Same for univariate query
	univariateParamsIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for inner-product query
	innerProductIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`
	// Same for local-opening query
	localOpeningIDs collection.Mapping[ifaces.QueryID, int] `gnark:"-"`

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
	HasherFactory *gkrmimc.HasherFactory `gnark:"-"`
}

// AllocateWizardCircuit allocates the inner-slices of the verifier struct from a precompiled IOP. It
// is necessary to run this function before calling the [frontend.Compile]
// function as this will pre-allocate all the witness fields of the circuit
// and will allow the gnark compiler to understand how big is the witness of
// the circuit.
func AllocateWizardCircuit(comp *CompiledIOP) (*WizardVerifierCircuit, error) {

	res := newWizardVerifierCircuit()

	for i, colName := range comp.Columns.AllKeys() {
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

		logrus.Tracef("VERIFIER CIRCUIT : registering column %v (as %v) in circuit (#%v)", colName, status.String(), i)
		// deactivate the flag guarding against empty circuits
		size := comp.Columns.GetSize(colName)

		// Allocates the column in the circuit and indexes it
		colID := len(res.Columns)
		res.Columns = append(res.Columns, gnarkutil.AllocateSlice(size))
		res.columnsIDs.InsertNew(colName, colID)
	}

	/*
		Allocate the queries params also. Note that AllKeys does give a
		deterministic order iteration and that's why we do not iterate
		on the map directly.
	*/
	for _, qName := range comp.QueriesParams.AllKeys() {

		/*
			Note that we do not filter out the "already compiled" queries
			here.
		*/
		qInfoIface := comp.QueriesParams.Data(qName)

		switch qInfo := qInfoIface.(type) {
		case query.UnivariateEval:
			// Note that nil is the default value for frontend.Variable
			res.univariateParamsIDs.InsertNew(qName, len(res.UnivariateParams))
			res.UnivariateParams = append(res.UnivariateParams, qInfo.GnarkAllocate())
		case query.InnerProduct:
			// Note that nil is the default value for frontend.Variable
			res.innerProductIDs.InsertNew(qName, len(res.InnerProductParams))
			res.InnerProductParams = append(res.InnerProductParams, qInfo.GnarkAllocate())
		case query.LocalOpening:
			// Note that nil is the default value for frontend.Variable
			res.localOpeningIDs.InsertNew(qName, len(res.LocalOpeningParams))
			res.LocalOpeningParams = append(res.LocalOpeningParams, query.GnarkLocalOpeningParams{})
		}
	}

	res.Spec = comp
	return res, nil
}

// Verify generates the constraints to assess the correctness of a wizard
// transcript. This function has to be called in the context of a
// [frontend.Define] function. Its work mirrors the [Verify] function.
func (c *WizardVerifierCircuit) Verify(api frontend.API) {
	c.HasherFactory = gkrmimc.NewHasherFactory(api)
	c.FS = fiatshamir.NewGnarkFiatShamir(api, c.HasherFactory)
	c.FS.Update(c.Spec.fiatShamirSetup)
	c.generateAllRandomCoins(api)

	logrus.Tracef("Generated the coins")

	for _, roundSteps := range c.Spec.gnarkSubVerifiers.Inner() {
		for _, step := range roundSteps {
			step(api, c)
		}
	}
}

// generateAllRandomCoins is as [VerifierRuntime.generateAllRandomCoins]. Note that the function
// does create constraints via the hasher factory that is inside of `c.FS`.
func (c *WizardVerifierCircuit) generateAllRandomCoins(_ frontend.API) {

	for currRound := 0; currRound < c.Spec.NumRounds(); currRound++ {
		if currRound > 0 {
			/*
				Sanity-check : Make sure all issued random coin have been
				"consumed" by all the verifiers steps, in the round we are
				"closing"
			*/
			toBeConsumed := c.Spec.Coins.AllKeysAt(currRound - 1)
			c.Coins.Exists(toBeConsumed...)

			if !c.Spec.DummyCompiled {

				// Make sure that all messages have been written and use them
				// to update the FS state. Note that we do not need to update
				// FS using the last round of the prover because he is always
				// the last one to "talk" in the protocol.
				toUpdateFS := c.Spec.Columns.AllKeysProofAt(currRound - 1)
				for _, msg := range toUpdateFS {

					msgID := c.columnsIDs.MustGet(msg)
					msgContent := c.Columns[msgID]

					logrus.Tracef("VERIFIER CIRCUIT : Updating the FS oracle with a message - %v", msg)
					c.FS.UpdateVec(msgContent)
				}

				toUpdateFS = c.Spec.Columns.AllKeysPublicInputAt(currRound - 1)
				for _, msg := range toUpdateFS {

					msgID := c.columnsIDs.MustGet(msg)
					msgContent := c.Columns[msgID]

					logrus.Tracef("VERIFIER CIRCUIT : Updating the FS oracle with public input - %v", msg)
					c.FS.UpdateVec(msgContent)
				}

				/*
					Also include the prover's allegations for all evaluations
				*/
				queries := c.Spec.QueriesParams.AllKeysAt(currRound - 1)
				for _, qName := range queries {
					// Implicitly, this will panic whenever we start supporting
					// a new type of query params
					params := c.GetParams(qName)
					params.UpdateFS(c.FS)
				}
			}
		}

		/*
			Then assigns the coins for the new round.
		*/
		toCompute := c.Spec.Coins.AllKeysAt(currRound)
		for _, coinName := range toCompute {
			info := c.Spec.Coins.Data(coinName)
			logrus.Tracef("VERIFIER CIRCUIT : Generate a random coin - %v", coinName)
			switch info.Type {
			case coin.Field:
				value := c.FS.RandomField()
				c.Coins.InsertNew(coinName, value)
			case coin.IntegerVec:
				value := c.FS.RandomManyIntegers(info.Size, info.UpperBound)
				c.Coins.InsertNew(coinName, value)
			}
		}
	}
}

// GetRandomCoinField returns the preassigned value of a random coin as
// [frontend.Variable]. The implementation implicitly checks that the field
// element is of the right type. It mirrors [VerifierRuntime.GetRandomCoinField]
func (c *WizardVerifierCircuit) GetRandomCoinField(name coin.Name) frontend.Variable {
	/*
		Early check, ensures the coin has been registered at all
		and that it has the correct type
	*/
	infos := c.Spec.Coins.Data(name)
	if infos.Type != coin.Field {
		utils.Panic("Coin was registered as %v but got %v", infos.Type, coin.Field)
	}
	// If this panics, it means we generate the coins wrongly
	return c.Coins.MustGet(name).(frontend.Variable)
}

// GetRandomCoinIntegerVec returns a pre-sampled integer vec random coin as an
// array of [frontend.Variable]. The implementation implicitly checks that the
// requested coin does indeed have the type [coin.IntegerVec] and panics if not.
// The function mirror [VerifierRuntime.GetRandomCoinIntegerVec].
func (c *WizardVerifierCircuit) GetRandomCoinIntegerVec(name coin.Name) []frontend.Variable {
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
func (c *WizardVerifierCircuit) GetUnivariateParams(name ifaces.QueryID) query.GnarkUnivariateEvalParams {
	qID := c.univariateParamsIDs.MustGet(name)
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
func (c *WizardVerifierCircuit) GetUnivariateEval(name ifaces.QueryID) query.UnivariateEval {
	return c.Spec.QueriesParams.Data(name).(query.UnivariateEval)
}

// GetInnerProductParams returns pre-assigned parameters for the requested
// [query.InnerProduct] query from the proof. It mirrors the work of
// [VerifierRuntime.GetInnerProductParams]
func (c *WizardVerifierCircuit) GetInnerProductParams(name ifaces.QueryID) query.GnarkInnerProductParams {
	qID := c.innerProductIDs.MustGet(name)
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
func (c *WizardVerifierCircuit) GetLocalPointEvalParams(name ifaces.QueryID) query.GnarkLocalOpeningParams {
	qID := c.localOpeningIDs.MustGet(name)
	return c.LocalOpeningParams[qID]
}

// GetColumns returns the gnark assignment of a column in a gnark circuit. It
// mirrors the function [VerifierRuntime.GetColumn]
func (c *WizardVerifierCircuit) GetColumn(name ifaces.ColID) []frontend.Variable {

	// case where the column is part of the verification key
	if c.Spec.Columns.Status(name) == column.VerifyingKey {
		val := smartvectors.IntoRegVec(c.Spec.Precomputed.MustGet(name))
		res := gnarkutil.AllocateSlice(len(val))
		// Return the column as an array of constants
		for i := range val {
			res[i] = val[i]
		}
		return res
	}

	msgID := c.columnsIDs.MustGet(name)
	wrappedMsg := c.Columns[msgID]

	size := c.Spec.Columns.GetSize(name)
	if len(wrappedMsg) != size {
		utils.Panic("bad dimension %v, spec expected %v", len(wrappedMsg), size)
	}

	return wrappedMsg
}

// GetColumnAt returns the gnark assignment of a column at a requested point in
// a gnark circuit. It mirrors the function [VerifierRuntime.GetColumnAt]
func (c *WizardVerifierCircuit) GetColumnAt(name ifaces.ColID, pos int) frontend.Variable {
	return c.GetColumn(name)[pos]
}

// newWizardVerifierCircuit creates an empty wizard verifier circuit.
// Initializes the underlying structs and collections.
func newWizardVerifierCircuit() *WizardVerifierCircuit {
	res := &WizardVerifierCircuit{}
	res.columnsIDs = collection.NewMapping[ifaces.ColID, int]()
	res.univariateParamsIDs = collection.NewMapping[ifaces.QueryID, int]()
	res.localOpeningIDs = collection.NewMapping[ifaces.QueryID, int]()
	res.innerProductIDs = collection.NewMapping[ifaces.QueryID, int]()
	res.Columns = [][]frontend.Variable{}
	res.UnivariateParams = make([]query.GnarkUnivariateEvalParams, 0)
	res.InnerProductParams = make([]query.GnarkInnerProductParams, 0)
	res.LocalOpeningParams = make([]query.GnarkLocalOpeningParams, 0)
	res.Coins = collection.NewMapping[coin.Name, interface{}]()
	return res
}

// GetWizardVerifierCircuitAssignment assigns values to the wizard verifier
// circuit from a proof. The result of this function can be used to construct a
// gnark assignment circuit involving the verification of Wizard proof.
func GetWizardVerifierCircuitAssignment(comp *CompiledIOP, proof Proof) *WizardVerifierCircuit {

	res := newWizardVerifierCircuit()

	/*
		Assigns the messages. Note that the iteration order is made
		consistent with `AllocateWizardCircuit`
	*/
	for i, colName := range comp.Columns.AllKeys() {

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
		res.columnsIDs.InsertNew(colName, len(res.Columns))
		res.Columns = append(res.Columns, assignedMsg)
	}

	/*
		Assigns the query parameters. Note that the iteration order is
		made deterministic to match the iteration order of the
	*/
	for _, qName := range comp.QueriesParams.AllKeys() {
		/*
			Note that we do not filter out the "already compiled" queries
			here.
		*/
		paramsIface := proof.QueriesParams.MustGet(qName)

		switch params := paramsIface.(type) {

		case query.UnivariateEvalParams:
			res.univariateParamsIDs.InsertNew(qName, len(res.UnivariateParams))
			res.UnivariateParams = append(res.UnivariateParams, params.GnarkAssign())

		case query.InnerProductParams:
			res.innerProductIDs.InsertNew(qName, len(res.InnerProductParams))
			res.InnerProductParams = append(res.InnerProductParams, params.GnarkAssign())

		case query.LocalOpeningParams:
			res.localOpeningIDs.InsertNew(qName, len(res.LocalOpeningParams))
			res.LocalOpeningParams = append(res.LocalOpeningParams, params.GnarkAssign())

		default:
			utils.Panic("unknow type %T", params)
		}
	}

	res.Spec = comp
	return res
}

// GetParams returns a query parameters as a generic interface
func (c *WizardVerifierCircuit) GetParams(id ifaces.QueryID) ifaces.GnarkQueryParams {
	switch t := c.Spec.QueriesParams.Data(id).(type) {
	case query.UnivariateEval:
		return c.GetUnivariateParams(id)
	case query.LocalOpening:
		return c.GetLocalPointEvalParams(id)
	case query.InnerProduct:
		return c.GetInnerProductParams(id)
	default:
		utils.Panic("unexpected type : %T", t)
	}
	panic("unreachable")
}
