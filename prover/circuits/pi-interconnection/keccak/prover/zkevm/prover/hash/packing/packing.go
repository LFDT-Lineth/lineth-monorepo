// Packing package implements the utilities for Packing
// the limbs of variable length to the lanes of fixed length.
package packing

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/packing/dedicated/spaghettifier"
)

const (
	MAXNBYTE       = 16
	LEFT_ALIGNMENT = 16

	POWER8 = 1 << 8
)

const (
	PACKING            = "PACKING"
	CLEANING           = "CLEANING"
	DECOMPOSITION      = "DECOMPOSITION"
	LENGTH_CONSISTENCY = "LENGTH_CONSISTENCY"
	SPAGHETTI          = "SPAGHETTI"
	LANE               = "LANE"
	BLOCK              = "BLOCK"
)

// Importaion implements the set of required columns for launching the Packing module.
type Importation struct {
	// The set of the limbs that are subject to Packing (i.e., should be  pushed into the pack).
	// Limbs are 16 bytes, left aligned.
	Limb ifaces.Column
	// It is 1 if the associated limb is the first limb of the new hash.
	IsNewHash ifaces.Column
	// NByte is the meaningful length of limbs in byte.
	// \Sum NByte[i] should divide the blockSize,
	// otherwise a padding step is required before calling the Packing module
	NByte ifaces.Column
	// The active part of the columns in the Importation module
	IsActive ifaces.Column
}

// The set of parameters and columns required to launch the module.
type PackingInput struct {
	// The maximum number of blocks that can be extracted from limbs.
	// If the number of blocks passes the max, newPack() would panic.
	MaxNumBlocks int
	PackingParam generic.HashingUsecase
	// The columns in Imported should be of size;
	// size := utils.NextPowerOfTwo(packingParam.blockSize * maxNumBlocks)
	Imported Importation
	Name     string
}

// Packing implements the [wizard.ProverAction] receiving the limbs and relevant parameters,
//
//	it repack them in the lanes of the same size.
type Packing struct {
	Inputs PackingInput

	// submodules
	Cleaning   cleaningCtx
	LookUps    lookUpTables
	Decomposed decomposition
	// it stores the result of the Packing
	// limbs are repacked in Lane column.
	Repacked laneRepacking
	Block    block
}

/*
NewPack creates a Packing object.

The lanes and relevant metadata can be accessed via Packing.Repacked.

It panics if  the columns in Imported have a size different from;

	size := utils.NextPowerOfTwo(inp.packingParam.blockSize * inp.maxNumBlocks)

It also panics if the number of generated blocks passes the limit inp.maxNumBlocks
*/
func NewPack(comp *wizard.CompiledIOP, inp PackingInput) *Packing {
	var (
		isNewHash  = inp.Imported.IsNewHash
		lookup     = NewLookupTables(comp)
		cleaning   = NewClean(comp, newCleaningInputs(inp.Imported, lookup, inp.Name))
		decomposed = newDecomposition(comp, getDecompositionInputs(cleaning, inp))
		spaghetti  = spaghettiMaker(comp, decomposed, isNewHash)
		lanes      = newLane(comp, spaghetti, inp)
		block      = newBlock(comp, getBlockInputs(lanes, inp.PackingParam))
	)

	return &Packing{
		Inputs:     inp,
		Cleaning:   cleaning,
		Decomposed: decomposed,
		Repacked:   lanes,
		Block:      block,
	}
}

// Run assign the Packing module.
func (pck *Packing) Run(run *wizard.ProverRuntime) {

	// assign subModules
	pck.Cleaning.Assign(run)
	pck.Decomposed.Assign(run)
	pck.Repacked.Assign(run)
	pck.Block.Assign(run)
}

// it stores the inputs /outputs of spaghettifier used in the Packing module.
type spaghettiCtx struct {
	// ContentSpaghetti
	DecLimbSp, DecLenSp, DecLenPowerSp ifaces.Column
	NewHashSp                          ifaces.Column
	// FilterSpaghetti
	FilterSpaghetti ifaces.Column
	PA              wizard.ProverAction
	SpaghettiSize   int
}

func spaghettiMaker(comp *wizard.CompiledIOP, decomposed decomposition, isNewHash ifaces.Column) spaghettiCtx {

	var (
		isNewHashTable []ifaces.Column
		size           = decomposed.Size
		zeroCol        = verifiercol.NewConstantCol(field.Zero(), size, fmt.Sprintf("spaghetti-maker-%v", decomposed.Inputs.Name))
	)

	// build isNewHash
	isNewHashTable = append(isNewHashTable, isNewHash)
	for i := 1; i < decomposed.NbSlices; i++ {
		isNewHashTable = append(isNewHashTable, zeroCol)
	}

	// Constraints over the spaghetti forms
	inp := spaghettifier.SpaghettificationInput{
		Name: decomposed.Inputs.Name,
		ContentMatrix: [][]ifaces.Column{
			decomposed.DecomposedLimbs,
			decomposed.DecomposedLen,
			decomposed.DecomposedLenPowers,
			isNewHashTable,
		},
		Filter:        decomposed.Filter,
		SpaghettiSize: decomposed.Size,
	}
	// declare ProverAction for Spaghettification
	pa := spaghettifier.Spaghettify(comp, inp)

	s := spaghettiCtx{
		PA:              pa,
		DecLimbSp:       pa.ContentSpaghetti[0],
		DecLenSp:        pa.ContentSpaghetti[1],
		DecLenPowerSp:   pa.ContentSpaghetti[2],
		NewHashSp:       pa.ContentSpaghetti[3],
		SpaghettiSize:   decomposed.Size,
		FilterSpaghetti: pa.FilterSpaghetti,
	}

	return s
}
