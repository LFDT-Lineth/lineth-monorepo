package plonkinternal

import (
	"fmt"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	plonkKoalabear "github.com/consensys/gnark/backend/plonk/koalabear"
	"github.com/consensys/gnark/constraint"
	cs "github.com/consensys/gnark/constraint/koalabear"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/profile"
	hasherfactory "github.com/consensys/linea-monorepo/prover/crypto/hasherfactory_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/plonkinternal/plonkbuilder"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

type Plonk struct {
	// The plonk circuit being integrated
	Circuit frontend.Circuit `serde:"omit"`
	// The traces of the compiled circuit
	trace *plonkKoalabear.Trace `serde:"omit"`
	// The sparse constrained system
	SPR *cs.SparseR1CS
	// Receives the list of rows which have to be marked containing range checks.
	rcGetter func() [][2]int `serde:"omit"`
	// hashedGetter is a function that returns the list of rows which are tagged
	// as range-checked.
	hashedGetter func() [][3][2]int `serde:"omit"`
}

// This flag control whether to activate the gnark profiling for the circuits. Please leave it
// to "false" because (1) it generates a lot of data (2) it is extremely time consuming.
const activateGnarkProfiling = false

// The CompilationCtx (context) carries all the compilation informations about a call to
// Plonk in Wizard. Namely, (non-exhaustively) it contains the gnark's internal
// informations about it, the generated Wizard columns and the compilation
// parameters that are used for the compilation. The context is also the
// receiver of all the methods allowing to construct the Plonk in Wizard module.
type CompilationCtx struct {
	// The compiled IOP
	comp *wizard.CompiledIOP
	// Name of the context. It is used to generate the column names and the
	// queries name so that we can understad where they come from. Two instances
	// of Plonk in wizard cannot have the same name.
	Name string
	// Subscript allows providing more context than [name]. It is used in the
	// logs and in the name of the profiling assets. It is however not used as
	// part of the name of generated wizard items.
	Subscript string
	// Round at which we create the ctx
	Round int
	// Number of instances of the circuit
	MaxNbInstances int

	Plonk Plonk

	// Columns
	Columns struct {
		// Circuit columns
		Ql, Qr, Qm, Qo, Qk, Qcp ifaces.Column
		// Witness columns
		L, R, O, PI, TinyPI, Cp []ifaces.Column
		// Activators are tiny verifier-visible columns that are used to
		// deactivate the constraints happening for constraints that are not
		// happening in the system. The verifier is required to check that the
		// columns are assigned to binary values and that they are structured
		// as a sequence of 1s followed by a sequence of 0s.
		Activators []ifaces.Column
		// Columns representing the permutation
		S [3]ifaces.ColAssignment
		// Commitment randomness
		Hcp coin.Info
		// The random coin output is an extension field element, but gnark
		// expects it to vector of base field elements. HcpEl contains the base
		// field decomposition of the extension field element.
		HcpEl []ifaces.Column
	}

	// Optional field used for specifying range checks option
	// parameters.
	RangeCheckOption struct {
		// wasCancelled is set if no wires need to be constrained
		wasCancelled         bool
		Enabled              bool
		NbBits               int
		NbLimbs              int
		AddGateForRangeCheck bool
		LimbDecomposition    []wizard.ProverAction
		// Selector for range checking from a column
		RcL, RcR, RcO ifaces.Column
		// RangeChecked stores the values to be range-checked
		RangeChecked []ifaces.Column
	}

	// FixedNbRowsOption is used to specify a fixed number of rows
	// in the CompilationCtx via the [WithFixedNbRow] option.
	FixedNbRowsOption struct {
		Enabled bool
		NbRow   int
	}

	// ExternalHasherOption is used to specify an external hasher
	// for the CompilationCtx via the [WithExternalHasher] option.
	ExternalHasherOption struct {
		Enabled bool
		// PosL, PosR and PosO are precomputing column storing the
		// positions of the L, R and O columns that are tagged for
		// each external hash constraints.
		PosOldState, PosBlock, PosNewState [poseidon2_koalabear.BlockSize]ifaces.Column
		// OldStates, Blocks, NewStates are the column affected by the
		// Poseidon2 query.
		OldStates, Blocks, NewStates [][poseidon2_koalabear.BlockSize]ifaces.Column
		// Fixed nb of row allows fixing the number of rows allocated
		// for the hash checking.
		FixedNbRows int
	}
}

// Create the context from a circuit. Extracts all the
// PLONK data representing the circuit
func createCtx(
	comp *wizard.CompiledIOP,
	name string,
	round int,
	circuit frontend.Circuit,
	maxNbInstance int,
	opts ...Option,
) (ctx CompilationCtx) {

	ctx = CompilationCtx{
		comp:           comp,
		Name:           name,
		Round:          round,
		MaxNbInstances: maxNbInstance,
	}

	ctx.Plonk.Circuit = circuit

	for _, opt := range opts {
		opt(&ctx)
	}

	logger := logrus.
		WithField("subscript", ctx.Subscript).
		WithField("round", ctx.Round).
		WithField("maxNbInstances", ctx.MaxNbInstances).
		WithField("name", ctx.Name)

	logger.Debug("Plonk in Wizard compiling the circuit")

	var pro *profile.Profile

	if activateGnarkProfiling {

		fname := name

		if len(ctx.Subscript) > 0 {
			fname = fname + "-" + ctx.Subscript
		}

		// This adds a nice pprof suffix
		fname = "profiling/" + fname + ".pprof"
		pro = profile.Start(profile.WithPath(fname))
	}

	var (
		ccs        *cs.SparseR1CS
		compileErr error
	)

	switch {
	case ctx.RangeCheckOption.Enabled:
		var rcGetter func() [][2]int
		ccs, rcGetter, compileErr = CompileCircuitWithRangeCheck(ctx.Plonk.Circuit, ctx.RangeCheckOption.AddGateForRangeCheck)
		ctx.Plonk.rcGetter = rcGetter
	case ctx.ExternalHasherOption.Enabled:
		var hshGetter func() [][3][2]int
		ccs, hshGetter, compileErr = CompileCircuitWithExternalHasher(ctx.Plonk.Circuit, true)
		ctx.Plonk.hashedGetter = hshGetter
	case !ctx.ExternalHasherOption.Enabled && !ctx.RangeCheckOption.Enabled:
		ccs, compileErr = CompileCircuitDefault(ctx.Plonk.Circuit)
	}

	if compileErr != nil {
		utils.Panic("error compiling the gnark circuit: name=%v err=%v", name, compileErr)
	}

	if activateGnarkProfiling {
		pro.Stop()
	}

	ctx.Plonk.SPR = ccs
	// We are passing a fake domain here for 2 reasons:
	// 	1. For all that matters for us, only what depends on the Cardinality is
	//		used in the PlonkInWizard compilation process. In particular, we
	// 		don't use the permutation columns created in [plonk.NewTraces]
	//	2. We may manipulate Plonk circuits with too many constraints to fit
	//		into a single FFT domain and passing an actual domain here would
	//		be impossible. However, nothing bars us from actually manipulating
	//		them in the wizard world.
	fftDomain := &fft.Domain{Cardinality: uint64(ctx.DomainSize())}

	if ctx.FixedNbRowsOption.Enabled && ctx.FixedNbRowsOption.NbRow < ctx.DomainSizePlonk() {
		utils.Panic("plonk-in-wizard: the number of constraints of the circuit outweight the fixed number of rows. fixed-nb-row=%v domain-size=%v nb-constraints=%v", ctx.FixedNbRowsOption.NbRow, ctx.DomainSizePlonk(), ccs.NbConstraints)
	}

	ctx.Plonk.trace = plonkKoalabear.NewTrace(ctx.Plonk.SPR, fftDomain)

	ctx.buildPermutation(ctx.Plonk.SPR, ctx.Plonk.trace) // no part of BuildTrace

	logger.
		WithField("nbConstraints", ccs.GetNbConstraints()).
		WithField("nbInternalVariables", ccs.GetNbInternalVariables()).
		WithField("columnSizeGnark", ctx.Plonk.SPR.NbConstraints+len(ctx.Plonk.SPR.Public)).
		Info("[plonk-in-wizard] done compiling the circuit")

	return ctx
}

// CompileCircuitDefault compiles the circuit using the default scs.Builder
// of gnark.
func CompileCircuitDefault(circ frontend.Circuit) (*cs.SparseR1CS, error) {
	newBuilder := plonkbuilder.From(scs.NewBuilder[constraint.U32])
	ccs, err := frontend.CompileU32(koalabear.Modulus(), newBuilder, circ)
	if err != nil {
		return nil, fmt.Errorf("frontend.Compile returned an err=%v", err)
	}

	return ccs.(*cs.SparseR1CS), err
}

// CompileCircuitWithRangeCheck compiles the circuit and returns the compiled
// constraints system.
func CompileCircuitWithRangeCheck(circ frontend.Circuit, addGates bool) (*cs.SparseR1CS, func() [][2]int, error) {

	gnarkBuilder, rcGetter := NewExternalRangeCheckerBuilder(addGates)

	// ccsIface, err := frontend.Compile(ecc.BLS12_377.ScalarField(), gnarkBuilder, circ)
	ccs, err := frontend.CompileU32(koalabear.Modulus(), gnarkBuilder, circ)
	if err != nil {
		return nil, nil, fmt.Errorf("frontend.Compile returned an err=%v", err)
	}

	return ccs.(*cs.SparseR1CS), rcGetter, err
}

// CompileCircuitWithExternalHasher compiles the circuit and returns the compiled
// constraints system.
func CompileCircuitWithExternalHasher(circ frontend.Circuit, addGates bool) (*cs.SparseR1CS, func() [][3][2]int, error) {

	gnarkBuilder, hshGetter := hasherfactory.NewExternalHasherBuilder(addGates)
	ccs, err := frontend.CompileU32(koalabear.Modulus(), gnarkBuilder, circ)
	if err != nil {
		return nil, nil, fmt.Errorf("frontend.Compile returned an err=%v", err)
	}

	return ccs.(*cs.SparseR1CS), hshGetter, err
}

// DomainSize returns the size of the domain. Meaning the size of the columns
// taking part in the wizard for the current Plonk instance. The function
// returns the next power of two of the number of constraints. Or, if the
// option [WithFixedNbRows] is used, the fixed number of rows.
func (ctx *CompilationCtx) DomainSize() int {

	if ctx.FixedNbRowsOption.Enabled {
		return ctx.FixedNbRowsOption.NbRow
	}

	return ctx.DomainSizePlonk()
}

// DomainSizePlonk returns the total size of the domain according to gnark.
// Ignoring the [FixedNbRowsOption].
func (ctx *CompilationCtx) DomainSizePlonk() int {
	return utils.NextPowerOfTwo(
		ctx.Plonk.SPR.NbConstraints + len(ctx.Plonk.SPR.Public),
	)
}

// Returns the size of the public input tiny column
func tinyPISize(spr *cs.SparseR1CS) int {
	return utils.NextPowerOfTwo(
		spr.GetNbPublicVariables(),
	)
}

// Complete the Plonk.Trace by computing the S permutation.
// (Copied from gnark)
//
// buildPermutation builds the Permutation associated with a circuit.
//
// The permutation s is composed of cycles of maximum length such that
//
//	s. (l∥r∥o) = (l∥r∥o)
//
// , where l∥r∥o is the concatenation of the indices of l, r, o in
// ql.l+qr.r+qm.l.r+qo.O+k = 0.
//
// The permutation is encoded as a slice s of size 3*size(l), where the
// i-th entry of l∥r∥o is sent to the s[i]-th entry, so it acts on a tab
// like this: for i in tab: tab[i] = tab[permutation[i]]
func (ctx *CompilationCtx) buildPermutation(spr *cs.SparseR1CS, pt *plonkKoalabear.Trace) {

	// nbVariables counts the number of variables occuring in the Plonk circuit. The
	// +1 is to account for a "special" variable that we use for padding. It is
	// associated with the "nbVariables - 1" variable-ID.
	nbVariables := spr.NbInternalVariables + len(spr.Public) + len(spr.Secret) + 1

	// nbVariables := spr.NbInternalVariables + len(spr.Public) + len(spr.Secret)
	sizeSolution := ctx.DomainSize()
	sizePermutation := 3 * sizeSolution

	permutation := make([]int64, sizePermutation)
	for i := 0; i < len(permutation); i++ {
		permutation[i] = -1
	}

	// init LRO position -> variable_ID
	lro := make([]int, sizePermutation) // position -> variable_ID
	for i := 0; i < len(spr.Public); i++ {
		lro[i] = i // IDs of LRO associated to placeholders (only L needs to be taken care of)
	}

	offset := len(spr.Public)

	j := 0
	it := spr.GetSparseR1CIterator()
	for c := it.Next(); c != nil; c = it.Next() {
		lro[offset+j] = int(c.XA)
		lro[sizeSolution+offset+j] = int(c.XB)
		lro[2*sizeSolution+offset+j] = int(c.XC)

		j++
	}

	for ; j < sizeSolution-offset; j++ {
		lro[offset+j] = nbVariables - 1
		lro[sizeSolution+offset+j] = nbVariables - 1
		lro[2*sizeSolution+offset+j] = nbVariables - 1
	}

	// init cycle:
	// map ID -> last position the ID was seen
	cycle := make([]int64, nbVariables)
	for i := 0; i < len(cycle); i++ {
		cycle[i] = -1
	}

	for i := 0; i < len(lro); i++ {
		if cycle[lro[i]] != -1 {
			// if != -1, it means we already encountered this value
			// so we need to set the corresponding permutation index.
			permutation[i] = cycle[lro[i]]
		}
		cycle[lro[i]] = int64(i)
	}

	// complete the Permutation by filling the first IDs encountered
	for i := 0; i < sizePermutation; i++ {
		if permutation[i] == -1 {
			permutation[i] = cycle[lro[i]]
		}
	}

	pt.S = permutation
}

// GetPlonkProverAction returns the [PlonkInWizardProverAction] responsible for
// assigning the first round of the wizard. In case we use the BBS commitment
// this stands for [initialBBSProverAction] or [noCommitProverAction] in the
// contrary case.
func (ctx CompilationCtx) GetPlonkProverAction() PlonkInWizardProverAction {
	if ctx.HasCommitment() {
		return InitialBBSProverAction{
			GenericPlonkProverAction: ctx.GenericPlonkProverAction(),
			ProverStateLock:          &sync.Mutex{},
		}
	}
	return PlonkNoCommitProverAction{
		GenericPlonkProverAction: ctx.GenericPlonkProverAction(),
	}
}

func (ctx CompilationCtx) GenericPlonkProverAction() GenericPlonkProverAction {
	return GenericPlonkProverAction{
		Name:           ctx.Name,
		SPR:            ctx.Plonk.SPR,
		MaxNbInstances: ctx.MaxNbInstances,
		DomainSize:     ctx.DomainSize(),
		Columns: struct {
			L          []ifaces.Column
			R          []ifaces.Column
			O          []ifaces.Column
			Cp         []ifaces.Column
			Hcp        coin.Info
			HcpEl      []ifaces.Column
			Activators []ifaces.Column
			TinyPI     []ifaces.Column
		}{
			L:          ctx.Columns.L,
			R:          ctx.Columns.R,
			O:          ctx.Columns.O,
			Cp:         ctx.Columns.Cp,
			Hcp:        ctx.Columns.Hcp,
			HcpEl:      ctx.Columns.HcpEl,
			Activators: ctx.Columns.Activators,
			TinyPI:     ctx.Columns.TinyPI,
		},
		RangeCheckOption: struct {
			WasCancelled         bool
			Enabled              bool
			NbBits               int
			NbLimbs              int
			AddGateForRangeCheck bool
			LimbDecomposition    []wizard.ProverAction
			RcL                  ifaces.Column
			RcR                  ifaces.Column
			RcO                  ifaces.Column
			RangeChecked         []ifaces.Column
		}{
			WasCancelled:         ctx.RangeCheckOption.wasCancelled,
			Enabled:              ctx.RangeCheckOption.Enabled,
			NbBits:               ctx.RangeCheckOption.NbBits,
			NbLimbs:              ctx.RangeCheckOption.NbLimbs,
			AddGateForRangeCheck: ctx.RangeCheckOption.AddGateForRangeCheck,
			LimbDecomposition:    ctx.RangeCheckOption.LimbDecomposition,
			RcL:                  ctx.RangeCheckOption.RcL,
			RcR:                  ctx.RangeCheckOption.RcR,
			RcO:                  ctx.RangeCheckOption.RcO,
			RangeChecked:         ctx.RangeCheckOption.RangeChecked,
		},
		ExternalHasherOption: struct {
			Enabled     bool
			PosOldState [poseidon2_koalabear.BlockSize]ifaces.Column
			PosBlock    [poseidon2_koalabear.BlockSize]ifaces.Column
			PosNewState [poseidon2_koalabear.BlockSize]ifaces.Column
			OldStates   [][poseidon2_koalabear.BlockSize]ifaces.Column
			Blocks      [][poseidon2_koalabear.BlockSize]ifaces.Column
			NewStates   [][poseidon2_koalabear.BlockSize]ifaces.Column
		}{
			Enabled:     ctx.ExternalHasherOption.Enabled,
			PosOldState: ctx.ExternalHasherOption.PosOldState,
			PosBlock:    ctx.ExternalHasherOption.PosBlock,
			PosNewState: ctx.ExternalHasherOption.PosNewState,
			OldStates:   ctx.ExternalHasherOption.OldStates,
			Blocks:      ctx.ExternalHasherOption.Blocks,
			NewStates:   ctx.ExternalHasherOption.NewStates,
		},
	}
}

// GenericPlonkProverAction is a collection data-structure which contains the
// elements used by the prover. It is wrapped by the actual prover actions
// taking place in the [PlonkInWizardProverAction] interface.
type GenericPlonkProverAction struct {
	Name           string
	SPR            *cs.SparseR1CS
	MaxNbInstances int
	DomainSize     int
	Columns        struct {
		L          []ifaces.Column
		R          []ifaces.Column
		O          []ifaces.Column
		Cp         []ifaces.Column
		Hcp        coin.Info
		HcpEl      []ifaces.Column
		Activators []ifaces.Column
		TinyPI     []ifaces.Column
	}
	RangeCheckOption struct {
		// WasCancelled is set if no wires need to be constrained
		WasCancelled         bool
		Enabled              bool
		NbBits               int
		NbLimbs              int
		AddGateForRangeCheck bool
		LimbDecomposition    []wizard.ProverAction
		// Selector for range checking from a column
		RcL, RcR, RcO ifaces.Column
		// RangeChecked stores the values to be range-checked
		RangeChecked []ifaces.Column
	}
	ExternalHasherOption struct {
		Enabled                            bool
		PosOldState, PosBlock, PosNewState [poseidon2_koalabear.BlockSize]ifaces.Column
		OldStates, Blocks, NewStates       [][poseidon2_koalabear.BlockSize]ifaces.Column
	}
	FixedNbRows int
}
