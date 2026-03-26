package poseidon2

import (
	gnarkposeidon2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	width           = 16
	blockSize       = 8
	fullRounds      = 28
	partialRounds   = 3
	matMulM4TmpSize = 20
	tSize           = 4
)

// TODO @YaoJGalteland @gbotrel why is this not using directly gnark-crypto's Poseidon2 implementation?

// poseidon2BlockCompression applies the Poseidon2 block compression function to a given block
// over a given state. This what is run under the hood by the Poseidon2 hash function
func poseidon2BlockCompression(oldState, block [blockSize]field.Element) (newState [blockSize]field.Element) {

	var state [width]field.Element
	copy(state[:8], oldState[:])
	copy(state[8:], block[:])

	// Create a buffer to hold the feed-forward input.
	copy(newState[:], state[8:])

	// Poseidon2 compression
	// The `vortex.CompressPoseidon2` function is the canonical reference.
	// This function implements the Poseidon2 algorithm from scratch for
	// verification and clarity.

	sBoxSum := make([]field.Element, fullRounds)

	// Initial round
	matMulExternalInPlace(&state)

	// Rounds 1 - 3
	// External rounds
	for round := 1; round < 1+partialRounds; round++ {
		addRoundKeyCompute(round-1, &state)
		for col := 0; col < width; col++ {
			state[col] = sBoxCompute(col, round, state[:])
		}
		matMulExternalInPlace(&state)
	}

	// Rounds 4 - 24
	// Internal rounds
	for round := 1 + partialRounds; round < fullRounds-partialRounds; round++ {
		addRoundKeyCompute(round-1, &state)
		for col := 0; col < width; col++ {
			state[col] = sBoxCompute(col, round, state[:])
		}
		sBoxSum[round] = matMulInternalInPlace(&state)
	}

	// Rounds 24 - 27
	// External rounds
	for round := fullRounds - partialRounds; round < fullRounds; round++ {
		addRoundKeyCompute(round-1, &state)

		for col := 0; col < width; col++ {
			state[col] = sBoxCompute(col, round, state[:])
		}
		matMulExternalInPlace(&state)
	}

	// Final round
	// Feed forward
	for i := range newState {
		newState[i].Add(&newState[i], &state[8+i])
	}
	return newState
}

// matMulExternalInPlace applies the external matrix multiplication.
// see https://eprint.iacr.org/2023/323.pdf
func matMulExternalInPlace(input *[width]field.Element) {

	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}
	var matMulM4Tmp [matMulM4TmpSize]field.Element
	var matMulM4 [width]field.Element
	var t [tSize]field.Element

	for i := 0; i < 4; i++ {
		matMulM4Tmp[5*i].Add(&input[4*i], &input[4*i+1])
		matMulM4Tmp[5*i+1].Add(&input[4*i+2], &input[4*i+3])
		matMulM4Tmp[5*i+2].Add(&matMulM4Tmp[5*i], &matMulM4Tmp[5*i+1])
		matMulM4Tmp[5*i+3].Add(&matMulM4Tmp[5*i+2], &input[4*i+1])
		matMulM4Tmp[5*i+4].Add(&matMulM4Tmp[5*i+2], &input[4*i+3])

		// The order here is important. Need to overwrite x[0] and x[2] after x[1] and x[3].
		matMulM4[4*i+3].Double(&input[4*i]).Add(&matMulM4[4*i+3], &matMulM4Tmp[5*i+4])
		matMulM4[4*i+1].Double(&input[4*i+2]).Add(&matMulM4[4*i+1], &matMulM4Tmp[5*i+3])
		matMulM4[4*i].Add(&matMulM4Tmp[5*i], &matMulM4Tmp[5*i+3])
		matMulM4[4*i+2].Add(&matMulM4Tmp[5*i+1], &matMulM4Tmp[5*i+4])
	}
	for i := 0; i < 4; i++ {
		t[0].Add(&t[0], &matMulM4[4*i])
		t[1].Add(&t[1], &matMulM4[4*i+1])
		t[2].Add(&t[2], &matMulM4[4*i+2])
		t[3].Add(&t[3], &matMulM4[4*i+3])
	}
	for i := 0; i < 4; i++ {
		input[4*i].Add(&matMulM4[4*i], &t[0])
		input[4*i+1].Add(&matMulM4[4*i+1], &t[1])
		input[4*i+2].Add(&matMulM4[4*i+2], &t[2])
		input[4*i+3].Add(&matMulM4[4*i+3], &t[3])
	}
}

// matMulInternal applies the internal matrix multiplication.
func matMulInternalInPlace(input *[width]field.Element) (sBoxSum field.Element) {
	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}

	sBoxSum.Set(&input[0])
	for i := 1; i < width; i++ {
		sBoxSum.Add(&sBoxSum, &input[i])
	}
	// mul by diag16:
	// [-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
	two := field.NewElement(2)
	half := field.NewElement(1065353217)
	halfExp3 := field.NewElement(1864368129)
	halfExp4 := field.NewElement(1997537281)
	halfExp8 := field.NewElement(2122383361)
	halfExp24 := field.NewElement(127) // -127

	var temp field.Element
	input[0].Sub(&sBoxSum, temp.Double(&input[0]))
	input[1].Add(&sBoxSum, &input[1])
	input[2].Add(&sBoxSum, temp.Mul(&input[2], &two))
	input[3].Add(&sBoxSum, temp.Mul(&input[3], &half))
	input[4].Add(&sBoxSum, temp.Double(&input[4]).Add(&temp, &input[4]))
	input[5].Add(&sBoxSum, temp.Double(&input[5]).Double(&temp))
	input[6].Sub(&sBoxSum, temp.Mul(&input[6], &half))
	input[7].Sub(&sBoxSum, temp.Double(&input[7]).Add(&temp, &input[7]))
	input[8].Sub(&sBoxSum, temp.Double(&input[8]).Double(&temp))
	input[9].Add(&sBoxSum, temp.Mul(&input[9], &halfExp8))
	input[10].Add(&sBoxSum, temp.Mul(&input[10], &halfExp3))
	input[11].Sub(&sBoxSum, temp.Mul(&input[11], &halfExp24))
	input[12].Sub(&sBoxSum, temp.Mul(&input[12], &halfExp8))
	input[13].Sub(&sBoxSum, temp.Mul(&input[13], &halfExp3))
	input[14].Sub(&sBoxSum, temp.Mul(&input[14], &halfExp4))
	input[15].Add(&sBoxSum, temp.Mul(&input[15], &halfExp24))

	return sBoxSum
}

// addRoundKey adds the round-th key to the input
func addRoundKeyCompute(round int, input *[width]field.Element) {
	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}
	for i := 0; i < len(gnarkposeidon2.GetDefaultParameters().RoundKeys[round]); i++ {
		input[i].Add(&input[i], &gnarkposeidon2.GetDefaultParameters().RoundKeys[round][i])
	}
	// No-op: elements beyond the round key length are unchanged.
}

// SBoxCompute applies the  s-box on input[index]
func sBoxCompute(index, poseidon2Round int, input []field.Element) (sBox field.Element) {
	if len(input) != width {
		utils.Panic("Input slice length must be %v", width)
	}

	if poseidon2Round < fullRounds-partialRounds && poseidon2Round > partialRounds && index != 0 {
		sBox = input[index]
	} else {
		// sbox degree is 3
		sBox.Square(&input[index]).
			Mul(&sBox, &input[index])
	}

	return sBox
}
