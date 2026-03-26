package importpad

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// sha2LengthFieldSizeBits is the number of bytes used to encode the message
// length in SHA-256 padding.
const sha2LengthFieldSizeBytes = 8

// sha2LengthFieldSizeBits is the number of bits used to encode the message
// length in SHA-256 padding.
const sha2LengthFieldSizeBits = sha2LengthFieldSizeBytes * 8

// Sha2Padder implements the [padder] interface for the SHA2 hash function.
type Sha2Padder struct {
	// AccInsertedBits contains the number of bits to hash, excluding the padding.
	// According to the SHA-256 specification, it is a 64-bit unsigned integer, so,
	// in case of small fields, it should be divided into several columns.
	AccInsertedBits byte32cmp.LimbColumns
	// accInsertedPlusNbBits is a set of constraints to ensure that the
	// AccInsertedBits = AccInsertedBits[i-1] + nbBits[i] in general case.
	AccInsertedPlusNbBits wizard.ProverAction
	// Contains the number of bits (nbBytes*8) inserted by each limb.
	NbBits ifaces.Column
}

func (sp *Sha2Padder) newBuilder() padderAssignmentBuilder {
	accInsertedBits := make([]*common.VectorBuilder, len(sp.AccInsertedBits.Limbs))

	for i, col := range sp.AccInsertedBits.Limbs {
		accInsertedBits[i] = common.NewVectorBuilder(col)
	}

	return &sha2PaddingAssignmentBuilder{
		AccInsertedBits:       accInsertedBits,
		prevAccInsertedBits:   0,
		accInsertedPlusNbBits: sp.AccInsertedPlusNbBits,
		nbBits:                common.NewVectorBuilder(sp.NbBits),
	}
}

// sha2PaddingAssignmentBuilder is a utility serving during the assignment of the
// sha2Padder module
type sha2PaddingAssignmentBuilder struct {
	AccInsertedBits []*common.VectorBuilder
	// prevAccInsertedBits contains value of the previous set AccInsertedBits
	prevAccInsertedBits   int
	accInsertedPlusNbBits wizard.ProverAction
	nbBits                *common.VectorBuilder
}

// newSha2Padder declares all the constraints ensuring the imported byte strings
// are properly padded following the specification of Sha2.
func (ipad *Importation) newSha2Padder(comp *wizard.CompiledIOP) padder {

	// The padding structure is
	//
	// 	=> xxxxxxx    ||  size n  bytes  ||  isPadded:false  ||  isLastPadded:false
	// 	=> 		1 		  ||  size 1  bytes  ||  isPadded:true   ||  isLastPadded:false
	// 	=>		0 		  ||  size n0 bytes  ||  isPadded:true   ||  isLastPadded:false
	// 	=>		0 		  ||  size n1 bytes  ||  isPadded:true   ||  isLastPadded:false
	// 	=>		..		  ||  size .. bytes  ||  isPadded:true   ||  isLastPadded:false
	// 	=>		0 		  ||  size nk bytes  ||  isPadded:true   ||  isLastPadded:false
	// 	=> [msgSize]  ||  size 8  bytes  ||  isPadded:true   ||  isLastPadded:true
	//
	// This is constrained as:
	//
	//  - The nbBits must be nbBytes * 8
	//
	//    isInserted[i] * (nbBits - nbBytes * 8) == 0
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
	// - AccInsertedBits is correctly set
	//
	//    // considering that accInsertedBits is represented by 4 limbs:
	//		if isInserted=0:
	// 			accInsertedBits[i] = accInsertedBits[i-1]
	//		if isInserted=1:
	//			// This also covers the initialization case
	//			accInsertedBits[i] = isInserted[i-1] * accInsertedBits[i-1] + nbBits[i]
	//
	// - The padding values of limbs are correctly set
	//
	//		if isPadded[i] == 1:
	//			limbs[i] = Add(
	//				// NB: there is already a constraint ensuring that theses
	//				// two legs are incompatibles.
	//				isPadded[i-1] == 0 => 1
	//				isPadded[i+1] == 0 ==> accInsertedBits[i]
	//			)
	//
	// - The number of number of padded values is within range
	//
	//		if isLastPadded[i] == 1:
	//			accPaddedBytes[i] in range 9 .. 72

	var (
		numRows = ipad.Limbs[0].Size()
		nbLimbs = len(ipad.Limbs)
		// Take the limb column size as a max size for the AccInsertedBits columns.
		maxLimbsSizeBits = totalLimbBits / nbLimbs
		// The number of columns needed to store the AccInsertedBits (64 bits in total).
		accInsertedBitsNbLimbs = utils.DivCeil(sha2LengthFieldSizeBits, maxLimbsSizeBits)
		pad                    = &Sha2Padder{
			NbBits: comp.InsertCommit(0, ifaces.ColIDf("%v_SHA2_NB_BITS", ipad.Inputs.Name), numRows, true),
			AccInsertedBits: byte32cmp.LimbColumns{
				Limbs:       make([]ifaces.Column, accInsertedBitsNbLimbs),
				LimbBitSize: maxLimbsSizeBits,
				IsBigEndian: true,
			},
		}
	)

	for i := 0; i < accInsertedBitsNbLimbs; i++ {
		pad.AccInsertedBits.Limbs[i] = comp.InsertCommit(0,
			ifaces.ColIDf("%v_SHA2_ACC_INSERTED_BITS_%d", ipad.Inputs.Name, i),
			numRows,
			true,
		)
	}

	var (
		isInsertedPrev      = column.Shift(ipad.IsInserted, -1)
		isInserted          = ipad.IsInserted
		isPaddedPrev        = column.Shift(ipad.IsPadded, -1)
		isPadded            = ipad.IsPadded
		isPaddedNext        = column.Shift(ipad.IsPadded, 1)
		nbBytes             = ipad.NBytes
		nbBits              = pad.NbBits
		accInsertedBitsPrev = pad.AccInsertedBits.Shift(-1).Limbs
		accInsertedBits     = pad.AccInsertedBits.Limbs
		isBinary            = func(x any) *sym.Expression {
			return sym.Sub(
				sym.Mul(x, x),
				x,
			)
		}
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_NBBYTES_NBBITS_CONSISTENCY", ipad.Inputs.Name),
		sym.Mul(isInserted, sym.Sub(nbBits, sym.Mul(nbBytes, 8))),
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

	for i := 0; i < len(ipad.Limbs); i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_INTERMEDIATE_PADDING_BYTES_ARE_ZEROES_%d", ipad.Inputs.Name, i),
			sym.Mul(
				isPaddedPrev,
				isPadded,
				isPaddedNext,
				ipad.Limbs[i],
			),
		)
	}

	// If it is padded row, then the accInsertedBits should be equal to the previous one.
	for i := 0; i < len(accInsertedBits); i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_ACC_INSERTED_BITS_CORRECTLY_SET_PADDED_%d", ipad.Inputs.Name, i),
			sym.Mul(
				isPadded,
				sym.Sub(accInsertedBits[i], accInsertedBitsPrev[i]),
			),
		)
	}

	// If it is first inserted row, then the less significant limb of accInsertedBits
	// should be equal to nbBits, while other limbs are zeroes.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_ACC_INSERTED_BITS_CORRECTLY_SET_%d", ipad.Inputs.Name, len(accInsertedBits)-1),
		sym.Mul(
			isInserted,
			sym.Sub(1, isInsertedPrev),
			sym.Sub(accInsertedBits[len(accInsertedBits)-1], nbBits),
		),
		true, // To cover the first row as well
	)

	// If it is first inserted row, then the less significant limb of accInsertedBits
	// should be equal to nbBits, while other top limbs are zeroes.
	for i := 0; i < len(accInsertedBits)-1; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_ACC_INSERTED_BITS_CORRECTLY_SET_%d", ipad.Inputs.Name, i),
			sym.Mul(isInserted, sym.Sub(1, isInsertedPrev), accInsertedBits[i]),
		)
	}

	_, pad.AccInsertedPlusNbBits = byte32cmp.NewMultiLimbAdd(comp, &byte32cmp.MultiLimbAddIn{
		Name:   fmt.Sprintf("%v_SHA2_ACC_INSERTED_BITS_PLUS_NB_BITS", ipad.Inputs.Name),
		ALimbs: pad.AccInsertedBits.Shift(-1),
		BLimbs: byte32cmp.LimbColumns{
			Limbs:       []ifaces.Column{nbBits},
			LimbBitSize: pad.AccInsertedBits.LimbBitSize,
			IsBigEndian: pad.AccInsertedBits.IsBigEndian,
		},
		Result:        pad.AccInsertedBits,
		Mask:          sym.Mul(isInserted, isInsertedPrev), // All inserted rows, except the first one
		NoBoundCancel: true,
	}, true)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SHA2_FIRST_PADDING_VALUE", ipad.Inputs.Name),
		sym.Mul(
			ipad.IsPadded,
			sym.Sub(1, isPaddedPrev),
			sym.Sub(
				// Only the first limb is used for the first byte
				ipad.Limbs[0],
				leftAlignLimb(0x80, 1, nbLimbs), // The domain separation byte 0b10000000
			),
		),
	)

	// Only first limb is used for the first byte, so the other limbs must be zeroes
	for i := 1; i < nbLimbs; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_FIRST_PADDING_VALUE_ZEROES_%d", ipad.Inputs.Name, i),
			sym.Mul(
				ipad.IsPadded,
				sym.Sub(1, isPaddedPrev),
				ipad.Limbs[i],
			),
		)
	}

	for i := 0; i < accInsertedBitsNbLimbs; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_LAST_PADDING_VALUE_%d", ipad.Inputs.Name, i),
			sym.Mul(
				ipad.IsPadded,
				sym.Sub(1, isPaddedNext),
				sym.Sub(ipad.Limbs[i], accInsertedBits[i]),
			),
		)
	}

	for i := accInsertedBitsNbLimbs; i < nbLimbs; i++ {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SHA2_LAST_PADDING_VALUE_ZEROES_%d", ipad.Inputs.Name, i),
			sym.Mul(ipad.IsPadded, sym.Sub(1, isPaddedNext), ipad.Limbs[i]),
		)
	}

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
		[]ifaces.Column{getLookupForSize(comp, 8+generic.Sha2Usecase.BlockSizeBytes())},
		[]ifaces.Column{ipad.AccPaddedBytes},
		ipad.IsPadded,
	)

	return pad
}

func (sp *Sha2Padder) pushPaddingRows(byteStringSize int, ipad *importationAssignmentBuilder) {

	var (
		nbLimbs        = len(ipad.Limbs)
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
	ipad.Limbs[0].PushField(leftAlignLimb(0x80, 1, nbLimbs))
	for i := 1; i < nbLimbs; i++ {
		ipad.Limbs[i].PushZero()
	}
	ipad.NBytes.PushOne()
	spa.nbBits.PushInt(8)
	ipad.AccPaddedBytes.PushOne()

	accInsertedBits := spa.leftAlignAccInsertedBits(byteStringSize * 8)
	spa.pushAccInsertedBits(accInsertedBits)

	for remainToPad > 8 {
		currNbBytes := utils.Min(remainToPad-8, 16)
		accPaddedBytes += currNbBytes
		remainToPad -= currNbBytes

		ipad.pushPaddingCommonColumns()

		for i := 0; i < nbLimbs; i++ {
			ipad.Limbs[i].PushZero()
		}

		ipad.NBytes.PushInt(currNbBytes)
		spa.nbBits.PushInt(currNbBytes * 8)
		ipad.AccPaddedBytes.PushInt(accPaddedBytes)
		spa.pushAccInsertedBits(accInsertedBits)
	}

	accPaddedBytes += 8

	ipad.pushPaddingCommonColumns()

	limbs := leftAlign(uint64(byteStringSize)*8, 8, generic.TotalLimbSize, nbLimbs)
	for i, limb := range limbs {
		ipad.Limbs[i].PushField(limb)
	}
	ipad.NBytes.PushInt(8)
	spa.nbBits.PushInt(64)
	ipad.AccPaddedBytes.PushInt(accPaddedBytes)
	spa.pushAccInsertedBits(accInsertedBits)
}

func (spa *sha2PaddingAssignmentBuilder) pushInsertingRow(nbBits int, isNewHash bool) {
	spa.nbBits.PushInt(nbBits)

	if !isNewHash {
		nbBits += spa.prevAccInsertedBits
	}

	spa.prevAccInsertedBits = nbBits

	spa.pushAccInsertedBits(spa.leftAlignAccInsertedBits(nbBits))
}

func (spa *sha2PaddingAssignmentBuilder) padAndAssign(run *wizard.ProverRuntime) {
	for i := range spa.AccInsertedBits {
		spa.AccInsertedBits[i].PadAndAssign(run, field.Zero())
	}

	spa.nbBits.PadAndAssign(run, field.Zero())
	spa.accInsertedPlusNbBits.Run(run)
}

func (spa *sha2PaddingAssignmentBuilder) pushAccInsertedBits(laAccInsertedBits []field.Element) {
	for i, limb := range laAccInsertedBits {
		spa.AccInsertedBits[i].PushField(limb)
	}
}

func (spa *sha2PaddingAssignmentBuilder) leftAlignAccInsertedBits(nbBits int) []field.Element {
	return leftAlign(
		uint64(nbBits),
		8,
		utils.Max(generic.TotalLimbSize/len(spa.AccInsertedBits), sha2LengthFieldSizeBytes),
		len(spa.AccInsertedBits),
	)
}
