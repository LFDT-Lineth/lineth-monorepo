package modexp

import (
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	// smallModexpSize is the bit-size bound for small modexp instances
	smallModexpSize = 256
	// largeModexpSize is the bit-size bound for large modexp instances
	largeModexpSize = 8192
	// limbSize is the size (in bits) of a limb as in the public inputs of the
	// circuit. This is a parameter linked to how the arithmetization encodes
	// 256 bits integers.
	limbSizeBits = 128
	// nbLargeModexpLimbs is the number of limbs used to represent
	// large modexp operands
	nbLargeModexpLimbs = largeModexpSize / limbSizeBits
	// modexpNumRows corresponds to the number of rows present in the MODEXP
	// module to represent a single instance. Each instance has 4 operands
	// dispatched in limbs of [limbSizeBits] bits.
	modexpNumRowsPerInstance = nbLargeModexpLimbs * 4
)

// Module implements the wizard part responsible for checking the MODEXP
// claims coming from the BLKMDXP module of the arithmetization.
type Module struct {
	// Input stores the columns used as a source for the antichamber.
	Input Input
	// IsActive is a binary indicator column marking with a 1, the rows of the
	// antichamber modules corresponding "active" rows: e.g. NOT padding rows.
	IsActive ifaces.Column
	// IsSmall, IsLarge are indicator columns that are constant per modexp
	// instances. They are mutually exclusive and activated by IsActive.
	IsSmall, IsLarge ifaces.Column
	// Limb contains the modexp arguments and is subjected to a projection
	// constraint from the BLK_MDXP (using IsActive as filter). It is constrained
	// to zero when IsActive = 0.
	Limbs ifaces.Column
	// ToSmallCirc and ToLargeCirc are indicator columns marking with a 1 the
	// positions of limbs corresponding to public inputs of (respectely) the
	// small and the large circuit.
	ToSmallCirc ifaces.Column
	// GnarkCircuitConnector256Bits, GnarkCircuitConnectorLarge are the
	// connection logic of the modexp circuit specialized for the small and
	// large instances respectively.
	//
	// If the alignments are nil, then it means that the circuits are not
	// initialized. It is useful for testing the glue assignment. To define the
	// circuits, call [Module.WithCircuit].
	GnarkCircuitConnector256Bits, GnarkCircuitConnectorLarge *plonk.Alignment
}

// NewModuleZkEvm constructs an instance of the modexp module. It should be called
// only once.
//
// To define the circuit, call [Module.WithCircuit] as the present function
// does not define them.
func NewModuleZkEvm(comp *wizard.CompiledIOP, settings Settings) *Module {
	return newModule(comp, newZkEVMInput(comp, settings)).
		WithCircuit(comp, query.PlonkRangeCheckOption(16, 6, false))
}

func newModule(comp *wizard.CompiledIOP, input Input) *Module {

	var (
		settings      = input.Settings
		maxNbInstance = settings.MaxNbInstance256 + settings.MaxNbInstanceLarge
		size          = utils.NextPowerOfTwo(maxNbInstance * modexpNumRowsPerInstance)
		mod           = &Module{
			Input:       input,
			IsActive:    comp.InsertCommit(0, "MODEXP_IS_ACTIVE", size),
			Limbs:       comp.InsertCommit(0, "MODEXP_LIMBS", size),
			IsSmall:     comp.InsertCommit(0, "MODEXP_IS_SMALL", size),
			IsLarge:     comp.InsertCommit(0, "MODEXP_IS_LARGE", size),
			ToSmallCirc: comp.InsertCommit(0, "MODEXP_TO_SMALL_CIRC", size),
		}
	)

	pragmas.MarkRightPadded(mod.IsActive)

	mod.Input.setIsModexp(comp)

	mod.csIsActive(comp)
	mod.csIsSmallAndLarge(comp)
	mod.csToCirc(comp)

	comp.InsertProjection(
		"MODEXP_BLKMDXP_PROJECTION",
		query.ProjectionInput{ColumnA: []ifaces.Column{mod.Input.Limbs},
			ColumnB: []ifaces.Column{mod.Limbs},
			FilterA: mod.Input.IsModExp,
			FilterB: mod.IsActive})
	return mod
}

// WithCircuits adds the Plonk-in-Wizard circuit verification to complete
// the anti-chamber.
func (mod *Module) WithCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *Module {
	settings := mod.Input.Settings

	if settings.MaxNbInstance256 > 0 {
		mod.GnarkCircuitConnector256Bits = plonk.DefineAlignment(
			comp,
			&plonk.CircuitAlignmentInput{
				Name:               "MODEXP_256_BITS",
				DataToCircuit:      mod.Limbs,
				DataToCircuitMask:  mod.ToSmallCirc,
				Circuit:            allocateCircuit(settings.NbInstancesPerCircuitModexp256, smallModexpSize),
				NbCircuitInstances: utils.DivCeil(settings.MaxNbInstance256, settings.NbInstancesPerCircuitModexp256),
				PlonkOptions:       options,
			},
		)
	}

	if settings.MaxNbInstanceLarge > 0 {
		mod.GnarkCircuitConnectorLarge = plonk.DefineAlignment(
			comp,
			&plonk.CircuitAlignmentInput{
				Name:               "MODEXP_LARGE",
				DataToCircuit:      mod.Limbs,
				DataToCircuitMask:  mod.IsLarge,
				Circuit:            allocateCircuit(settings.NbInstancesPerCircuitModexpLarge, largeModexpSize),
				NbCircuitInstances: utils.DivCeil(settings.MaxNbInstanceLarge, settings.NbInstancesPerCircuitModexpLarge),
				PlonkOptions:       options,
			},
		)
	}
	if mod.GnarkCircuitConnector256Bits == nil && mod.GnarkCircuitConnectorLarge == nil {
		utils.Panic("modexp module: circuit connection defined but no modexp limits defined")
	}

	return mod
}

// csIsActive ensures that the ant.IsActive column is well constructed and that
// all the other columns are zero when it is zero.
func (mod *Module) csIsActive(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, mod.IsActive)

	comp.InsertGlobal(
		0,
		"MODEXP_IS_ACTIVE_DOES_NOT_INCREASE",
		sym.Mul(
			mod.IsActive,
			sym.Sub(mod.IsActive, column.Shift(mod.IsActive, -1)),
		),
	)

	//
	// NB: IsActive can only have a multiple of 128 1's. That's because it is
	// not supposed to go off in the middle of an actual modexp instance.
	// However, this is pre-enforced by the fact that this column is used as
	// the indicator of a projection query linking the blk_mdxp module with
	// the antichamber.
	//
	// This implictly constrains that aspect and thus, it does not require
	// a particular constraint.
	//

	mustCancelWhenBinCancel(comp, mod.IsActive, mod.Limbs)
}

// csIsSmallAndLarge constrains IsSmall and IsLarge
func (mod *Module) csIsSmallAndLarge(comp *wizard.CompiledIOP) {

	mustBeBinary(comp, mod.IsSmall)
	mustBeBinary(comp, mod.IsLarge)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("MODEXP_IS_SMALL_LARGE_ARE_MUTUALLY_EXCLUSIVE"),
		sym.Sub(
			mod.IsActive,
			sym.Add(mod.IsSmall, mod.IsLarge),
		),
	)

	//
	// NB: The facts that
	// * IsSmall and IsLarge are mutually exclusive columns
	// * that IsActive implictly only switches at the end of the last modexp
	// instance (see the comment in the [csIsActive] function).
	// * The constraint [MODEXP_IS_SMALL_CONSTANT_BY_SEGMENT] is here
	//
	// Imply already an equivalent [MODEXP_IS_LARGE_CONSTANT_BY_SEGMENT]
	// constraint. So we do not need to declare it.
	//

	comp.InsertGlobal(
		0,
		"MODEXP_IS_SMALL_CONSTANT_BY_SEGMENT",
		sym.Mul(
			sym.Sub(1, variables.NewPeriodicSample(modexpNumRowsPerInstance, 0)),
			sym.Sub(mod.IsSmall, column.Shift(mod.IsSmall, -1)),
		),
	)

	//
	// The constraint below ensures that if the IS_SMALL flag is set, then the
	// limbs 2..64 of the operands of the corresponding modexp must be zero
	// (otherwise, they would represent numbers larger than 256 bits).
	//
	// The converse constraint does not exists in the large (8192) case because it would
	// not be wrong to supply
	//

	comp.InsertGlobal(
		0,
		"MODEXP_IS_SMALL_IMPLIES_SMALL_OPERANDS",
		sym.Mul(
			mod.Limbs,
			mod.IsSmall,
			sym.Sub(1,
				variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-2),
				variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-1),
			),
		),
	)
}

// csToCirc ensures the well-construction of ant.ToSmallCirc
func (mod *Module) csToCirc(comp *wizard.CompiledIOP) {

	comp.InsertGlobal(
		0,
		"MODEXP_TO_SMALL_CIRC_VAL",
		sym.Sub(
			mod.ToSmallCirc,
			sym.Mul(
				mod.IsSmall,
				sym.Add(
					variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-2),
					variables.NewPeriodicSample(nbLargeModexpLimbs, nbLargeModexpLimbs-1),
				),
			),
		),
	)

	//
	// NB: We set ToLargeCirc = IsLarge as these to indicator coincidates
	// so there is no need to add extra constraints.
	//
}

// mustBeBinary constraints c to be binary
func mustBeBinary(comp *wizard.CompiledIOP, c ifaces.Column) {

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_CANCEL_IS_BINARY", c.GetColID()),
		sym.Mul(c, sym.Sub(c, 1)),
	)
}

// mustCancelWhenBinCancel enforces to 'c' to be zero when the binary column
// `bin` is zero. The constraint does not work if bin is not constrained to be
// binary.
func mustCancelWhenBinCancel(comp *wizard.CompiledIOP, bin, c ifaces.Column) {

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_CANCEL_WHEN_NOT_%v", c.GetColID(), bin.GetColID()),
		sym.Mul(
			sym.Sub(1, bin),
			c,
		),
	)
}
