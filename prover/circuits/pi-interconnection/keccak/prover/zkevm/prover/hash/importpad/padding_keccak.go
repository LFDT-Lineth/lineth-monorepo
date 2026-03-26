package importpad

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
)

// KeccakPadder implements the [padder] interface. The struct is empty mainly
// because it does not need to create extra columns.
type KeccakPadder struct{}

// newKeccakPadder declare all the constraints ensuring the imported byte strings
// are properly padded following the spec of the keccak hash function.
func (iPadd *Importation) newKeccakPadder(comp *wizard.CompiledIOP) padder {

	// 		if isPadded[i-1] = 0, isPadded[i] = 1, isPadded[i+1] =1 ----> limb = 1, nByte = 1
	// 		if isPadded[i-1] = 1, isPadded[i] = 1, isPadded[i+1] =0 ----> limb = 128, nByte = 1
	// 		if isPadded[i-1] = 0, isPadded[i] = 1, isPadded[i+1] =0 ----> limb = 129 , nByte = 1
	// 		if isPadded[i-1] = 1, isPadded[i] = 1, isPadded[i+1] =1 ----> limb = 0, nByte < 16
	//  the constraints over NBytes also guarantees the correct number of  padded zeroes.

	var (
		dsv = sym.NewConstant(leftAlign(1, 1))   // domain separator value, for padding
		fpv = sym.NewConstant(leftAlign(128, 1)) // final padding value
	)

	var (
		isPadded     = iPadd.IsPadded
		isPaddedPrev = column.Shift(isPadded, -1)
		isPaddedNext = column.Shift(isPadded, 1)
		limb         = iPadd.Limbs
		nBytes       = iPadd.NBytes
	)

	// This single constraint cover all the cases for the padding value. And can
	// be translated as
	//
	// if isPadded = 0; then
	//		limb = 	{ IF (isPadded[i-1] == 0) THEN 1 	ELSE 0} +
	//    			{ IF (isPadded[i+1] == 0) THEN 128 	ELSE 0}
	//
	// NB: this only works because we are guaranteed by all the callers that
	// the empty string cannot exists in the imported data.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_KECCAK_PADDING_VALUE", iPadd.Inputs.Name),
		sym.Mul(
			isPadded,
			sym.Sub(
				limb,
				sym.Mul(
					sym.Sub(1, isPaddedPrev),
					dsv,
				),
				sym.Mul(
					sym.Sub(1, isPaddedNext),
					fpv,
				),
			),
		),
	)

	// This constraints ensures that that whenever we are looking a in the DSV,
	// FPV or FPV + DSV, then the value of nBytes should be 1.
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_DOMAIN_SEPARATOR_NBYTE", iPadd.Inputs.Name),
		sym.Mul(
			isPadded,
			sym.Sub(2, isPaddedPrev, isPaddedNext),
			sym.Sub(nBytes, 1),
		),
	)

	// This check that we do not pad by more than a whole block. It does not check
	// that this does exactly the right number of padded bytes. This is OK since
	// later stages of the packing check that the padded byte string has a length
	// divisible by the blocksize.
	comp.InsertInclusionConditionalOnIncluded(0,
		ifaces.QueryIDf("%v_LOOKUP_NB_PADDED_BYTES", iPadd.Inputs.Name),
		[]ifaces.Column{getLookupForSize(comp, generic.KeccakUsecase.BlockSizeBytes())},
		[]ifaces.Column{iPadd.AccPaddedBytes},
		iPadd.IsPadded,
	)

	return KeccakPadder{}
}

// pushPaddingRows pushes the padding rows corresponding to a plaintext of
// size byteStringSize.
func (kp KeccakPadder) pushPaddingRows(byteStringSize int, iPadd *importationAssignmentBuilder) {

	var (
		blocksize   = generic.KeccakUsecase.BlockSizeBytes()
		remainToPad = blocksize - (byteStringSize % blocksize)
	)

	if remainToPad == 0 {
		remainToPad = generic.KeccakUsecase.BlockSizeBytes()
	}

	if remainToPad == 1 {
		iPadd.pushPaddingCommonColumns()
		iPadd.Limbs.PushField(leftAlign(129, 1))
		iPadd.NBytes.PushOne()
		iPadd.AccPaddedBytes.PushOne()
		return
	}

	iPadd.pushPaddingCommonColumns()
	iPadd.Limbs.PushField(leftAlign(1, 1))
	iPadd.NBytes.PushOne()
	iPadd.AccPaddedBytes.PushOne()
	remainToPad--

	for remainToPad > 1 {
		currNbBytes := utils.Min(remainToPad-1, 16)
		remainToPad -= currNbBytes
		iPadd.pushPaddingCommonColumns()
		iPadd.Limbs.PushZero()
		iPadd.NBytes.PushInt(currNbBytes)
		iPadd.AccPaddedBytes.PushIncBy(currNbBytes)
	}

	iPadd.pushPaddingCommonColumns()
	iPadd.Limbs.PushField(leftAlign(128, 1))
	iPadd.NBytes.PushOne()
	iPadd.AccPaddedBytes.PushInc()
}

// keccakPadderAssignmentBuilder implements the [padderAssignmentBuilder]. The
// struct is the empty struct as this padder does not requires adding any column
// in the module.
type keccakPadderAssignmentBuilder struct{}

func (kp keccakPadderAssignmentBuilder) pushInsertingRow(int, bool) {}

func (kp keccakPadderAssignmentBuilder) padAndAssign(*wizard.ProverRuntime) {}
