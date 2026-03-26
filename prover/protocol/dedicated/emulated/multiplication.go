package emulated

import (
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/bigrange"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

// MultiplicationAssignmentProverAction represents an emulated big integer multiplication with given modulus.
type MultiplicationAssignmentProverAction struct {
	// NbBitsPerLimb is the number of bits per limb.
	NbBitsPerLimb int
	// Round is the module Round. It is automatically computed at creation from
	// the rounds of the input columns.
	Round int
	// Name is the module Name, used for column and query ids.
	Name string

	// TermL and TermR are the multiplicands.
	TermL, TermR limbs.Limbs[limbs.LittleEndian]
	// Modulus is the modulus for the multiplication.
	Modulus limbs.Limbs[limbs.LittleEndian]

	// Result is the result of the multiplication modulo Modulus. Computed by the module
	// at prover time.
	Result limbs.Limbs[limbs.LittleEndian]
	// Quotient is the quotient of the multiplication. Computed by the module at prover time.
	Quotient limbs.Limbs[limbs.LittleEndian]
	// Carry are the carries used during the multiplication. Computed by the module at prover time.
	Carry limbs.Limbs[limbs.LittleEndian]

	// Challenge is the random challenge used for the random polynomial evaluation.
	Challenge *coin.Info
}

// NewMul creates a new emulated multiplication module that computes
//
//	left * right = result (mod modulus)
//
// We assume that the inputs are already given limb-wise in little-endian order
// (smalles limb first) and that the limbs are already range-checked to be
// within [0, 2^nbBitsPerLimb).
//
// The module computes the result and quotient limbs at prover time and assigns
// them to the respective columns automatically.
//
// The returned multiplication module can be used to reference the computed
// auxiliary columns.
func NewMul(comp *wizard.CompiledIOP, name string, left, right, modulus limbs.Limbs[limbs.LittleEndian], nbBitsPerLimb int) *MultiplicationAssignmentProverAction {
	// XXX(ivokub): add option to have activator column. When it is given then we can
	// avoid assigning zeros when the multiplication is not active
	nbRows := left.NumRow()
	// compute the range checking parameters
	nbRangecheckBits := 16
	nbRangecheckLimbs := (nbBitsPerLimb + nbRangecheckBits - 1) / nbRangecheckBits
	// compute the minimal round needed
	round := 0
	for _, l := range left.GetLimbs() {
		round = max(round, l.Round())
	}
	for _, l := range right.GetLimbs() {
		round = max(round, l.Round())
	}
	for _, l := range modulus.GetLimbs() {
		round = max(round, l.Round())
	}

	// compute the number of limbs needed for storing the quotient.
	nbQuoBits := nbMultiplicationResLimbs(left.NumLimbs()*nbBitsPerLimb, right.NumLimbs()*nbBitsPerLimb)
	nbQuoBits += 1 // for possible carry
	// compute the number of carry bits needed
	nbCarryBits := nbQuoBits
	nbQuoBits = max(0, nbQuoBits-modulus.NumLimbs()*nbBitsPerLimb+1) // we divide by modulus of nbLimbs size
	nbQuoLimbs := utils.DivCeil(nbQuoBits, nbBitsPerLimb)
	result := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EMUL_REMAINDER_LIMB", name), modulus.NumLimbs(), nbRows)
	quotient := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EMUL_QUOTIENT_LIMB", name), nbQuoLimbs, nbRows)
	nbCarryLimbs := utils.DivCeil(nbCarryBits, nbBitsPerLimb)
	carry := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EMUL_CARRY", name), nbCarryLimbs, nbRows)
	// create the challenge which will be used in the next round for random poly eval.
	challenge := comp.InsertCoin(round+1, coin.Namef("%s_EMUL_CHALLENGE", name), coin.FieldExt)

	proverAction := &MultiplicationAssignmentProverAction{
		TermL:         left,
		TermR:         right,
		Modulus:       modulus,
		Result:        result,
		Quotient:      quotient,
		Carry:         carry,
		Challenge:     &challenge,
		NbBitsPerLimb: nbBitsPerLimb,
		Round:         round,
		Name:          name,
	}

	// we need to register prover action already here to ensure it is called
	// before bigrange prover actions
	comp.RegisterProverAction(round, proverAction)

	// range check the result and quotient limbs to be within bounds
	for i, l := range quotient.GetLimbs() {
		bigrange.BigRange(
			comp,
			ifaces.ColumnAsVariable(l), int(nbRangecheckLimbs), nbRangecheckBits,
			fmt.Sprintf("%s_EMUL_QUOTIENT_LIMB_RANGE_%d", name, i),
		)
	}
	for i, l := range result.GetLimbs() {
		bigrange.BigRange(
			comp,
			ifaces.ColumnAsVariable(l), int(nbRangecheckLimbs), nbRangecheckBits,
			fmt.Sprintf("%s_EMUL_REMAINDER_LIMB_RANGE_%d", name, i),
		)
	}

	// define the global constraints

	// first we define constraints which ensure the multiplication is correctly defined
	proverAction.csMultiplication(comp)

	return proverAction
}

func (cs *MultiplicationAssignmentProverAction) csMultiplication(comp *wizard.CompiledIOP) {
	// checks the correctness of the multiplication check:
	//
	//  left(x) * right(x) = modulus(x) * quotient(x) + result(x) + carry(x) * (2^nbBitsPerLimb - challenge)
	//
	// where left, right, modulus, quotient, result, carry are the polynomials defined by
	// the respective limbs and challenge is the random challenge used for polynomial evaluation.

	// left(x) and right(x)
	leftEval := csPolyEval(cs.TermL, cs.Challenge)
	rightEval := csPolyEval(cs.TermR, cs.Challenge)

	// modulus(x) and quotient(x)
	modulusEval := csPolyEval(cs.Modulus, cs.Challenge)
	quotientEval := csPolyEval(cs.Quotient, cs.Challenge)

	// result(x)
	resultEval := csPolyEval(cs.Result, cs.Challenge)

	// carry(x)
	carryEval := csPolyEval(cs.Carry, cs.Challenge)
	// compute (2^nbBitsPerLimb - challenge)
	coef := big.NewInt(0).Lsh(big.NewInt(1), uint(cs.NbBitsPerLimb))
	carryCoef := symbolic.Sub(
		symbolic.NewConstant(coef),
		cs.Challenge.AsVariable(),
	)

	// left(x) * right(x)
	mulEval := symbolic.Mul(leftEval, rightEval)
	// carry(x) * (2^nbits - challenge)
	carryCoefEval := symbolic.Mul(carryEval, carryCoef)
	// modulus(x) * quotient(x)
	qmEval := symbolic.Mul(modulusEval, quotientEval)

	// we enforce that
	//
	//  left(x) * right(x) - modulus(x) * quotient(x) - result(x) - carry(x) * (2^nbBitsPerLimb - challenge) = 0
	//
	// in the next round (due to using the challenge)
	comp.InsertGlobal(
		cs.Round+1,
		ifaces.QueryIDf("%s_EMUL_MULTIPLICATION", cs.Name),
		symbolic.Sub(
			mulEval,
			qmEval,
			resultEval,
			carryCoefEval,
		),
	)
}
