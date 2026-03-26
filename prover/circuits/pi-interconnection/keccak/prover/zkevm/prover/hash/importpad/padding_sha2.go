package importpad

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
)

// Sha2Padder implements the [padder] interface for the SHA2 hash function.
type Sha2Padder struct {
	AccInsertedBytes ifaces.Column
}

// sha2PaddingAssignmentBuilder is a utility serving during the assignment of the
// sha2Padder module
type sha2PaddingAssignmentBuilder struct {
	AccInsertedBytes *common.VectorBuilder
}

// newSha2Padder declares all the constraints ensuring the imported byte strings
// are properly padded following the specification of Sha2.
func (ipad *Importation) newSha2Padder(comp *wizard.CompiledIOP) padder {

	// The padding structure is
	//
	// 	=> xxxxxxx 		|| 	size n 	bytes	||	isPadded:false || 	isLastPadded:false
	// 	=> 		1 		|| 	size 1 	bytes	||	isPadded:true  || 	isLastPadded:false
	// 	=>		0 		||	size n0	bytes	||	isPadded:true  || 	isLastPadded:false
	// 	=>		0 		||	size n1	bytes	||	isPadded:true  || 	isLastPadded:false
	// 	=>		..		|| 	size .. bytes	||	isPadded:true  || 	isLastPadded:false
	// 	=>		0 		||	size nk	bytes	||	isPadded:true  || 	isLastPadded:false
	// 	=> [msgSize] 	||	size 8	bytes	||	isPadded:true  || 	isLastPadded:true
	//
	// This is constrained as:
	//
	// 	- There is at least two consecutive padded limbs:
	//
	// 		isPadded[i] * IsBinary((isPadded[i-1]) + isPadded[i+1] - 1) == 0
	//			where IsBinary(x) = x^2 - x
	//
	//	- The first padded limb is equal to 1, and all intermediate ones are 0
	//
	//		isPadded[i] * (1 - isPadded[i-1]) * (nbBytes - 1) == 0
	//
	//  - The last padded byte is a uint64 so it has a nb of bytes of 8
	//
	//		isPadded[i] * (1 - isPadded[i+1]) * (nbBytes - 8) == 0
	//
	//  - In the middle, they have to be zero
	//
	//		isPadded[i] * isPadded[i+1] * isPadded[i-1] * nbBytes == 0
	//
	// - AccInsertedBytes is correctly set
	//
	//		if isInserted=0:
	// 			accInsertedBytes[i] = accInsertedBytes[i-1]
	//		if isInserted=1:
	//			// This also covers the initialization case
	//			accInsertedBytes[i] = isInserted[i-1] * accInsertedBytes[i-1] + nbBytes[i]
	//
	// - The padding values of limbs are correctly set
	//
	//		if isPadded[i] == 1:
	//			limbs[i] = Add(
	//				// NB: there is already a constraint ensuring that theses
	//				// two legs are incompatibles.
	//				isPadded[i-1] == 0 => 1
	//				isPadded[i+1] == 0 ==> accInsertedBytes[i]
	//			)
	//
	// - The number of number of padded values is within range
	//
	//		if isLastPadded[i] == 1:
	//			accPaddedBytes[i] in range 9 .. 72

	var (
		numRows = ipad.Limbs.Size()
		pad     = &Sha2Padder{
			AccInsertedBytes: comp.InsertCommit(0,
				ifaces.ColIDf("%v_SHA2_ACC_INSERTED_BYTES", ipad.Inputs.Name),
				numRows,
			),
		}
	)

	var (
		isInsertedPrev       = column.Shift(ipad.IsInserted, -1)
		isInserted           = ipad.IsInserted
		isPaddedPrev         = column.Shift(ipad.IsPadded, -1)
		isPadded             = ipad.IsPadded
		isPaddedNext         = column.Shift(ipad.IsPadded, 1)
		nbBytes              = ipad.NBytes
		accInsertedBytesPrev = column.Shift(pad.AccInsertedBytes, -1)
		accInsertedBytes     = pad.AccInsertedBytes
		isBinary             = func(x any) *sym.Expression {
			return sym.Sub(
				sym.Mul(x, x),
				x,
			)
		}
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_PADDING_AT_LEAST_TWO_LIMBS", ipad.Inputs.Name),
		sym.Mul(
			isPadded,
			isBinary(sym.Add(isPaddedPrev, isPaddedNext, -1)),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_FIRST_PADDING_HAS_1_BYTE", ipad.Inputs.Name),
		sym.Mul(
			isPadded,
			sym.Sub(1, isPaddedPrev),
			sym.Sub(nbBytes, 1),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_LAST_PADDING_HAS_8_BYTE", ipad.Inputs.Name),
		sym.Mul(
			isPadded,
			sym.Sub(1, isPaddedNext),
			sym.Sub(nbBytes, 8),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_INTERMEDIATE_PADDING_BYTES_ARE_ZEROES", ipad.Inputs.Name),
		sym.Mul(
			isPaddedPrev,
			isPadded,
			isPaddedNext,
			ipad.Limbs,
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_ACC_INSERTED_BYTES_CORRECTLY_SET", ipad.Inputs.Name),
		sym.Sub(
			accInsertedBytes,
			sym.Mul(isPadded, accInsertedBytesPrev),
			sym.Mul(isInserted, sym.Add(sym.Mul(isInsertedPrev, accInsertedBytesPrev), nbBytes)),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_PADDING_VALUES", ipad.Inputs.Name),
		sym.Mul(
			ipad.IsPadded,
			sym.Sub(
				ipad.Limbs,
				sym.Mul(
					sym.Sub(1, isPaddedPrev),
					leftAlign(0x80, 1), // The domain separation byte 0b10000000
				),
				sym.Mul(
					sym.Sub(1, isPaddedNext),
					accInsertedBytes,
					leftAlign(1, 8), // this ensures the value of accInsertBytes is dirtified.
					8,
				),
			),
		),
	)

	// This check that we do not pad by more than a whole block. It does not check
	// that this does exactly the right number of padded bytes. This is OK since
	// later stages of the packing check that the padded byte string has a length
	// divisible by the blocksize.
	//
	// The +8 is to account for the last 64 bits string length. In theory it
	// should be restricted to only 9..72 but the above constraints ensure that
	// that at least the first and the last limbs are included.
	comp.InsertInclusionConditionalOnIncluded(0,
		ifaces.QueryIDf("%v_LOOKUP_NB_PADDED_BYTES", ipad.Inputs.Name),
		[]ifaces.Column{getLookupForSize(comp, 8+generic.KeccakUsecase.BlockSizeBytes())},
		[]ifaces.Column{ipad.AccPaddedBytes},
		ipad.IsPadded,
	)

	return pad
}

func (sp *Sha2Padder) pushPaddingRows(byteStringSize int, ipad *importationAssignmentBuilder) {

	var (
		blocksize      = generic.Sha2Usecase.BlockSizeBytes()
		remainToPad    = blocksize - (byteStringSize % blocksize)
		spa            = ipad.Padder.(*sha2PaddingAssignmentBuilder)
		accPaddedBytes = 0
	)

	if remainToPad < 9 {
		remainToPad += 64
	}

	accPaddedBytes++
	remainToPad--

	ipad.pushPaddingCommonColumns()
	ipad.Limbs.PushField(leftAlign(0x80, 1))
	ipad.NBytes.PushOne()
	ipad.AccPaddedBytes.PushOne()
	spa.AccInsertedBytes.PushInt(byteStringSize)

	for remainToPad > 8 {
		currNbBytes := utils.Min(remainToPad-8, 16)
		accPaddedBytes += currNbBytes
		remainToPad -= currNbBytes

		ipad.pushPaddingCommonColumns()
		ipad.Limbs.PushZero()
		ipad.NBytes.PushInt(currNbBytes)
		ipad.AccPaddedBytes.PushInt(accPaddedBytes)
		spa.AccInsertedBytes.PushInt(byteStringSize)
	}

	accPaddedBytes += 8

	ipad.pushPaddingCommonColumns()
	ipad.Limbs.PushField(leftAlign(uint64(byteStringSize)*8, 8))
	ipad.NBytes.PushInt(8)
	ipad.AccPaddedBytes.PushInt(accPaddedBytes)
	spa.AccInsertedBytes.PushInt(byteStringSize)
}

func (spa *sha2PaddingAssignmentBuilder) pushInsertingRow(nbBytes int, isNewHash bool) {
	if isNewHash {
		spa.AccInsertedBytes.PushInt(nbBytes)
	} else {
		spa.AccInsertedBytes.PushIncBy(nbBytes)
	}
}

func (spa *sha2PaddingAssignmentBuilder) padAndAssign(run *wizard.ProverRuntime) {
	spa.AccInsertedBytes.PadAndAssign(run, field.Zero())
}
