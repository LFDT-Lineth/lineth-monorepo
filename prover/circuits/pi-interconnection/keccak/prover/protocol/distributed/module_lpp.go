package distributed

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// ModuleLPP is a compilation structure holding the central informations
// of the LPP part of a module.
type ModuleLPP struct {

	// ModuleTranslator is the translator for the GL part of the module
	// it also has the ownership of the [wizard.Compiled] IOP built for
	// this module.
	ModuleTranslator

	// DefinitionInput stores the [FilteredModuleInputs] that was used
	// to generate the module.
	DefinitionInput FilteredModuleInputs

	// SegmentModuleIndex is the index of the module in the segment. The value
	// is used to assign a column and check that "isFirst" and "isLast" are
	// right-fully computed.
	SegmentModuleIndex ifaces.Column

	// InitialFiatShamirState is the state at which to start the FiatShamir
	// computation
	InitialFiatShamirState ifaces.Column

	// N0Hash is the hash of the N0 positions for the Horner queries
	N0Hash ifaces.Column

	// N1Hash is the hash of the N1 positions for the Horner queries
	N1Hash ifaces.Column

	// LogDerivativeSum is the translated log-derivative query in the module
	LogDerivativeSum *query.LogDerivativeSum

	// GrandProduct is the translated grand-product query in the module
	GrandProduct *query.GrandProduct

	// Horner is the translated horner query in the module
	Horner *query.Horner

	// PublicInput contains the list of the public inputs for the current LPP
	// module.
	PublicInputs LimitlessPublicInput[wizard.PublicInput]
}

// SetInitialFSHash sets the initial FiatShamir state
type SetInitialFSHash struct {
	ModuleLPP
	skipped bool `serde:"omit"`
}

// AssignLPPQueries is a [wizard.ProverAction] responsible for assigning the LPP
// query results as well as the N0Hash and N1Hash values.
type AssignLPPQueries struct {
	ModuleLPP
}

// CheckNxHash is a [wizard.VerifierAction] responsible for checking the N0Hash
// and N1Hash values.
type CheckNxHash struct {
	ModuleLPP
	skipped bool `serde:"omit"`
}

// LppWitnessAssignment is a [wizard.ProverAction] responsible for assigning the
// LPP witness values at round "round".
type LppWitnessAssignment struct {
	ModuleLPP
	Round int
}

// BuildModuleLPP builds a [ModuleLPP] from scratch from a [FilteredModuleInputs].
// The function works by creating a define function that will call [NewModuleLPP]
// / and then it calls [wizard.Compile] without passing compilers.
func BuildModuleLPP(moduleInput FilteredModuleInputs) *ModuleLPP {

	var (
		moduleLPP  *ModuleLPP
		defineFunc = func(b *wizard.Builder) {
			moduleLPP = NewModuleLPP(b, moduleInput)
		}
		// Since the NewModuleLPP contains a pointer to the compiled IOP already
		// there is no need to use the one returned by [wizard.Compile].
		_ = wizard.Compile(defineFunc)
	)

	return moduleLPP
}

// NewModuleLPP declares and constructs a new [ModuleLPP] from a [wizard.Builder]
// and a [FilteredModuleInput]. The function performs all the necessary
// declarations to build the LPP part of the module and returns the constructed
// module.
func NewModuleLPP(builder *wizard.Builder, moduleInput FilteredModuleInputs) *ModuleLPP {

	moduleLPP := &ModuleLPP{
		ModuleTranslator: ModuleTranslator{
			Wiop: builder.CompiledIOP,
			Disc: moduleInput.Disc,
		},
		DefinitionInput:        moduleInput,
		InitialFiatShamirState: builder.InsertProof(0, "INITIAL_FIATSHAMIR_STATE", 1),
	}

	for _, col := range moduleInput.Columns {

		if col.Round() != 0 {
			utils.Panic("cannot translate a column with non-zero round %v", col.Round())
		}

		_, isLPP := moduleInput.ColumnsLPPSet[col.GetColID()]

		if !isLPP {
			continue
		}

		if data, isPrecomp := moduleInput.ColumnsPrecomputed[col.GetColID()]; isPrecomp {
			moduleLPP.InsertPrecomputed(*col, data)
			continue
		}

		moduleLPP.InsertColumn(*col, 0)
	}

	if len(moduleInput.LogDerivativeArgs) > 0 {
		q := moduleLPP.InsertLogDerivative(
			1,
			ifaces.QueryID("MAIN_LOGDERIVATIVE"),
			moduleInput.LogDerivativeArgs,
		)
		moduleLPP.LogDerivativeSum = &q
	}

	if len(moduleInput.GrandProductArgs) > 0 {
		q := moduleLPP.InsertGrandProduct(
			1,
			ifaces.QueryID("MAIN_GRANDPRODUCT"),
			moduleInput.GrandProductArgs,
		)
		moduleLPP.GrandProduct = &q
	}

	if len(moduleInput.HornerArgs) > 0 {
		q := moduleLPP.InsertHorner(
			1,
			ifaces.QueryID("MAIN_HORNER"),
			moduleInput.HornerArgs,
		)
		moduleLPP.Horner = &q

		moduleLPP.N0Hash = moduleLPP.Wiop.InsertProof(1, "N0_HASH", 1)
		moduleLPP.N1Hash = moduleLPP.Wiop.InsertProof(1, "N1_HASH", 1)
	}

	// In case the LPP part is empty, we have a scenario where the sub-proof to
	// build has no registered coin. This creates errors in the compilation
	// due to sanity-check firing up. We add a coin to remediate.
	moduleLPP.InsertCoin(coin.Namef("LPP_DUMMY_COIN_%v", 1), 1)

	moduleLPP.declarePublicInput()

	moduleLPP.Wiop.RegisterProverAction(1, &AssignLPPQueries{*moduleLPP})
	moduleLPP.Wiop.RegisterVerifierAction(1, &CheckNxHash{ModuleLPP: *moduleLPP})
	moduleLPP.Wiop.FiatShamirHooksPreSampling.AppendToInner(1, &SetInitialFSHash{ModuleLPP: *moduleLPP})

	return moduleLPP
}

// ModuleName returns the module name of the [ModuleLPP].
func (m *ModuleLPP) ModuleName() ModuleName {
	return m.DefinitionInput.ModuleName
}

// GetMainProverStep returns a [wizard.ProverStep] running [Assign] passing
// the provided [ModuleWitness] argument.
func (m *ModuleLPP) GetMainProverStep(witness *ModuleWitnessLPP) wizard.MainProverStep {
	return func(run *wizard.ProverRuntime) {
		m.Assign(run, witness)
	}
}

// Assign is the entry-point for the assignment of the [ModuleLPP]. It
// is responsible for setting up the [ProverRuntime.State] with the witness
// value and assigning the columns.
//
// The function will only populates the [ModuleWitness] from its columns
// assignment as the columns are assigned in the runtime. The function will
// only assign the columns corresponding to the first submodule The
// others are done via [LppWitnessAssignment] with round > 0.
func (m *ModuleLPP) Assign(run *wizard.ProverRuntime, witness *ModuleWitnessLPP) {
	run.State.InsertNew(moduleWitnessKey, witness)

	run.AssignColumn(
		m.InitialFiatShamirState.GetColID(),
		smartvectors.NewConstant(witness.InitialFiatShamirState, 1),
	)

	a := LppWitnessAssignment{ModuleLPP: *m, Round: 0}
	a.Run(run)

	m.assignPublicInput(run, witness)
}

func (a LppWitnessAssignment) Run(run *wizard.ProverRuntime) {

	var (
		witness         = run.State.MustGet(moduleWitnessKey).(*ModuleWitnessLPP)
		m               = a.ModuleLPP
		round           = a.Round
		definitionInput = m.DefinitionInput
	)

	if witness.ModuleIndex != m.DefinitionInput.ModuleIndex {
		utils.Panic("witness.ModuleIndex: %v != m.DefinitionInput.ModuleIndex: %v", witness.ModuleIndex, m.DefinitionInput.ModuleIndex)
	}

	if witness.ModuleName != m.DefinitionInput.ModuleName {
		utils.Panic("witness.ModuleName: %v != m.DefinitionInput.ModuleName: %v", witness.ModuleName, m.DefinitionInput.ModuleName)
	}

	// [definitionInput.Columns] stores the list of columns to assign.
	// Though, it stores the columns as in the origin CompiledIOP so we
	// cannot directly use them to refer to columns of the current IOP.
	// Yet, the column share the same names.
	for _, col := range definitionInput.Columns {

		colName := col.GetColID()

		// Skips the non-LPP columns
		if _, ok := definitionInput.ColumnsLPPSet[colName]; !ok {
			continue
		}

		newCol := m.Wiop.Columns.GetHandle(colName)

		if newCol.Round() != round {
			continue
		}

		if m.Wiop.Precomputed.Exists(colName) {
			continue
		}

		colWitness, ok := witness.Columns[colName]
		if !ok {
			utils.Panic(
				"witness of column %v was not found, module=%v witness-columns=%v module-columns=%v module-column-LPP=%v",
				colName,
				m.ModuleName(),
				utils.SortedKeysOf(witness.Columns, func(a, b ifaces.ColID) bool { return a < b }),
				m.DefinitionInput.Columns,
				m.DefinitionInput.ColumnsLPPSet,
			)
		}

		run.AssignColumn(colName, colWitness)
		delete(witness.Columns, colName)
	}
}

// addCoinFromExpression scans the metadata of the expression looking
// for coins and adds them to the [ModuleLPP] as [coin.FieldFromSeed].
func (m *ModuleLPP) addCoinFromExpression(exprs ...*symbolic.Expression) {

	for _, expr := range exprs {

		metadata := expr.BoardListVariableMetadata()

		for i := range metadata {

			switch meta := metadata[i].(type) {

			case coin.Info:

				m.InsertCoin(meta.Name, meta.Round)
				continue

			case ifaces.Accessor:

				m.addCoinFromAccessor(meta)
				continue
			}
		}
	}
}

func (m *ModuleLPP) addCoinFromAccessor(acce ifaces.Accessor) {

	switch a := acce.(type) {

	case *accessors.FromExprAccessor:

		m.addCoinFromExpression(a.Expr)
		return

	case *accessors.FromCoinAccessor:

		m.InsertCoin(a.Info.Name, a.Info.Round)
		return
	}
}

func (a AssignLPPQueries) Run(run *wizard.ProverRuntime) {

	moduleWitness := run.State.MustGet(moduleWitnessKey).(*ModuleWitnessLPP)
	run.State.Del(moduleWitnessKey)

	logDerivativeArgs, grandProductArgs, hornerArgs := a.DefinitionInput.LogDerivativeArgs,
		a.DefinitionInput.GrandProductArgs, a.DefinitionInput.HornerArgs

	if len(hornerArgs) > 0 {
		hornerParams := a.getHornerParams(run, moduleWitness.N0Values)
		run.AssignHornerParams(a.Horner.ID, hornerParams)
		n0Hash, n1Hash := hashNxs(hornerParams, 0), hashNxs(hornerParams, 1)

		run.AssignColumn(a.N0Hash.GetColID(), smartvectors.NewRegular([]field.Element{n0Hash}))
		run.AssignColumn(a.N1Hash.GetColID(), smartvectors.NewRegular([]field.Element{n1Hash}))
	}

	if len(grandProductArgs) > 0 {
		run.AssignGrandProduct(a.GrandProduct.ID, a.GrandProduct.Compute(run))
	}

	if len(logDerivativeArgs) > 0 {

		y, err := a.LogDerivativeSum.Compute(run)
		if err != nil {
			utils.Panic("LogDerivative has a zero term in the denominator: %v", err)
		}

		run.AssignLogDerivSum(a.LogDerivativeSum.ID, y)
	}

	a.ModuleLPP.assignMultiSetHash(run)
}

func (m ModuleLPP) getHornerParams(run *wizard.ProverRuntime, n0Values []int) query.HornerParams {

	hornerParams := query.HornerParams{}
	for i := range n0Values {
		hornerParams.Parts = append(hornerParams.Parts, query.HornerParamsPart{
			N0: n0Values[i],
		})
	}

	hornerParams.SetResult(run, *m.Horner)
	return hornerParams
}

func (a *CheckNxHash) Run(run wizard.Runtime) error {

	if a.Horner != nil {

		var (
			hornerParams  = run.GetHornerParams(a.Horner.ID)
			n0HashAlleged = a.N0Hash.GetColAssignmentAt(run, 0)
			n1HashAlleged = a.N1Hash.GetColAssignmentAt(run, 0)
			n0Hash        = hashNxs(hornerParams, 0)
			n1Hash        = hashNxs(hornerParams, 1)
		)

		if n1HashAlleged != n1Hash {
			return fmt.Errorf("n0Hash %v != n1HashAlleged %v", n1Hash, n1HashAlleged)
		}

		if n0HashAlleged != n0Hash {
			return fmt.Errorf("n0Hash %v != n0HashAlleged %v", n0Hash, n0HashAlleged)
		}
	}

	a.checkMultiSetHash(run)

	return nil
}

func (a *CheckNxHash) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	if a.Horner != nil {

		var (
			hornerParams  = run.GetHornerParams(a.Horner.ID)
			n0HashAlleged = a.N0Hash.GetColAssignmentGnarkAt(run, 0)
			n1HashAlleged = a.N1Hash.GetColAssignmentGnarkAt(run, 0)
			n0Hash        = hashNxsGnark(run.GetHasherFactory(), hornerParams, 0)
			n1Hash        = hashNxsGnark(run.GetHasherFactory(), hornerParams, 1)
		)

		api.AssertIsEqual(n0Hash, n0HashAlleged)
		api.AssertIsEqual(n1Hash, n1HashAlleged)
	}

	a.checkGnarkMultiSetHash(api, run)
}

func (a *CheckNxHash) Skip() {
	a.skipped = true
}

func (a *CheckNxHash) IsSkipped() bool {
	return a.skipped
}

func (a *SetInitialFSHash) Run(run wizard.Runtime) error {
	state := a.InitialFiatShamirState.GetColAssignment(run).Get(0)
	run.Fs().SetState([]field.Element{state})
	return nil
}

func (a *SetInitialFSHash) RunGnark(api frontend.API, run wizard.GnarkRuntime) {
	state := a.InitialFiatShamirState.GetColAssignmentGnark(run)[0]
	run.Fs().SetState([]frontend.Variable{state})
}

func (a *SetInitialFSHash) Skip() {
	a.skipped = true
}

func (a *SetInitialFSHash) IsSkipped() bool {
	return a.skipped
}

// hashNxs scans params and hash either the N0s or the N1s value all together
// (pass x=0, to compute the hash of the N0s and x=1 for the N1s).
func hashNxs(params query.HornerParams, x int) field.Element {

	hsh := mimc.NewMiMC()

	for _, part := range params.Parts {

		nx := 0

		if x == 0 {
			nx = part.N0
		} else {
			nx = part.N1
		}

		nxField := field.NewElement(uint64(nx))
		nxBytes := nxField.Bytes()
		hsh.Write(nxBytes[:])
	}

	resBytes := hsh.Sum(nil)
	var res field.Element
	res.SetBytes(resBytes)

	return res
}

// hashNxsGnark is as [hashNxs] but in a gnark circuit
func hashNxsGnark(factory mimc.HasherFactory, params query.GnarkHornerParams, x int) frontend.Variable {

	hsh := factory.NewHasher()

	for _, part := range params.Parts {

		var nx frontend.Variable

		if x == 0 {
			nx = part.N0
		} else {
			nx = part.N1
		}

		hsh.Write(nx)
	}

	return hsh.Sum()
}

func (modLPP *ModuleLPP) declarePublicInput() {

	var (
		nbModules = len(modLPP.DefinitionInput.Disc.Modules)
		// segmentCountGl is an array of zero.
		segmentCountGl = make([]field.Element, nbModules)
		// segmenCountLpp is an array of zero with a one at the position
		// corresponding to the current module.
		segmentCountLpp = make([]field.Element, nbModules)
		defInp          = modLPP.DefinitionInput
	)

	modLPP.SegmentModuleIndex = modLPP.Wiop.InsertProof(0, "SEGMENT_MODULE_INDEX", 1)
	segmentCountLpp[modLPP.Disc.IndexOf(modLPP.DefinitionInput.ModuleName)] = field.One()

	modLPP.PublicInputs = LimitlessPublicInput[wizard.PublicInput]{
		VKeyMerkleRoot:      declarePiColumn(modLPP.Wiop, VerifyingKeyMerkleRootPublicInput),
		TargetNbSegments:    declareListOfPiColumns(modLPP.Wiop, 0, TargetNbSegmentPublicInputBase, nbModules),
		SegmentCountGL:      declareListOfConstantPi(modLPP.Wiop, SegmentCountGLPublicInputBase, segmentCountGl),
		SegmentCountLPP:     declareListOfConstantPi(modLPP.Wiop, SegmentCountLPPPublicInputBase, segmentCountLpp),
		GeneralMultiSetHash: declareListOfPiColumns(modLPP.Wiop, 1, GeneralMultiSetPublicInputBase, mimc.MSetHashSize),

		SharedRandomnessMultiSetHash: declareListOfConstantPi(
			modLPP.Wiop,
			SharedRandomnessMultiSetPublicInputBase,
			make([]field.Element, mimc.MSetHashSize),
		),

		SharedRandomness: modLPP.Wiop.InsertPublicInput(
			InitialRandomnessPublicInput,
			accessors.NewFromPublicColumn(modLPP.InitialFiatShamirState, 0),
		),
	}

	// These are the "dummy" public inputs that are only here so that the
	// moduleGL and moduleLPP have identical set of public inputs. The order
	// of declaration is also important. Namely, these needs to be declared before
	// the non-dummy ones.
	for _, pi := range defInp.PublicInputs {
		modLPP.Wiop.InsertPublicInput(pi.Name, accessors.NewConstant(field.Zero()))
	}

	// This section adds the public inputs for the log-derivative, grand-product
	// horner-sum.
	if modLPP.Horner != nil {
		modLPP.PublicInputs.HornerSum = modLPP.Wiop.InsertPublicInput(
			HornerPublicInput,
			accessors.NewFromHornerAccessorFinalValue(modLPP.Horner),
		)
	} else {
		modLPP.PublicInputs.HornerSum = modLPP.Wiop.InsertPublicInput(
			HornerPublicInput,
			accessors.NewConstant(field.Zero()),
		)
	}

	if modLPP.LogDerivativeSum != nil {
		modLPP.PublicInputs.LogDerivativeSum = modLPP.Wiop.InsertPublicInput(
			LogDerivativeSumPublicInput,
			accessors.NewLogDerivSumAccessor(*modLPP.LogDerivativeSum),
		)
	} else {
		modLPP.PublicInputs.LogDerivativeSum = modLPP.Wiop.InsertPublicInput(
			LogDerivativeSumPublicInput,
			accessors.NewConstant(field.Zero()),
		)
	}

	if modLPP.GrandProduct != nil {
		modLPP.PublicInputs.GrandProduct = modLPP.Wiop.InsertPublicInput(
			GrandProductPublicInput,
			accessors.NewGrandProductAccessor(*modLPP.GrandProduct),
		)
	} else {
		modLPP.PublicInputs.GrandProduct = modLPP.Wiop.InsertPublicInput(
			GrandProductPublicInput,
			accessors.NewConstant(field.One()),
		)
	}
}

func (modLPP *ModuleLPP) assignPublicInput(run *wizard.ProverRuntime, witness *ModuleWitnessLPP) {

	// This assigns the segment module index proof column
	run.AssignColumn(
		modLPP.SegmentModuleIndex.GetColID(),
		smartvectors.NewConstant(field.NewElement(uint64(witness.SegmentModuleIndex)), 1),
	)

	// This assigns the VKeyMerkleRoot
	assignPiColumn(run, modLPP.PublicInputs.VKeyMerkleRoot.Name, witness.VkMerkleRoot)

	// This assigns the columns corresponding to the public input indicating
	// the number of segments
	assignListOfPiColumns(run, TargetNbSegmentPublicInputBase, vector.ForTest(witness.TotalSegmentCount...))
}

// assignLPPCommitmentMSetGL assigns the LPP commitment MSet. It is meant to be
// run as part of a prover action.
func (modLPP *ModuleLPP) assignMultiSetHash(run *wizard.ProverRuntime) {

	var lppCommitments field.Element
	if run.HasPublicInput(lppMerkleRootPublicInput + "_0") {
		lppCommitments = run.GetPublicInput(lppMerkleRootPublicInput + "_0")
	}

	var (
		segmentIndex           = modLPP.SegmentModuleIndex.GetColAssignmentAt(run, 0)
		typeOfProof            = field.NewElement(uint64(proofTypeLPP))
		hasHorner              = modLPP.Horner != nil
		mset                   = mimc.MSetHash{}
		defInp                 = modLPP.DefinitionInput
		moduleIndex            = field.NewElement(uint64(defInp.ModuleIndex))
		segmentIndexInt        = segmentIndex.Uint64()
		numSegmentOfCurrModule = modLPP.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetVal(run)
	)

	mset.Remove(moduleIndex, segmentIndex, lppCommitments)

	// If the segment is not the last one of its module we add the "sent" value
	// in the multiset.
	if hasHorner && segmentIndexInt < numSegmentOfCurrModule.Uint64()-1 {
		n1Hash := modLPP.N1Hash.GetColAssignmentAt(run, 0)
		mset.Insert(moduleIndex, segmentIndex, typeOfProof, n1Hash)
	}

	// If the segment is not the first one of its module, we add the received
	// value in the multiset
	if hasHorner && !segmentIndex.IsZero() {

		var (
			prevSegmentIndex field.Element
			one              = field.One()
			n0Hash           = modLPP.N0Hash.GetColAssignmentAt(run, 0)
		)

		prevSegmentIndex.Sub(&segmentIndex, &one)
		mset.Remove(moduleIndex, prevSegmentIndex, typeOfProof, n0Hash)
	}

	assignListOfPiColumns(run, GeneralMultiSetPublicInputBase, mset[:])
}

// checkLPPCommitmentMSetGL checks that the LPP commitment MSet is correctly
// assigned. It is meant to be run as part of a verifier action.
func (modLPP *ModuleLPP) checkMultiSetHash(run wizard.Runtime) error {

	var (
		targetMSet             = GetPublicInputList(run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
		lppCommitments         = run.GetPublicInput(lppMerkleRootPublicInput + "_0")
		segmentIndex           = modLPP.SegmentModuleIndex.GetColAssignmentAt(run, 0)
		typeOfProof            = field.NewElement(uint64(proofTypeLPP))
		hasHorner              = modLPP.Horner != nil
		mset                   = mimc.MSetHash{}
		defInp                 = modLPP.DefinitionInput
		moduleIndex            = field.NewElement(uint64(defInp.ModuleIndex))
		segmentIndexInt        = segmentIndex.Uint64()
		numSegmentOfCurrModule = modLPP.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetVal(run)
	)

	mset.Remove(moduleIndex, segmentIndex, lppCommitments)

	// If the segment is not the last one of its module we add the "sent" value
	// in the multiset.
	if hasHorner && segmentIndexInt < numSegmentOfCurrModule.Uint64()-1 {
		n1Hash := modLPP.N1Hash.GetColAssignmentAt(run, 0)
		// This is a local module
		mset.Insert(moduleIndex, segmentIndex, typeOfProof, n1Hash)
	}

	// If the segment is not the first one of its module, we add the received
	// value in the multiset
	if hasHorner && !segmentIndex.IsZero() {

		var (
			prevSegmentIndex field.Element
			one              = field.One()
			n0Hash           = modLPP.N0Hash.GetColAssignmentAt(run, 0)
		)

		prevSegmentIndex.Sub(&segmentIndex, &one)
		mset.Remove(moduleIndex, prevSegmentIndex, typeOfProof, n0Hash)
	}

	if !vector.Equal(targetMSet, mset[:]) {
		return fmt.Errorf("LPP commitment MSet mismatch, expected: %v, got: %v", targetMSet, mset[:])
	}

	return nil
}

// checkGnarkMultiSetHash checks that the commitment MSet and the randomness MSet
// are correctly set. It is meant to be run as part of a verifier action..
func (modLPP *ModuleLPP) checkGnarkMultiSetHash(api frontend.API, run wizard.GnarkRuntime) error {

	var (
		targetMSetGeneral      = GetPublicInputListGnark(api, run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
		lppCommitments         = run.GetPublicInput(api, lppMerkleRootPublicInput+"_0")
		segmentIndex           = modLPP.SegmentModuleIndex.GetColAssignmentGnarkAt(run, 0)
		typeOfProof            = field.NewElement(uint64(proofTypeLPP))
		hasHorner              = modLPP.Horner != nil
		hasher                 = run.GetHasherFactory().NewHasher()
		multiSetGeneral        = mimc.EmptyMSetHashGnark(hasher)
		defInp                 = modLPP.DefinitionInput
		moduleIndex            = field.NewElement(uint64(defInp.ModuleIndex))
		numSegmentOfCurrModule = modLPP.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetFrontendVariable(api, run)
		isFirst                = api.IsZero(segmentIndex)
		isLast                 = api.IsZero(api.Sub(numSegmentOfCurrModule, segmentIndex, 1))
	)

	multiSetGeneral.Remove(api, moduleIndex, segmentIndex, lppCommitments)

	if hasHorner {

		// If the segment is not the last one, we can add the "n1 hash" to the
		// multiset.
		n1Hash := modLPP.N1Hash.GetColAssignmentGnarkAt(run, 0)
		n1HashSingletonMsetHash := mimc.MsetOfSingletonGnark(api, hasher, moduleIndex, segmentIndex, typeOfProof, n1Hash)
		for i := 0; i < mimc.MSetHashSize; i++ {
			n1HashSingletonMsetHash.Inner[i] = api.Mul(n1HashSingletonMsetHash.Inner[i], api.Sub(1, isLast))
			multiSetGeneral.Inner[i] = api.Add(multiSetGeneral.Inner[i], n1HashSingletonMsetHash.Inner[i])
		}

		// If the segment is not the first one, we can remove the "n0 hash" from the
		// multiset.
		n0Hash := modLPP.N0Hash.GetColAssignmentGnarkAt(run, 0)
		n0HashSingletonMsetHash := mimc.MsetOfSingletonGnark(api, hasher, moduleIndex, api.Sub(segmentIndex, 1), typeOfProof, n0Hash)
		for i := 0; i < mimc.MSetHashSize; i++ {
			n0HashSingletonMsetHash.Inner[i] = api.Mul(n0HashSingletonMsetHash.Inner[i], api.Sub(1, isFirst))
			multiSetGeneral.Inner[i] = api.Sub(multiSetGeneral.Inner[i], n0HashSingletonMsetHash.Inner[i])
		}
	}

	for i := range multiSetGeneral.Inner {
		api.AssertIsEqual(multiSetGeneral.Inner[i], targetMSetGeneral[i])
	}

	return nil
}
