package distributed

import (
	"fmt"
	"strconv"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// ModuleGL is a compilation structure holding the central informations
// of the GL part of a module.
type ModuleGL struct {

	// ModuleTranslator is the translator for the GL part of the module
	// it also has the ownership of the [wizard.Compiled] IOP built for
	// this module.
	ModuleTranslator

	// DefinitionInput stores the [FilteredModuleInputs] that was used
	// to generate the module.
	DefinitionInput *FilteredModuleInputs

	// SegmentModuleIndex is the index of the module in the segment. The value
	// is used to assign a column and check that "isFirst" and "isLast" are
	// right-fully computed.
	SegmentModuleIndex ifaces.Column

	// IsFirst is a column of length one storing a binary value indicating
	// if the current (vertical) instance of the module is the first one.
	// This is used to activate/deactivate the local-constraints relating
	// to the first rows of the module. The content of the column is part
	// of the public inputs of the module.
	IsFirst ifaces.Column

	// IsLast is as [IsFirst] but for the last instance of the module.
	IsLast ifaces.Column

	// SentValuesGlobal is a list of local-openings pointing to the values
	// nearing the end of the segment that are subjected to global-constraints
	// connecting them to the next segment. The column is structured as an
	// heterogenous list of values followed by zeroes. If the current segment
	// corresponds to the last segment, it is normally computed.
	SentValuesGlobal []query.LocalOpening

	// SentValuesGlobalHash is the hash of the [SentValuesGlobal]. It is
	// computed and checked by a verifier action. It also one of the
	// public inputs of the module.
	SentValuesGlobalHash ifaces.Column

	// SentValuesGlobalMap maps string keys to a local-opening position to
	// access the corresponding value in [SentValuesGlobal]. This is handy
	// to check if a value has already been added to [SentValuesGlobal]. The
	// key string are formatted as "<column-name>/<position>".
	SentValuesGlobalMap map[string]int

	// ReceivedValuesGlobal is a column storing the values of the previous
	// segment that are needed to fill the missing values of the current
	// segment in global-constraints toward the beginning of the segment.
	// The column is structured as an heterogenous list of values followed
	// by zeroes. If the current segment corresponds to the first segment
	// it is assigned to zeroes (the constraint is disabled in this case).
	//
	// The column has the [column.Proof] status.
	ReceivedValuesGlobal ifaces.Column

	// ReceivedValuesGlobalAccs the list of the accessors to [ReceivedValuesGlobal]
	// which are used to compute the
	ReceivedValuesGlobalAccs []ifaces.Accessor

	// ReceivedValuesGlobalHash is the hash of the [ReceivedValuesGlobal]. It is
	// computed and checked by a verifier action. It is also one of the public
	// inputs of the modules.
	ReceivedValuesGlobalHash ifaces.Column

	// ReceivedValuesGlobalMap maps string keys to a local-opening position to
	// access the corresponding value in [SentValuesGlobal]. This is handy
	// to check if a value has already been added to [SentValuesGlobal]. The
	// key string are formatted as "<column-name>/<position>".
	ReceivedValuesGlobalMap map[string]int

	// PublicInputs contains the public inputs of the module.
	PublicInputs LimitlessPublicInput[wizard.PublicInput]

	// ExplicitlyVerifiedGlobalCsCompletion is a list of expressions containing
	// no columns (and thus can't be used to generate local-constraints) whose
	// cancellation are checked by a verifier action.
	ExplicitlyVerifiedGlobalCsCompletion []*sym.Expression
}

// ModuleGLAssignSendReceiveGlobal is an implementation of the [wizard.ProverRuntime]
// which is responsible for assigning the [SentValuesGlobalHash], the [ReceivedValuesGlobal]
// and the [ReceivedValuesGlobalHash] columns.
type ModuleGLAssignSendReceiveGlobal struct {
	*ModuleGL
}

// ModuleGLCheckSendReceiveGlobal is an implementation of the [wizard.VerifierAction]
// which is responsible for checking that the [SentValuesGlobalHash] and the
// [ReceivedValuesGlobalHash] are correctly computed.
type ModuleGLCheckSendReceiveGlobal struct {
	*ModuleGL
	skipped bool `serde:"omit"`
}

// ModuleGLAssignGL is a [wizard.ProverAction] responsible for assigning the
// "GL" columns which is happening at round 1. The function also assigns the
// all the [query.LocalOpening] (for GL and LPP columns).
type ModuleGLAssignGL struct {
	*ModuleGL
}

// BuildModuleGL builds a [ModuleGL] from scratch from a [FilteredModuleInputs].
// The function works by creating a define function that will call [NewModuleGL]
// / and then it calls [wizard.Compile] without passing compilers.
func BuildModuleGL(moduleInput *FilteredModuleInputs) *ModuleGL {

	var (
		moduleGL   *ModuleGL
		defineFunc = func(b *wizard.Builder) {
			moduleGL = NewModuleGL(b, moduleInput)
		}
		// Since the NewModuleGL contains a pointer to the compiled IOP already
		// there is no need to use the one returned by [wizard.Compile].
		_ = wizard.Compile(defineFunc)
	)

	return moduleGL
}

// NewModuleGL declares and constructs a new ModuleGL from a [wizard.Builder]
// and a [FilteredModuleInput]. The function performs all the necessary
// declarations to build the GL part of the module and returns the constructed
// module.
func NewModuleGL(builder *wizard.Builder, moduleInput *FilteredModuleInputs) *ModuleGL {

	moduleGL := &ModuleGL{
		ModuleTranslator: ModuleTranslator{
			Wiop: builder.CompiledIOP,
			Disc: moduleInput.Disc,
		},
		DefinitionInput:         moduleInput,
		SentValuesGlobalMap:     map[string]int{},
		ReceivedValuesGlobalMap: map[string]int{},
	}

	for _, col := range moduleInput.Columns {

		if col.Round() != 0 {
			utils.Panic("cannot translate a column with non-zero round %v", col.Round())
		}

		var (
			_, isLPP               = moduleInput.ColumnsLPPSet[col.GetColID()]
			newRound               = 1
			precompData, isPrecomp = moduleInput.ColumnsPrecomputed[col.GetColID()]
		)

		if isLPP {
			newRound = 0
		}

		if isPrecomp {
			moduleGL.InsertPrecomputed(*col, precompData)
			continue
		}

		moduleGL.InsertColumn(*col, newRound)
	}

	// As the columns of the GL and the LPP modules are split between two round although
	// there is no random coins in the GL module, we need to add at least one dummy coin
	// otherwise the compiler will throw an error stating that we have several rounds for
	// the columns and the queries but not for the coins.
	_ = moduleGL.Wiop.InsertCoin(1, "DUMMY_GL_COIN", coin.Field)

	moduleGL.IsFirst = moduleGL.Wiop.InsertProof(0, "IS_FIRST", 1)
	moduleGL.IsLast = moduleGL.Wiop.InsertProof(0, "IS_LAST", 1)

	for _, globalCs := range moduleInput.GlobalConstraints {
		moduleGL.InsertGlobal(*globalCs)
	}

	for _, localCs := range moduleInput.LocalConstraints {
		moduleGL.InsertLocal(*localCs)
	}

	for _, rangeCs := range moduleInput.Range {
		newCol := moduleGL.TranslateColumn(rangeCs.Handle)
		moduleGL.Wiop.InsertRange(1, rangeCs.ID, newCol, rangeCs.B)
	}

	for _, localOpening := range moduleInput.LocalOpenings {
		newCol := moduleGL.TranslateColumn(localOpening.Pol)
		moduleGL.Wiop.InsertLocalOpening(1, localOpening.ID, newCol)
	}

	for _, piw := range moduleInput.PlonkInWizard {
		moduleGL.InsertPlonkInWizard(piw)
	}

	moduleGL.processSendAndReceiveGlobal()

	for _, globalCs := range moduleInput.GlobalConstraints {
		// Although we iterate on the original global-constraints, we
		// want to be sure to pass the translated constraint to process
		// the send-receive queries.
		newGlobalCs := moduleGL.Wiop.QueriesNoParams.Data(globalCs.ID).(query.GlobalConstraint)
		moduleGL.CompleteGlobalCs(newGlobalCs)
	}

	moduleGL.declarePublicInput()

	moduleGL.Wiop.RegisterProverAction(1, &ModuleGLAssignGL{ModuleGL: moduleGL})
	moduleGL.Wiop.RegisterProverAction(1, &ModuleGLAssignSendReceiveGlobal{ModuleGL: moduleGL})
	moduleGL.Wiop.RegisterVerifierAction(1, &ModuleGLCheckSendReceiveGlobal{ModuleGL: moduleGL})

	return moduleGL
}

// GetMainProverStep returns a [wizard.ProverStep] running [Assign] passing
// the provided [ModuleWitness] argument.
func (m *ModuleGL) GetMainProverStep(witness *ModuleWitnessGL) wizard.MainProverStep {
	return func(run *wizard.ProverRuntime) {
		m.Assign(run, witness)
	}
}

// Assign is the entry-point for the assignment of [ModuleGL]. It
// is responsible for setting up the [ProverRuntime.State] with the witness
// value and assigning the following columns.
//
//   - The LPP columns
//   - The IsFirst, IsLast columns
//
// But not the local-openings for the local-constraints.
//
// The function depopulates the [ModuleWitness] from its columns assignment
// as the columns are assigned in the runtime.
func (m *ModuleGL) Assign(run *wizard.ProverRuntime, witness *ModuleWitnessGL) {

	var (
		// columns stores the list of columns to assign. Though, it
		// stores the columns as in the origin CompiledIOP so we cannot
		// directly use them to refer to columns of the current IOP.
		// Yet, the column share the same names.
		columns = m.DefinitionInput.Columns
	)

	run.State.InsertNew(moduleWitnessKey, witness)

	for _, col := range columns {

		colName := col.GetColID()

		if _, ok := m.DefinitionInput.ColumnsLPPSet[colName]; !ok {
			continue
		}

		newCol := m.Wiop.Columns.GetHandle(colName)

		if m.Wiop.Precomputed.Exists(newCol.GetColID()) {
			continue
		}

		if newCol.Round() != 0 {
			utils.Panic("expected a column with round 0, got %v, column: %v", newCol.Round(), colName)
		}

		colWitness, ok := witness.Columns[colName]
		if !ok {
			utils.Panic("witness of column %v was not found", colName)
		}

		run.AssignColumn(colName, colWitness)
		delete(witness.Columns, colName)
	}

	isFirst, isLast := field.Element{}, field.Element{}

	if witness.SegmentModuleIndex == 0 {
		isFirst = field.One()
	}

	if witness.SegmentModuleIndex == witness.TotalSegmentCount[witness.ModuleIndex]-1 {
		isLast = field.One()
	}

	run.AssignColumn(m.IsFirst.GetColID(), smartvectors.NewConstant(isFirst, 1))
	run.AssignColumn(m.IsLast.GetColID(), smartvectors.NewConstant(isLast, 1))

	m.assignPublicInput(run, witness)
}

// InsertGlobal inserts a global constraint in the target compiled IOP and
// performs all the necessary compilation steps needed for its security. The
// translated constraint may have a different round than the original one,
// (0 if the original touches only GL columns but 1 if there are LPP columns).
//
// The expression is first translated and then, the optimal round number is
// deduced from looking at the translated expression. After that, the function
// analyzes the offsets of the translated expression for each of its column
// variable. From the analysis, it infers which positions of which columns
// need to be "sent" and/or "received". However, it does not perform the
// filling of the missing rows of the global-constraint.
func (m *ModuleGL) InsertGlobal(q query.GlobalConstraint) query.GlobalConstraint {

	if q.Name() == "CYCLIC_COUNTER_4514_24_COUNTER_IS_ZERO_WHEN_INACTIVE" {
		board := q.Expression.Board()
		meta := board.ListVariableMetadata()
		fmt.Printf("metadata = %v\n", meta)
	}

	var (
		newExpr      = m.TranslateExpression(q.Expression)
		newExprRound = wizardutils.LastRoundToEval(newExpr)
		newGlobal    = m.Wiop.InsertGlobal(newExprRound, q.ID, newExpr)
		offsetRange  = query.MinMaxOffset(newGlobal.Expression)
		columnOfExpr = column.ColumnsOfExpression(newExpr)
	)

	if offsetRange.Min == 0 && offsetRange.Max == 0 {
		return newGlobal
	}

	for _, col := range columnOfExpr {

		var (
			colOffset = column.StackOffsets(col)
			rootCol   = column.RootParents(col)
		)

		// If the column is a [verifiercol.ConstCol], then there is no need to
		// send any missing value.
		if _, isVCol := rootCol.(verifiercol.ConstCol); isVCol {
			continue
		}

		for i := colOffset; i < offsetRange.Max; i++ {

			posToRcv := i - offsetRange.Max
			posToSend := posToRcv + col.Size()

			m.requestReceptionGlobal(rootCol, posToRcv)
			m.sendValueGlobal(rootCol, posToSend)
		}
	}

	return newGlobal
}

// InsertLocal inserts a local constraint in the target compiled IOP and
// performs all the necessary compilation steps needed for its security. The
// translated constraint may have a different round than the original one,
// (0 if the original touches only GL columns but 1 if there are LPP columns).
// However, it retains the name of the original constraint.
//
// Additionally, the offsets of the constraints are analyzed and the constraint
// is activated/deactived depending on the offset and the values of
// [IsFirst] or [IsLast].
//
// If the local constraint affects both the beginning and the end of a column,
// the function will panic.
func (m *ModuleGL) InsertLocal(q query.LocalConstraint) query.LocalConstraint {

	var (
		newExpr      = m.TranslateExpression(q.Expression)
		newExprRound = wizardutils.LastRoundToEval(newExpr)
		offsetRange  = query.MinMaxOffset(newExpr)
	)

	// Note: if we remove this check. The constraint will only be enforced when
	// the module as a single vertical segment. Meaning the system becomes unsound
	// without this check. If this comes to fail, the solution will be to
	// add an extra constraint to ensure that IsFirst * IsLast == 1. This is
	// equivalent to enforcing that the module has a single vertical segment.
	//
	// But doing that, would harm the "limitless" feature so it should be avoided
	// at all costs.
	if offsetRange.Min < 0 && offsetRange.Max >= 0 {
		utils.Panic("local constraint has both negative and positive offsets, min=%v max=%v name=%v", offsetRange.Min, offsetRange.Max, q.ID)
	}

	if offsetRange.Min < 0 {
		newExpr = sym.Mul(newExpr, accessors.NewFromPublicColumn(m.IsLast, 0))
	}

	if offsetRange.Max >= 0 {
		newExpr = sym.Mul(newExpr, accessors.NewFromPublicColumn(m.IsFirst, 0))
	}

	return m.Wiop.InsertLocal(newExprRound, q.ID, newExpr)
}

// CompleteGlobalCs completes a translated global-constraint by filling
// the missing rows of the global-constraint using local-constraints
// involving the [ReceivedValueGlobal]. The function expects the new
// translated global constraints and not the original one.
func (m *ModuleGL) CompleteGlobalCs(newGlobal query.GlobalConstraint) {

	var (
		newExpr            = newGlobal.Expression
		newExprRound       = wizardutils.LastRoundToEval(newExpr)
		offsetRange        = query.MinMaxOffset(newExpr)
		firstRowToComplete = min(-offsetRange.Max, 0)
		lastRowToComplete  = max(-offsetRange.Min, 0)
	)

	for row := firstRowToComplete; row < lastRowToComplete; row++ {

		// The function is looking for variables of type [ifaces.Column]
		// and replacing them with either shifted version of the column
		// or with an accessor to "received" value.
		localExpr := newExpr.ReconstructBottomUp(

			func(e *sym.Expression, children []*sym.Expression) (new *sym.Expression) {

				v, isVar := e.Operator.(sym.Variable)
				if !isVar {
					return e.SameWithNewChildren(children)
				}

				col, isCol := v.Metadata.(ifaces.Column)
				if !isCol {
					return e
				}

				var (
					colOffset = column.StackOffsets(col)
					shfPos    = row + colOffset
					rootCol   = column.RootParents(col)
				)

				if cnst, isConst := rootCol.(verifiercol.ConstCol); isConst {
					return sym.NewConstant(cnst.F)
				}

				if _, isVCol := rootCol.(verifiercol.VerifierCol); isVCol {
					utils.Panic("unexpected type of column: %T", col)
				}

				if shfPos < 0 {
					rcvValue := m.getReceivedValueGlobal(rootCol, shfPos)
					return sym.NewVariable(rcvValue)
				}

				shfCol := column.Shift(rootCol, shfPos)
				return sym.NewVariable(shfCol)
			},
		)

		// This check that expression actually contains columns (it might not)
		// and if it does, it will tell verifier to explicitly check the
		// expression but will not register a local constraint.
		if cols := column.ColumnsOfExpression(localExpr); len(cols) == 0 {
			m.ExplicitlyVerifiedGlobalCsCompletion = append(m.ExplicitlyVerifiedGlobalCsCompletion, localExpr)
			continue
		}

		localExpr = sym.Mul(localExpr, sym.Sub(1, accessors.NewFromPublicColumn(m.IsFirst, 0)))
		m.Wiop.InsertLocal(
			newExprRound,
			ifaces.QueryID("COMPLETE_GLOBAL_CS_"+strconv.Itoa(row)+"_QUERY_"+string(newGlobal.ID)),
			localExpr,
		)
	}

}

// sendValueGlobal inserts a local-opening in the target compiled IOP and
// registers it in the [SentValuesGlobal] list. If the value is already
// registered, the function is a no-op and returns the already registered
// value. The function returns the resulting local-opening query.
func (m *ModuleGL) sendValueGlobal(col ifaces.Column, pos int) query.LocalOpening {

	name := string(col.GetColID()) + "/" + strconv.Itoa(pos)

	if pos, ok := m.SentValuesGlobalMap[name]; ok {
		return m.SentValuesGlobal[pos]
	}

	newLO := m.Wiop.InsertLocalOpening(
		1,
		ifaces.QueryIDf("SENT_VALUE_GLOBAL_%v", len(m.SentValuesGlobal)),
		column.Shift(col, pos),
	)

	// It is fine to skip the result of the newLO because we already
	// includes its hash in the FS.
	m.Wiop.QueriesParams.MarkAsSkippedFromProverTranscript(newLO.ID)

	m.SentValuesGlobalMap[name] = len(m.SentValuesGlobal)
	m.SentValuesGlobal = append(m.SentValuesGlobal, newLO)
	return newLO
}

// requestReceptionGlobal inserts an entry in the [ReceivedValuesGlobal] column
// if it is not already present. Unlike [sendValueGlobal], the function does
// not return anything.The entry will be taken into account to construct the
// [ModuleGL.ReceivedValuesGlobal]. Only then, it will be possible to construct
// the [ModuleGL.ReceivedValuesGlobalAccs].
func (m *ModuleGL) requestReceptionGlobal(col ifaces.Column, pos int) {
	name := string(col.GetColID()) + "/" + strconv.Itoa(pos)
	if _, ok := m.ReceivedValuesGlobalMap[name]; ok {
		return
	}
	m.ReceivedValuesGlobalMap[name] = len(m.ReceivedValuesGlobalMap)
}

// getReceivedValueGlobal returns the accessor corresponding to a received
// value in the [ModuleGL.ReceivedValuesGlobal] column. The column has to
// be instantiated before calling this function.
func (m *ModuleGL) getReceivedValueGlobal(col ifaces.Column, pos int) ifaces.Accessor {
	name := string(col.GetColID()) + "/" + strconv.Itoa(pos)
	accPos := m.ReceivedValuesGlobalMap[name]
	return m.ReceivedValuesGlobalAccs[accPos]
}

// processSendAndReceiveGlobal instantiates all the columns relevant to
// the "send and receive" mechanism of the global constraints. It declares
// the [SentValuesGlobalHash], the [ReceivedValuesGlobal] and the
// [ReceivedValuesGlobalHash] columns and populates the
// [ReceivedValuesGlobalAccs]. The function additionally records the prover
// and verifier action needed to assign these columns.
func (m *ModuleGL) processSendAndReceiveGlobal() {

	if len(m.ReceivedValuesGlobalMap) == 0 {
		return
	}

	// The columns are inserted at round 1 because we want it to store informations
	// about potentially either GL or LPP columns.
	m.SentValuesGlobalHash = m.Wiop.InsertProof(1, ifaces.ColID("SENT_VALUES_GLOBAL_HASH"), 1)
	m.ReceivedValuesGlobalHash = m.Wiop.InsertProof(1, ifaces.ColID("RECEIVED_VALUES_GLOBAL_HASH"), 1)

	m.ReceivedValuesGlobal = m.Wiop.InsertProof(
		1,
		ifaces.ColID("RECEIVED_VALUES_GLOBAL"),
		utils.NextPowerOfTwo(len(m.ReceivedValuesGlobalMap)),
	)

	m.Wiop.Columns.ExcludeFromProverFS(m.ReceivedValuesGlobal.GetColID())

	m.ReceivedValuesGlobalAccs = make([]ifaces.Accessor, len(m.ReceivedValuesGlobalMap))
	for i := range m.ReceivedValuesGlobalAccs {
		m.ReceivedValuesGlobalAccs[i] = accessors.NewFromPublicColumn(m.ReceivedValuesGlobal, i)
	}

	// Since, everything that is sent has to be received by the next
	// instance of the same module. We have that the number of elements
	// to be received is equal to the number of elements sent, giving
	// us the following sanity-check.
	if len(m.ReceivedValuesGlobalAccs) != len(m.SentValuesGlobal) {
		utils.Panic(
			"number of received values must be equal to the number of sent values: %v != %v",
			len(m.ReceivedValuesGlobalAccs), len(m.SentValuesGlobal),
		)
	}
}

// Run implements the [wizard.ProverAction] interface and assignes the
// send-receive columns. Since the send-receive columns are not part of
// the second round of the IOP, the function cannot directly fetch its
// inputs from a witness argument. To remediate the action fetches the
// input from a [ProverRuntime.State] item and the "receive" with the
// fetched values.
//
// The function also assigns the [SentValueGlobal] local openings.
func (a *ModuleGLAssignSendReceiveGlobal) Run(run *wizard.ProverRuntime) {

	if len(a.ReceivedValuesGlobalMap) > 0 {

		hashSend := field.Element{}

		for i := range a.SentValuesGlobal {
			lo := a.SentValuesGlobal[i]
			v := lo.Pol.GetColAssignmentAt(run, 0)
			run.AssignLocalPoint(lo.ID, v)
			hashSend = mimc.BlockCompression(hashSend, v)
		}

		run.AssignColumn(
			a.SentValuesGlobalHash.GetColID(),
			smartvectors.NewConstant(hashSend, 1),
		)

		witness := run.State.MustGet(moduleWitnessKey).(*ModuleWitnessGL)
		rcvData := witness.ReceivedValuesGlobal

		if len(rcvData) != len(a.ReceivedValuesGlobalAccs) {
			utils.Panic("len(rcvData: %v) != len(a.ReceivedValuesGlobalAccs: %v)", len(rcvData), len(a.ReceivedValuesGlobalAccs))
		}

		run.AssignColumn(
			a.ReceivedValuesGlobal.GetColID(),
			smartvectors.RightZeroPadded(rcvData, a.ReceivedValuesGlobal.Size()),
		)

		hashRcv := field.Element{}
		for i := range rcvData {
			v := rcvData[i]
			hashRcv = mimc.BlockCompression(hashRcv, v)
		}

		run.AssignColumn(
			a.ReceivedValuesGlobalHash.GetColID(),
			smartvectors.NewConstant(hashRcv, 1),
		)
	}

	a.ModuleGL.assignMultiSetHash(run)
}

// Run implements the [wizard.VerifierAction] interface and recomputes and
// checks the values of [ModuleGL.SentValuesGlobalHash] and
// [ModuleGL.ReceivedValuesGlobalHash].
func (a *ModuleGLCheckSendReceiveGlobal) Run(run wizard.Runtime) error {

	if len(a.ReceivedValuesGlobalMap) == 0 {
		return nil
	}

	var (
		sendGlobalHash   = a.SentValuesGlobalHash.GetColAssignmentAt(run, 0)
		hsh              = mimc.NewMiMC()
		hashSendComputed = field.Element{}
	)

	for i := range a.SentValuesGlobal {
		v := run.GetLocalPointEvalParams(a.SentValuesGlobal[i].ID)
		yBytes := v.Y.Bytes()
		hsh.Write(yBytes[:])
	}

	hashSendComputedBytes := hsh.Sum(nil)
	hashSendComputed.SetBytes(hashSendComputedBytes)

	if hashSendComputed != sendGlobalHash {
		return fmt.Errorf(
			"invalid hash send: %v != %v",
			hashSendComputed.Text(16), sendGlobalHash.Text(16),
		)
	}

	var (
		rcvGlobalHash   = a.ReceivedValuesGlobalHash.GetColAssignmentAt(run, 0)
		hashRcvComputed = field.Element{}
		rcvGlobalCol    = a.ReceivedValuesGlobal.GetColAssignment(run).IntoRegVecSaveAlloc()
		numReceived     = len(a.ReceivedValuesGlobalAccs)
	)

	hsh.Reset()

	for i := range rcvGlobalCol[:numReceived] {
		yBytes := rcvGlobalCol[i].Bytes()
		hsh.Write(yBytes[:])
	}

	hashRcvComputedBytes := hsh.Sum(nil)
	hashRcvComputed.SetBytes(hashRcvComputedBytes)

	if hashRcvComputed != rcvGlobalHash {
		return fmt.Errorf(
			"invalid hash rcv: %v != %v",
			hashRcvComputed.Text(16), rcvGlobalHash.Text(16),
		)
	}

	a.ModuleGL.checkMultiSetHash(run)

	for i := range a.ExplicitlyVerifiedGlobalCsCompletion {
		res := accessors.EvaluateExpression(run, a.ExplicitlyVerifiedGlobalCsCompletion[i])
		if !res.IsZero() {
			return fmt.Errorf("not zero: %v", res)
		}
	}

	return nil
}

// Run implements the [wizard.VerifierAction] interface and recomputes and
// checks the values of [ModuleGL.SentValuesGlobalHash] and
// [ModuleGL.ReceivedValuesGlobalHash].
func (a *ModuleGLCheckSendReceiveGlobal) RunGnark(api frontend.API, run wizard.GnarkRuntime) {

	if len(a.ReceivedValuesGlobalMap) == 0 {
		return
	}

	var (
		sendGlobalHash = a.SentValuesGlobalHash.GetColAssignmentGnarkAt(run, 0)
		hsh            = run.GetHasherFactory().NewHasher()
	)

	for i := range a.SentValuesGlobal {
		v := run.GetLocalPointEvalParams(a.SentValuesGlobal[i].ID)
		hsh.Write(v.Y)
	}

	hashSendComputed := hsh.Sum()

	api.AssertIsEqual(hashSendComputed, sendGlobalHash)

	var (
		rcvGlobalHash = a.ReceivedValuesGlobalHash.GetColAssignmentGnarkAt(run, 0)
		rcvGlobalCol  = a.ReceivedValuesGlobal.GetColAssignmentGnark(run)
		numReceived   = len(a.ReceivedValuesGlobalAccs)
	)

	hsh.Reset()

	for i := range rcvGlobalCol[:numReceived] {
		hsh.Write(rcvGlobalCol[i])
	}

	api.AssertIsEqual(hsh.Sum(), rcvGlobalHash)

	for i := range a.ExplicitlyVerifiedGlobalCsCompletion {
		res := accessors.EvaluateExpressionGnark(api, run, a.ExplicitlyVerifiedGlobalCsCompletion[i])
		api.AssertIsEqual(res, 0)
	}

	a.ModuleGL.checkGnarkMultiSetHash(api, run)
}

func (a *ModuleGLCheckSendReceiveGlobal) Skip() {
	a.skipped = true
}

func (a *ModuleGLCheckSendReceiveGlobal) IsSkipped() bool {
	return a.skipped
}

func (a *ModuleGLAssignGL) Run(run *wizard.ProverRuntime) {

	var (
		witness = run.State.MustGet(moduleWitnessKey).(*ModuleWitnessGL)
		// columns stores the list of columns to assign. Though, it
		// stores the columns as in the origin CompiledIOP so we cannot
		// directly use them to refer to columns of the current IOP.
		// Yet, the column share the same names.
		columns = a.DefinitionInput.Columns
	)

	for _, col := range columns {

		colName := col.GetColID()

		if _, ok := a.DefinitionInput.ColumnsLPPSet[colName]; ok {
			// We can't assign LPP columns as they (normally) have already
			// been assigned at this point.
			continue
		}

		newCol := a.Wiop.Columns.GetHandle(colName)

		if a.Wiop.Precomputed.Exists(colName) {
			// The column has been registered as a precomputed column but is living at round 1
			continue
		}

		if newCol.Round() != 1 {
			utils.Panic("expected a column with round 1, got %v, column: %v", newCol.Round(), colName)
		}

		colWitness, ok := witness.Columns[colName]
		if !ok {
			utils.Panic("witness of column %v was not found", colName)
		}

		run.AssignColumn(colName, colWitness)
		delete(witness.Columns, colName)
	}

	for i := range a.DefinitionInput.LocalOpenings {
		newLo := run.GetLocalPointEval(a.DefinitionInput.LocalOpenings[i].ID)
		y := newLo.Pol.GetColAssignmentAt(run, 0)
		run.AssignLocalPoint(a.DefinitionInput.LocalOpenings[i].ID, y)
	}
}

func (modGl *ModuleGL) declarePublicInput() {

	var (
		nbModules = len(modGl.DefinitionInput.Disc.Modules)
		// segmenCountGL is an array of zero with a one at the position
		// corresponding to the current module.
		segmentCountGl = make([]field.Element, nbModules)
		// segmentCountLpp is an array of zero.
		segmentCountLpp = make([]field.Element, nbModules)
		defInp          = modGl.DefinitionInput
	)

	modGl.SegmentModuleIndex = modGl.Wiop.InsertProof(0, "SEGMENT_MODULE_INDEX", 1)

	segmentCountGl[modGl.Disc.IndexOf(modGl.DefinitionInput.ModuleName)] = field.One()

	modGl.PublicInputs = LimitlessPublicInput[wizard.PublicInput]{
		VKeyMerkleRoot:               declarePiColumn(modGl.Wiop, VerifyingKeyMerkleRootPublicInput),
		TargetNbSegments:             declareListOfPiColumns(modGl.Wiop, 0, TargetNbSegmentPublicInputBase, nbModules),
		SegmentCountGL:               declareListOfConstantPi(modGl.Wiop, SegmentCountGLPublicInputBase, segmentCountGl),
		SegmentCountLPP:              declareListOfConstantPi(modGl.Wiop, SegmentCountLPPPublicInputBase, segmentCountLpp),
		GeneralMultiSetHash:          declareListOfPiColumns(modGl.Wiop, 1, GeneralMultiSetPublicInputBase, mimc.MSetHashSize),
		SharedRandomnessMultiSetHash: declareListOfPiColumns(modGl.Wiop, 1, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize),
		SharedRandomness:             modGl.Wiop.InsertPublicInput(InitialRandomnessPublicInput, accessors.NewConstant(field.Zero())),
	}

	// This adds the functional inputs by multiplying them with the value of
	// isFirst.
	for i := range defInp.PublicInputs {

		pubInputAcc := accessors.NewConstant(field.Zero())

		if defInp.PublicInputs[i].Acc != nil {
			pubInputAcc = modGl.TranslateAccessor(defInp.PublicInputs[i].Acc)
			pubInputAcc = accessors.NewFromExpression(sym.Mul(
				pubInputAcc,
				accessors.NewFromPublicColumn(modGl.IsFirst, 0),
			), "IS_FIRST_MULT_"+defInp.PublicInputs[i].Name)
		}

		modGl.Wiop.InsertPublicInput(
			defInp.PublicInputs[i].Name,
			pubInputAcc,
		)
	}

	// This section adds the dummy public inputs for the log-derivative, grand-product
	// horner-sum.

	modGl.PublicInputs.HornerSum = modGl.Wiop.InsertPublicInput(
		HornerPublicInput,
		accessors.NewConstant(field.Zero()),
	)

	modGl.PublicInputs.LogDerivativeSum = modGl.Wiop.InsertPublicInput(
		LogDerivativeSumPublicInput,
		accessors.NewConstant(field.Zero()),
	)

	modGl.PublicInputs.GrandProduct = modGl.Wiop.InsertPublicInput(
		GrandProductPublicInput,
		accessors.NewConstant(field.One()),
	)

}

func (modGL *ModuleGL) assignPublicInput(run *wizard.ProverRuntime, witness *ModuleWitnessGL) {

	// This assigns the segment module index proof column
	run.AssignColumn(
		modGL.SegmentModuleIndex.GetColID(),
		smartvectors.NewConstant(field.NewElement(uint64(witness.SegmentModuleIndex)), 1),
	)

	// This assigns the VKeyMerkleRoot
	assignPiColumn(run, modGL.PublicInputs.VKeyMerkleRoot.Name, witness.VkMerkleRoot)

	// This assigns the columns corresponding to the public input indicating
	// the number of segments
	assignListOfPiColumns(run, TargetNbSegmentPublicInputBase, vector.ForTest(witness.TotalSegmentCount...))
}

// assignLPPCommitmentMSetGL assigns the LPP commitment MSet. It is meant to be
// run as part of a prover action. It also adds the "sent" and "received" values
// to the MSet.
func (modGL *ModuleGL) assignMultiSetHash(run *wizard.ProverRuntime) {

	var lppCommitments field.Element
	if run.HasPublicInput(lppMerkleRootPublicInput + "_0") {
		lppCommitments = run.GetPublicInput(lppMerkleRootPublicInput + "_0")
	}

	var (
		segmentIndex             = modGL.SegmentModuleIndex.GetColAssignmentAt(run, 0)
		typeOfProof              = field.NewElement(uint64(proofTypeGL))
		hasSentOrReceive         = len(modGL.ReceivedValuesGlobalMap) > 0
		multiSetGeneral          = mimc.MSetHash{}
		multiSetSharedRandomness = mimc.MSetHash{}
		defInp                   = modGL.DefinitionInput
		moduleIndex              = field.NewElement(uint64(defInp.ModuleIndex))
		segmentIndexInt          = segmentIndex.Uint64()
		numSegmentModule         = modGL.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetVal(run)
	)

	multiSetSharedRandomness.Insert(moduleIndex, segmentIndex, lppCommitments)
	multiSetGeneral.Add(multiSetSharedRandomness)

	// If the segment is not the last one of its module we add the "sent" value
	// in the multiset.
	if hasSentOrReceive && segmentIndexInt < numSegmentModule.Uint64()-1 {
		globalSentHash := modGL.SentValuesGlobalHash.GetColAssignmentAt(run, 0)
		multiSetGeneral.Insert(moduleIndex, segmentIndex, typeOfProof, globalSentHash)
	}

	// If the segment is not the first one of its module, we add the received
	// value in the multiset
	if hasSentOrReceive && !segmentIndex.IsZero() {

		var (
			prevSegmentIndex field.Element
			one              = field.One()
			globalRcvdHash   = modGL.ReceivedValuesGlobalHash.GetColAssignmentAt(run, 0)
		)

		prevSegmentIndex.Sub(&segmentIndex, &one)
		multiSetGeneral.Remove(moduleIndex, prevSegmentIndex, typeOfProof, globalRcvdHash)
	}

	assignListOfPiColumns(run, GeneralMultiSetPublicInputBase, multiSetGeneral[:])
	assignListOfPiColumns(run, SharedRandomnessMultiSetPublicInputBase, multiSetSharedRandomness[:])
}

// checkMultiSetHash checks that the LPP commitment MSet is correctly
// assigned. It is meant to be run as part of a verifier action. The function
// also checks that isFirst and isLast are correctly assigned.
func (modGL *ModuleGL) checkMultiSetHash(run wizard.Runtime) error {

	var (
		targetMSetGeneral          = GetPublicInputList(run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
		targetMSetSharedRandomness = GetPublicInputList(run, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize)
		lppCommitments             = run.GetPublicInput(lppMerkleRootPublicInput + "_0")
		segmentIndex               = modGL.SegmentModuleIndex.GetColAssignmentAt(run, 0)
		typeOfProof                = field.NewElement(uint64(proofTypeGL))
		hasSentOrReceive           = len(modGL.ReceivedValuesGlobalMap) > 0
		multiSetGeneral            = mimc.MSetHash{}
		multiSetSharedRandomness   = mimc.MSetHash{}
		defInp                     = modGL.DefinitionInput
		moduleIndex                = field.NewElement(uint64(defInp.ModuleIndex))
		segmentIndexInt            = segmentIndex.Uint64()
		numSegmentOfCurrModule     = modGL.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetVal(run)
		isFirst                    = modGL.IsFirst.GetColAssignmentAt(run, 0)
		isLast                     = modGL.IsLast.GetColAssignmentAt(run, 0)
	)

	multiSetSharedRandomness.Insert(moduleIndex, segmentIndex, lppCommitments)
	multiSetGeneral.Add(multiSetSharedRandomness)

	if !isFirst.IsZero() && !isFirst.IsOne() {
		return fmt.Errorf("isFirst is not 0 or 1")
	}

	if !isLast.IsZero() && !isLast.IsOne() {
		return fmt.Errorf("isLast is not 0 or 1")
	}

	// This checks that isFirst and isLast are well assigned wrt to the segment
	// index
	if (segmentIndexInt == 0) != isFirst.IsOne() {
		return fmt.Errorf("isFirst does not match the segment index")
	}

	if (segmentIndexInt == numSegmentOfCurrModule.Uint64()-1) != isLast.IsOne() {
		return fmt.Errorf("isLast does not match the segment index")
	}

	// If the segment is not the last one of its module we add the "sent" value
	// in the multiset.
	if hasSentOrReceive && segmentIndexInt < numSegmentOfCurrModule.Uint64()-1 {
		globalSentHash := modGL.SentValuesGlobalHash.GetColAssignmentAt(run, 0)
		// This is a local module
		multiSetGeneral.Insert(moduleIndex, segmentIndex, typeOfProof, globalSentHash)
	}

	// If the segment is not the first one of its module, we add the received
	// value in the multiset
	if hasSentOrReceive && !segmentIndex.IsZero() {

		var (
			prevSegmentIndex field.Element
			one              = field.One()
			globalRcvdHash   = modGL.ReceivedValuesGlobalHash.GetColAssignmentAt(run, 0)
		)

		prevSegmentIndex.Sub(&segmentIndex, &one)
		multiSetGeneral.Remove(moduleIndex, prevSegmentIndex, typeOfProof, globalRcvdHash)
	}

	if !vector.Equal(targetMSetGeneral, multiSetGeneral[:]) {
		return fmt.Errorf("LPP commitment MSet mismatch, expected: %v, got: %v", targetMSetGeneral, multiSetGeneral[:])
	}

	if !vector.Equal(targetMSetSharedRandomness, multiSetSharedRandomness[:]) {
		return fmt.Errorf("shared randomness MSet mismatch, expected: %v, got: %v", targetMSetSharedRandomness, multiSetSharedRandomness[:])
	}

	return nil
}

// checkGnarkMultiSetHash checks that the LPP commitment MSet is correctly
// assigned. It is meant to be run as part of a verifier action.
func (modGL *ModuleGL) checkGnarkMultiSetHash(api frontend.API, run wizard.GnarkRuntime) error {

	var (
		targetMSetGeneral          = GetPublicInputListGnark(api, run, GeneralMultiSetPublicInputBase, mimc.MSetHashSize)
		targetMSetSharedRandomness = GetPublicInputListGnark(api, run, SharedRandomnessMultiSetPublicInputBase, mimc.MSetHashSize)
		lppCommitments             = run.GetPublicInput(api, lppMerkleRootPublicInput+"_0")
		segmentIndex               = modGL.SegmentModuleIndex.GetColAssignmentGnarkAt(run, 0)
		typeOfProof                = field.NewElement(uint64(proofTypeGL))
		hasSentOrReceive           = len(modGL.ReceivedValuesGlobalMap) > 0
		hasher                     = run.GetHasherFactory().NewHasher()
		multiSetGeneral            = mimc.EmptyMSetHashGnark(hasher)
		multiSetSharedRandomness   = mimc.EmptyMSetHashGnark(hasher)
		defInp                     = modGL.DefinitionInput
		moduleIndex                = frontend.Variable(defInp.ModuleIndex)
		numSegmentOfCurrModule     = modGL.PublicInputs.TargetNbSegments[defInp.ModuleIndex].Acc.GetFrontendVariable(api, run)
		isFirst                    = modGL.IsFirst.GetColAssignmentGnarkAt(run, 0)
		isLast                     = modGL.IsLast.GetColAssignmentGnarkAt(run, 0)
	)

	multiSetSharedRandomness.Insert(api, moduleIndex, segmentIndex, lppCommitments)
	multiSetGeneral.Add(api, multiSetSharedRandomness)

	api.AssertIsBoolean(isFirst)
	api.AssertIsBoolean(isLast)

	// This checks that isFirst and isLast are well assigned wrt to the segment
	// index
	api.AssertIsEqual(isFirst, api.IsZero(segmentIndex))
	api.AssertIsEqual(isLast, api.IsZero(api.Sub(numSegmentOfCurrModule, segmentIndex, 1)))

	// If the segment is not the last one of its module we add the "sent" value
	// in the multiset.
	if hasSentOrReceive {
		globalSentHash := modGL.SentValuesGlobalHash.GetColAssignmentGnarkAt(run, 0)
		mSetOfSentGlobal := mimc.MsetOfSingletonGnark(api, hasher, moduleIndex, segmentIndex, typeOfProof, globalSentHash)
		for i := range mSetOfSentGlobal.Inner {
			mSetOfSentGlobal.Inner[i] = api.Mul(mSetOfSentGlobal.Inner[i], api.Sub(1, isLast))
			multiSetGeneral.Inner[i] = api.Add(multiSetGeneral.Inner[i], mSetOfSentGlobal.Inner[i])
		}

		// If the segment is not the first one of its module, we add the received
		// value in the multiset. If the segment index is zero, the singleton hash
		// will be zero-ed out by the "1 - isFirst" term
		globalRcvdHash := modGL.ReceivedValuesGlobalHash.GetColAssignmentGnarkAt(run, 0)
		mSetOfReceivedGlobal := mimc.MsetOfSingletonGnark(api, hasher, moduleIndex, api.Sub(segmentIndex, 1), typeOfProof, globalRcvdHash)
		for i := range mSetOfReceivedGlobal.Inner {
			mSetOfReceivedGlobal.Inner[i] = api.Mul(mSetOfReceivedGlobal.Inner[i], api.Sub(1, isFirst))
			multiSetGeneral.Inner[i] = api.Sub(multiSetGeneral.Inner[i], mSetOfReceivedGlobal.Inner[i])
		}
	}

	for i := range multiSetGeneral.Inner {
		api.AssertIsEqual(multiSetGeneral.Inner[i], targetMSetGeneral[i])
		api.AssertIsEqual(multiSetSharedRandomness.Inner[i], targetMSetSharedRandomness[i])
	}

	return nil
}
