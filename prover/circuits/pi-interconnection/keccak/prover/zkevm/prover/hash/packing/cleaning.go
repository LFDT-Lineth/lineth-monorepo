package packing

import (
	"math/big"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
)

// cleaningInputs collects the inputs of [NewClean] function.
type cleaningInputs struct {
	// It stores Limb-column that is subject to cleaning,
	// given the meaningful number of bytes in nByte-column.
	Imported Importation
	// Lookup table used for storing powers of 2^8,
	// removing the redundant zeroes from Limbs.
	Lookup lookUpTables
	// Name gives additional context for the input name
	Name string
}

// cleaningCtx stores all the intermediate columns required for imposing the constraints.
// Cleaning module is responsible for cleaning the limbs.
type cleaningCtx struct {
	Inputs *cleaningInputs
	// The column storing the result of the cleaning
	CleanLimb ifaces.Column
	// NbZeros =  MaxBytes - imported.NBytes
	NbZeros ifaces.Column
	// PowersNbZeros represent powers of nbZeroes;
	//  PowersNbZeros = 2^(8 * nbZeroes)
	PowersNbZeros ifaces.Column
}

// NewClean imposes the constraint for cleaning the limbs.
func NewClean(comp *wizard.CompiledIOP, inp cleaningInputs) cleaningCtx {

	createCol := common.CreateColFn(comp, CLEANING+"_"+inp.Name, inp.Imported.Limb.Size(), pragmas.RightPadded)
	ctx := cleaningCtx{
		CleanLimb:     createCol("CleanLimb"),
		NbZeros:       createCol("NbZeroes"),
		PowersNbZeros: createCol("PowersNbZeroes"),
		Inputs:        &inp,
	}

	// impose that;
	//  -  powersNbZeros = 2^(8*nbZeros)
	//  - nbZeros = MaxBytes - NBytes
	ctx.csNbZeros(comp)

	// impose the cleaning of limbs
	limb := sym.Mul(ctx.PowersNbZeros, ctx.CleanLimb)

	comp.InsertGlobal(0, ifaces.QueryIDf("LimbCleaning_%v", inp.Name),
		sym.Sub(limb, inp.Imported.Limb),
	)

	return ctx
}

// csNbZeros imposes the constraints between nbZero and powersNbZeros;
// -  powersNbZeros = 2^(nbZeros * 8)
//
// -  nbZeros = 16 - nByte
func (ctx cleaningCtx) csNbZeros(comp *wizard.CompiledIOP) {
	var (
		isActive = ctx.Inputs.Imported.IsActive
		nByte    = ctx.Inputs.Imported.NByte
	)

	// Equivalence of "PowersNbZeros" with "2^(NbZeros * 8)"
	comp.InsertInclusion(0, ifaces.QueryIDf("NumToPowers_%v", ctx.Inputs.Name),
		[]ifaces.Column{ctx.Inputs.Lookup.ColNumber, ctx.Inputs.Lookup.ColPowers},
		[]ifaces.Column{ctx.NbZeros, ctx.PowersNbZeros},
	)
	commonconstraints.MustZeroWhenInactive(comp, isActive, ctx.NbZeros)

	//  The constraint for nbZeros = (MaxBytes - NByte)* isActive
	nbZeros := sym.Sub(MAXNBYTE, nByte)
	comp.InsertGlobal(0, ifaces.QueryIDf("NB_ZEROES_%v", ctx.Inputs.Name),
		sym.Mul(
			sym.Sub(
				nbZeros, ctx.NbZeros),
			isActive),
	)
}

// assign the native columns
func (ctx *cleaningCtx) Assign(run *wizard.ProverRuntime) {
	ctx.assignNbZeros(run)
	ctx.assignCleanLimbs(run)
}

func (ctx *cleaningCtx) assignCleanLimbs(run *wizard.ProverRuntime) {

	var (
		cleanLimbs = common.NewVectorBuilder(ctx.CleanLimb)
		limbs      = ctx.Inputs.Imported.Limb.GetColAssignment(run).IntoRegVecSaveAlloc()
		nByte      = ctx.Inputs.Imported.NByte.GetColAssignment(run).IntoRegVecSaveAlloc()
	)

	// populate cleanLimbs
	limbSerialized := [32]byte{}
	var f field.Element
	for pos := 0; pos < len(limbs); pos++ {
		// Extract the limb, which is left aligned to the 16-th byte
		limbSerialized = limbs[pos].Bytes()
		nbyte := field.ToInt(&nByte[pos])
		res := limbSerialized[LEFT_ALIGNMENT : LEFT_ALIGNMENT+nbyte]
		cleanLimbs.PushField(*(f.SetBytes(res)))
	}

	cleanLimbs.PadAndAssign(run, field.Zero())
}

func (ctx *cleaningCtx) assignNbZeros(run *wizard.ProverRuntime) {
	// populate nbZeros and powersNbZeros
	var (
		nbZeros       = *common.NewVectorBuilder(ctx.NbZeros)
		powersNbZeros = common.NewVectorBuilder(ctx.PowersNbZeros)
		nByte         = smartvectors.Window(ctx.Inputs.Imported.NByte.GetColAssignment(run))
	)

	fr16 := field.NewElement(16)
	var res field.Element
	var a big.Int
	for row := 0; row < len(nByte); row++ {
		b := nByte[row]

		// @alex: it is possible that the "imported" is returning "inactive"
		// zones when using Sha2.
		if b.IsZero() {
			nbZeros.PushZero()
			powersNbZeros.PushOne()
			continue
		}

		res.Sub(&fr16, &b)
		nbZeros.PushField(res)
		res.BigInt(&a)
		res.Exp(field.NewElement(POWER8), &a)
		powersNbZeros.PushField(res)
	}

	nbZeros.PadAndAssign(run, field.Zero())
	powersNbZeros.PadAndAssign(run, field.One())
}

// newCleaningInputs constructs CleaningInputs
func newCleaningInputs(imported Importation, lookup lookUpTables, name string) cleaningInputs {
	return cleaningInputs{
		Imported: imported,
		Lookup:   lookup,
		Name:     name,
	}
}
