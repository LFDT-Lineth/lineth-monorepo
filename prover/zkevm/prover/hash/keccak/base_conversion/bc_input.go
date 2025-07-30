/* base_conversion package implements the utilities for base conversion,
as it is required by keccakf.

The inputs to keccakf are blocks in BaseA or BaseB (little-endian).
The output from keccakf is the hash result in baseB (little-endian).

 Thus, the implementation applies a base conversion over the blocks;
going from uint-BE to BaseA/BaseB-LE.

Also, a base conversion over the hash result,
going from BaseB-LE to uint-BE. */

package base_conversion

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/keccakf"
)

const (
	BASE_CONVERSION = "BASE_CONVERSION"
	BLOCK           = "BLOCK"
	HASH_OUTPUT     = "HASH_OUTPUT"
	Power16         = 1 << 16
	Power8          = 1 << 8
	POWER4          = 1 << 4
	LaneSizeByte    = 8  // size of the lanes in byte
	numLimbsOutput  = 32 // number of limbs for base conversion over the output
	numLimbsInput   = 4  // number of limbs for base conversion over the input
	halfDigest      = 16 // half of the digest size for keccak hash

	baseAPower16 = keccakf.BaseAPow4 * keccakf.BaseAPow4 *
		keccakf.BaseAPow4 * keccakf.BaseAPow4 // no overflow

	baseBPower16 = keccakf.BaseBPow4 * keccakf.BaseBPow4 *
		keccakf.BaseBPow4 * keccakf.BaseBPow4
)

// BlockBaseConversionInputs stores the inputs for [newBlockBaseConversion]
type BlockBaseConversionInputs struct {
	Lane                 ifaces.Column
	IsFirstLaneOfNewHash ifaces.Column
	IsLaneActive         ifaces.Column
	Lookup               lookUpTables
}

// The submodule BlockBaseConversion implements the base conversion over
// the inputs to the keccakf (i.e., blocks/lanes), in order to export them to the keccakf.
// The lanes from the first block of hash should be in baseA and others are in baseB.
type BlockBaseConversion struct {
	Inputs *BlockBaseConversionInputs
	// It is 1 when the lane is from the first block of the hash
	IsFromFirstBlock ifaces.Column
	// IsFromBlockBaseB := 1-isFromFirstBlock
	IsFromBlockBaseB ifaces.Column
	// Limbs in baseX; the one from first blocks are in baseA and others are in baseB.
	LimbsX [4]ifaces.Column
	// lanes from first block in baseA, others in baseB
	LaneX ifaces.Column
	//  the ProverAction for [DecomposeBE]
	PA wizard.ProverAction
	// Size of the module
	Size int
}

// NewBlockBaseConversion declare the intermediate columns,
// and the constraints for changing the blocks in base uint64 into baseA/baseB.
// It also change the order of Bytes from Big-Endian to Little-Endian.
func NewBlockBaseConversion(comp *wizard.CompiledIOP,
	inp BlockBaseConversionInputs) *BlockBaseConversion {
	b := &BlockBaseConversion{
		Inputs: &inp,
		Size:   inp.Lane.Size(),
	}
	// declare the columns
	b.insertCommit(comp)

	// declare the constraints
	// - isFromFirstBlock is well formed
	// - base conversion via lookups
	b.csIsFromFirstBlock(comp)
	b.csBaseConversion(comp)
	return b
}

// it declares the native columns
func (b *BlockBaseConversion) insertCommit(comp *wizard.CompiledIOP) {
	createCol := common.CreateColFn(comp, BASE_CONVERSION, b.Size, pragmas.RightPadded)
	for j := range b.LimbsX {
		b.LimbsX[j] = createCol("LimbX_%v", j)
	}
	b.LaneX = createCol("LaneX")
	b.IsFromFirstBlock = createCol("IsFromFirstBlock")
	b.IsFromBlockBaseB = createCol("IsFromBlockBaseB")
}

// assign the columns specific to the submodule
func (b *BlockBaseConversion) Run(run *wizard.ProverRuntime) {
	b.assignIsFromFirstBlock(run)
	b.assignSlicesLaneX(run)
}

// the constraints over isFromFirstBlock
func (b *BlockBaseConversion) csIsFromFirstBlock(comp *wizard.CompiledIOP) {
	var (
		param             = generic.KeccakUsecase
		nbOfLanesPerBlock = param.NbOfLanesPerBlock()
	)

	// isFromFirstBlock = sum_j Shift(l.isFirstLaneFromNewHash,-j) for j:=0,...,
	s := sym.NewConstant(0)
	for j := 0; j < nbOfLanesPerBlock; j++ {
		s = sym.Add(
			s, column.Shift(b.Inputs.IsFirstLaneOfNewHash, -j),
		)
	}
	comp.InsertGlobal(0, ifaces.QueryIDf("IsFromFirstBlock"),
		sym.Sub(s, b.IsFromFirstBlock))

	commonconstraints.MustBeMutuallyExclusiveBinaryFlags(
		comp,
		b.Inputs.IsLaneActive,
		[]ifaces.Column{
			b.IsFromFirstBlock,
			b.IsFromBlockBaseB},
	)

}

// the constraints over base conversion
// - base conversion from uint64 into BaseA/BaseB
// - from Big-Endian to Little-Endian
func (b *BlockBaseConversion) csBaseConversion(comp *wizard.CompiledIOP) {
	// if isFromFirstBlock = 1  ---> convert to keccak.BaseA
	// otherwise convert to keccak.BaseB

	// first, decompose to limbs
	inp := DecompositionInputs{
		Name:          "LANE_DECOMPOSITION_BE",
		Col:           b.Inputs.Lane,
		NumLimbs:      numLimbsInput,
		BytesPerLimbs: LaneSizeByte / numLimbsInput}

	decomposed := DecomposeBE(comp, inp)
	b.PA = decomposed

	// reverse to go from big-endian to little-endian,
	// Note: reversing the bytes within the limb is done in the lookUp.
	slice := make([]ifaces.Column, len(decomposed.Limbs))
	copy(slice, decomposed.Limbs)
	slices.Reverse(slice)

	// base conversion limb by limb and via lookups; to go from uint64 to BaseA/BaseB.
	// Note: reversing the bytes within the limb is done in the lookUp.
	for j := range decomposed.Limbs {
		comp.InsertInclusionConditionalOnIncluded(0,
			ifaces.QueryIDf("BaseConversion_Into_BaseA_%v", j),
			[]ifaces.Column{b.Inputs.Lookup.ColUint16, b.Inputs.Lookup.ColBaseA},
			[]ifaces.Column{slice[j], b.LimbsX[j]},
			b.IsFromFirstBlock)

		comp.InsertInclusionConditionalOnIncluded(0,
			ifaces.QueryIDf("BaseConversion_Into_BaseB_%v", j),
			[]ifaces.Column{b.Inputs.Lookup.ColUint16, b.Inputs.Lookup.ColBaseB},
			[]ifaces.Column{slice[j], b.LimbsX[j]},
			b.IsFromBlockBaseB)
	}

	// recomposition of limbsX into laneX
	// build the base
	base := sym.Add(
		sym.Mul(b.IsFromFirstBlock, baseAPower16),
		sym.Mul(b.IsFromBlockBaseB, baseBPower16),
	)

	// reconstruct LaneX from limbsX
	laneX := sym.NewConstant(0)
	for k := range b.LimbsX {
		laneX = sym.Add(
			sym.Mul(base, laneX),
			b.LimbsX[k])
	}

	comp.InsertGlobal(0, ifaces.QueryIDf("Recompose_Lane_BaseX"),
		sym.Sub(laneX, b.LaneX),
	)

}

// assign column isFromFirstBlock
func (b *BlockBaseConversion) assignIsFromFirstBlock(run *wizard.ProverRuntime) {
	ones := vector.Repeat(field.One(), generic.KeccakUsecase.NbOfLanesPerBlock())
	var (
		size                 = b.Size
		isFirstLaneOfNewHash = b.Inputs.IsFirstLaneOfNewHash.GetColAssignment(run).IntoRegVecSaveAlloc()
		param                = generic.KeccakUsecase
		numLanesInBlock      = param.NbOfLanesPerBlock()
		isActive             = b.Inputs.IsLaneActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		col                  = common.NewVectorBuilder(b.IsFromFirstBlock)
		colB                 = common.NewVectorBuilder(b.IsFromBlockBaseB)
	)
	for j := 0; j < size; j++ {
		if isFirstLaneOfNewHash[j].IsOne() {
			col.PushSliceF(ones)
			j = j + (numLanesInBlock - 1)
		} else {
			col.PushInt(0)
		}
	}

	isNotFirstBlock := make([]field.Element, size)
	vector.Sub(isNotFirstBlock, isActive, col.Slice())
	colB.PushSliceF(isNotFirstBlock)

	col.PadAndAssign(run, field.Zero())
	colB.PadAndAssign(run, field.Zero())
}

// assign limbs for the base conversion;  limbX, laneX
func (b *BlockBaseConversion) assignSlicesLaneX(
	run *wizard.ProverRuntime) {
	var (
		isFirstBlock = b.IsFromFirstBlock.GetColAssignment(run).IntoRegVecSaveAlloc()
		lane         = b.Inputs.Lane.GetColAssignment(run).IntoRegVecSaveAlloc()
		laneX        = common.NewVectorBuilder(b.LaneX)
		limbX        = make([]*common.VectorBuilder, 4)
	)

	for j := range limbX {
		limbX[j] = common.NewVectorBuilder(b.LimbsX[j])
	}

	b.PA.Run(run)

	// populate laneX
	for j := range lane {
		res := lane[j].Bytes()
		bytes := make([]byte, 8)
		copy(bytes, res[24:])
		// to go from BE to LE; since keccak works with LE-BaseA and LE-BaseB
		slices.Reverse(bytes)

		if isFirstBlock[j].IsOne() {
			laneX.PushField(bytesToBaseX(bytes, &keccakf.BaseAFr))

			for k := range b.LimbsX {
				a := bytes[k*2 : k*2+2]
				limbX[k].PushField(bytesToBaseX(a, &keccakf.BaseAFr))
			}

		} else {
			laneX.PushField(bytesToBaseX(bytes, &keccakf.BaseBFr))

			for k := range b.LimbsX {
				a := bytes[k*2 : k*2+2]
				limbX[k].PushField(bytesToBaseX(a, &keccakf.BaseBFr))
			}
		}
	}

	// assign the laneX
	laneX.PadAndAssign(run, field.Zero())

	for k := range b.LimbsX {
		limbX[k].PadAndAssign(run, field.Zero())
	}
}

// Converts a slice of bytes to the filed.Element in a given base (little-Endian).
func bytesToBaseX(x []byte, base *field.Element) field.Element {

	res := field.Zero()
	one := field.One()
	resIsZero := true
	for j := 0; j < len(x); j++ {

		for k := 7; k >= 0; k-- {
			// The test allows skipping useless field muls or testing
			// the entire field element.
			if !resIsZero {
				res.Mul(&res, base)
			}

			// Skips the field addition if the bit is zero
			bit := (uint8(x[j]) >> k) & 1
			if bit > 0 {
				res.Add(&res, &one)
				resIsZero = false
			}
		}
	}

	return res
}
