package gnarkutil

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark/backend/witness"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/math/emulated"
	gkr_poseidon2 "github.com/consensys/gnark/std/permutation/poseidon2/gkr-poseidon2"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils"
)

/*
Allocate a slice of field element
*/
// func AllocateSlice(n int) []koalagnark.Var {
// 	return make([]koalagnark.Var, n)
// }

/*
AllocateSliceExt allocates a slice of field extension elements
*/
func AllocateSliceExt(n int) []koalagnark.Ext {
	return make([]koalagnark.Ext, n)
}

// WitnessFromWrappedBls converts a slice of field elements to a slice of witness variables
// of the same length with only public inputs.
func WitnessFromWrappedBls(v ...koalagnark.Element) witness.Witness {

	var (
		wit, _  = witness.New(ecc.BLS12_377.ScalarField())
		witChan = make(chan any, len(v))
	)

	for _, w := range v {
		witChan <- w.Emulated()
	}

	close(witChan)

	if err := wit.Fill(len(v), 0, witChan); err != nil {
		panic(err)
	}

	return wit
}

// WitnessFromWrappedKoala converts a slice of base field elements to a slice
// of witness variables of the same length with only public inputs. The function
// assumes the WrappedVariable are not emulated element.
func WitnessFromWrappedKoala(v ...koalagnark.Element) witness.Witness {

	var (
		wit, _  = witness.New(koalabear.Modulus())
		witChan = make(chan any)
	)

	go func() {
		for _, w := range v {
			witChan <- w.Native()
		}
		close(witChan)
	}()

	if err := wit.Fill(len(v), 0, witChan); err != nil {
		panic(err)
	}

	return wit
}

// WitnessFromNativeKoala converts a slice of base field elements to a
// [witness.Witness].
func WitnessFromNativeKoala(v ...koalabear.Element) witness.Witness {

	var (
		wit, _  = witness.New(koalabear.Modulus())
		witChan = make(chan any)
	)

	go func() {
		for _, w := range v {
			witChan <- w
		}
		close(witChan)
	}()

	if err := wit.Fill(len(v), 0, witChan); err != nil {
		panic(err)
	}

	return wit
}

// EmulatedFromHiLo converts slices of frontend.Variable representing inputs
// lower than 2**bitWidth to slice of emulated.Element for the target field.
// The hi/lo inputs are expected to be in LITTLE-ENDIAN form.
func EmulatedFromHiLo[T emulated.FieldParams](
	api frontend.API,
	f *emulated.Field[T],
	hi, lo []frontend.Variable,
	bitWidth int,
) *emulated.Element[T] {
	return EmulatedFromLimbSlice(api, f, append(lo, hi...), bitWidth)
}

// EmulatedFromLimbSlice converts slice of frontend.Variable representing inputs
// lower than 2**bitWidth to slice of emulated.Element for the target field. The
// input is expected to be in LITTLE-ENDIAN form.
func EmulatedFromLimbSlice[T emulated.FieldParams](
	api frontend.API,
	f *emulated.Field[T],
	input []frontend.Variable,
	bitWidth int,
) *emulated.Element[T] {

	targetNbLimbs, targetBitWidth := emulated.GetEffectiveFieldParams[T](
		api.Compiler().Field(),
	)

	if targetNbLimbs*targetBitWidth < uint(len(input)*bitWidth) {
		utils.Panic(
			"can't fit on emulated field expected#bits=%v provided#bits=%v",
			targetNbLimbs*targetBitWidth, len(input)*bitWidth,
		)
	}

	// Then, it's nice and we don't need to reslice the inputs. We might need to
	// zero pad on the right
	if targetBitWidth == uint(bitWidth) {

		res := make([]frontend.Variable, targetNbLimbs)
		for i := 0; i < len(input); i++ {
			res[i] = input[i]
		}

		for i := len(input); i < int(targetNbLimbs); i++ {
			res[i] = 0
		}

		return f.NewElement(res)
	}

	// Otherwise, we need to slice the inputs. For now, we do it with binary
	// decomposition but it could be optimized. If too few bits are provided,
	// right-pad [bits] with zeroes.
	bits := make([]frontend.Variable, targetNbLimbs*targetBitWidth)
	for i := 0; i < len(input); i++ {
		inputBits := api.ToBinary(input[i], bitWidth)
		for j := range inputBits {
			bits[i*bitWidth+j] = inputBits[j]
		}
	}

	for i := len(input) * bitWidth; i < len(bits); i++ {
		bits[i] = 0
	}

	recomposed := make([]frontend.Variable, targetNbLimbs)
	for i := 0; i < len(recomposed); i++ {
		recomposed[i] = api.FromBinary(bits[i*int(targetBitWidth) : (i+1)*int(targetBitWidth)]...)
	}

	return f.NewElement(recomposed)
}

func RegisterHintsAndGkrGates() {
	solver.RegisterHint(breakIntoBytesHint, divBy31Hint)
	if err := gkr_poseidon2.RegisterGates(ecc.BLS12_377); err != nil {
		panic("could not register the gates")
	}
}

// GetSlicePosition returns a position in a slice of variables. Or zero if no match
// is found.
func GetSlicePosition(api frontend.API, v []frontend.Variable, pos frontend.Variable) (res, isMatched frontend.Variable) {

	res, isMatched = frontend.Variable(0), frontend.Variable(0)

	for i, v := range v {
		iIsMatch := api.IsZero(api.Sub(pos, frontend.Variable(i)))
		res = api.Select(iIsMatch, v, res)
		// No need to use "Or" is since all the "i" are distinct, pos cannot be
		// equal to more than one "i".
		isMatched = api.Add(isMatched, iIsMatch)
	}

	return res, isMatched
}
