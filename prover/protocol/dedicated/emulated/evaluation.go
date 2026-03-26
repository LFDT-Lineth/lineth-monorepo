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

// AssignEmulatedColumnsProverAction represents an emulated polynomial evaluation module
// that computes
//
//	\sum_i Coeffs[i] * \prod_j Terms[i][j] = modulus * quotient + carry * (2^nbBitsPerLimb - challenge)
//
// where all Terms[i][j], modulus, quotient, and carry are given limb-wise
// in little-endian order (smallest limb first).
// The module computes the quotient and carry limbs at prover time
// and assigns them to the respective columns automatically.
//
// The returned evaluation module can be used to reference the computed
// auxiliary columns.
type AssignEmulatedColumnsProverAction struct {
	// NbBitsPerLimb is the number of bits per limb
	NbBitsPerLimb int
	// Round is the maximum Round number of the input columns
	Round int
	// Name of the module
	Name string
	// MaxTermDegree is the maximum degree of all terms
	MaxTermDegree int
	// NbLimbs is the maximum number of limbs seen over terms and modulus
	NbLimbs int

	// Terms are the evaluation terms
	// such that \sum_i Coeffs[i] * \prod_j Terms[i][j] == 0
	Terms [][]limbs.Limbs[limbs.LittleEndian]
	// Coeffs are the integer coefficients for each term. Must be +1 or -1.
	Coeffs []int
	// Modulus is the modulus for the evaluation
	Modulus limbs.Limbs[limbs.LittleEndian]

	// Quotient is the computed quotient limbs
	Quotient limbs.Limbs[limbs.LittleEndian]
	// Carry are the computed carry limbs
	Carry limbs.Limbs[limbs.LittleEndian]

	// Challenge is the random challenge used for the polynomial evaluation
	Challenge *coin.Info
}

// NewEval creates a new emulated evaluation module that computes
//
//	\sum_i coeffs[i] * \prod_j Terms[i][j] = modulus * quotient + carry * (2^nbBitsPerLimb - challenge)
//
// We assume that the inputs are already given limb-wise in little-endian order
// (smallest limb first) and that the limbs are already range-checked to be
// within [0, 2^nbBitsPerLimb).
//
// The coefficients must be +1 or -1. The terms must be arranged such that the
// resulting quotient is non-negative.
//
// The module computes the quotient and carry limbs at prover time and assigns
// them to the respective columns automatically.
//
// The returned evaluation module can be used to reference the computed
// auxiliary columns.
//
// NB! We internally compute the full integer multiplication c[i+j] = a[i] *
// b[j] recursively. So the bitlength of c can be bitlength(a) + bitlength(b)-1.
// This means that every term degree increases the bitlength of the result by
// nbBitsPerLimb.
//
// It is safe to use have max term degree 3 (in case of 16-bit limbs) when the
// number of limbs is not more than 512. Higher degree also works, but only if
// one of the terms has significantly smaller bitlength (i.e. it is a selector
// or a small constant).
func NewEval(comp *wizard.CompiledIOP, name string, nbBitsPerLimb int, modulus limbs.Limbs[limbs.LittleEndian], terms [][]limbs.Limbs[limbs.LittleEndian], coeffs []int) *AssignEmulatedColumnsProverAction {
	if len(coeffs) != len(terms) {
		utils.Panic("coefficients length %d does not match terms length %d", len(coeffs), len(terms))
	}
	for i, c := range coeffs {
		if c != 1 && c != -1 {
			utils.Panic("coefficient %d at index %d must be +1 or -1", c, i)
		}
	}
	round := 0
	nbRows := modulus.NumRow()
	maxTermDegree := 0
	nbLimbs := modulus.NumLimbs()
	nbRangecheckBits := 16
	nbRangecheckLimbs := (nbBitsPerLimb + nbRangecheckBits - 1) / nbRangecheckBits
	for i := range terms {
		maxTermDegree = max(maxTermDegree, len(terms[i]))
		for j := range terms[i] {
			for _, l := range terms[i][j].GetLimbs() {
				round = max(round, l.Round())
			}
		}
	}
	nbQuoBits := 0
	for i := range terms {
		nbTermQuoLimbsBits := terms[i][0].NumLimbs() * nbBitsPerLimb
		for j := 1; j < len(terms[i]); j++ {
			nbTermQuoLimbsBits = nbMultiplicationResLimbs(nbTermQuoLimbsBits, terms[i][j].NumLimbs()*nbBitsPerLimb)
			nbLimbs = max(nbLimbs, terms[i][j].NumLimbs())
		}
		nbQuoBits = max(nbQuoBits, nbTermQuoLimbsBits)
	}
	nbQuoBits += utils.Log2Ceil(len(terms)) // add some slack for the addition of terms
	nbCarryBits := nbQuoBits
	nbQuoBits = max(0, nbQuoBits-modulus.NumLimbs()*nbBitsPerLimb+1) // we divide by modulus of nbLimbs size
	nbQuoLimbs := utils.DivCeil(nbQuoBits, nbBitsPerLimb)
	for _, l := range modulus.GetLimbs() {
		round = max(round, l.Round())
	}

	quotient := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EMUL_EVAL_QUO_LIMB", name), nbQuoLimbs, nbRows)
	nbCarryLimbs := utils.DivCeil(nbCarryBits, nbBitsPerLimb)
	carry := limbs.NewLimbs[limbs.LittleEndian](comp, ifaces.ColIDf("%s_EMUL_EVAL_CARRY_LIMB", name), nbCarryLimbs, nbRows)
	// define the challenge and challenge powers for polynomial evaluation at random point
	challenge := comp.InsertCoin(round+1, coin.Namef("%s_EMUL_CHALLENGE", name), coin.FieldExt)

	proverAction := &AssignEmulatedColumnsProverAction{
		Terms:         terms,
		Coeffs:        coeffs,
		Modulus:       modulus,
		Quotient:      quotient,
		Carry:         carry,
		Challenge:     &challenge,
		NbBitsPerLimb: nbBitsPerLimb,
		Round:         round,
		Name:          name,
		MaxTermDegree: maxTermDegree,
		NbLimbs:       nbLimbs,
	}

	// we need to register the prover actions for assigning the emulated columns before doing range checks
	// to ensure the values are available (prover action is FIFO)
	comp.RegisterProverAction(round, proverAction)

	// range check the quotient limbs
	for i, l := range quotient.GetLimbs() {
		bigrange.BigRange(
			comp,
			ifaces.ColumnAsVariable(l), int(nbRangecheckLimbs), nbRangecheckBits,
			fmt.Sprintf("%s_EMUL_QUOTIENT_LIMB_RANGE_%d", name, i),
		)
	}

	// define the constraints for the emulated evaluation
	proverAction.csEval(comp)
	return proverAction
}

func (cs *AssignEmulatedColumnsProverAction) csEval(comp *wizard.CompiledIOP) {
	// TODO(ivokub): should we write the evaluation results in a limb?

	// this method computes the following constraint:
	//
	//  \sum_i Coeffs[i] * \prod_j Terms[i][j](x) - modulus(x) * quotient(x) - carry(x) * (2^nbBitsPerLimb - challenge) = 0

	// we first compute all unique limb polynomials. This is to ensure that
	// if the same limb appears multiple times in the terms, we only compute
	// its polynomial evaluation once.
	uniqueLimbs := make(map[string]*symbolic.Expression)
	for i := range cs.Terms {
		for j := range cs.Terms[i] {
			name := cs.Terms[i][j].String()
			if _, ok := uniqueLimbs[name]; !ok {
				uniqueLimbs[name] = csPolyEval(cs.Terms[i][j], cs.Challenge)
			}
		}
	}

	// compute the evaluation sum
	//
	//  \sum_i Coeffs[i] * \prod_j Terms[i][j](x)
	termProds := make([]*symbolic.Expression, len(cs.Terms))
	for i := range cs.Terms {
		// compute \prod_j Terms[i][j](x)
		termProd := symbolic.NewConstant(1)
		for j := range cs.Terms[i] {
			name := cs.Terms[i][j].String()
			termProd = symbolic.Mul(termProd, uniqueLimbs[name])
		}
		termProds[i] = termProd
	}
	evalSum := symbolic.NewLinComb(termProds, cs.Coeffs)
	// now we compute the other polynomials

	// modulus(x)
	modulusEval := csPolyEval(cs.Modulus, cs.Challenge)
	// quotient(x)
	quotientEval := csPolyEval(cs.Quotient, cs.Challenge)
	// carry(x)
	carryEval := csPolyEval(cs.Carry, cs.Challenge)
	// 2^nbBitsPerLimb - challenge
	coef := big.NewInt(0).Lsh(big.NewInt(1), uint(cs.NbBitsPerLimb))
	carryCoef := symbolic.Sub(
		symbolic.NewConstant(coef),
		cs.Challenge.AsVariable(),
	)

	// (2^nbBitsPerLimb - challenge) * carry(x)
	carryCoefEval := symbolic.Mul(carryEval, carryCoef)
	// modulus(x) * quotient(x)
	qmEval := symbolic.Mul(quotientEval, modulusEval)

	// finally define the constraint:
	// \sum_i \prod_j Terms[i][j](x) - modulus(x) * quotient(x) - carry(x) * (2^nbBitsPerLimb - challenge) = 0
	comp.InsertGlobal(
		cs.Round+1,
		ifaces.QueryIDf("%s_EMUL_EVAL", cs.Name),
		symbolic.Sub(
			evalSum,
			qmEval,
			carryCoefEval,
		),
	)
}
