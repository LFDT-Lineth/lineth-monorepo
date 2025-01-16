package main

import (
	"fmt"
	"io"
	"math/big"
	"math/rand/v2"

	"github.com/consensys/gnark/std/math/emulated/emparams"
	"github.com/consensys/linea-monorepo/prover/backend/files"
)

func main() {

	for _, tcase := range testCases {
		f := files.MustOverwrite("./" + tcase.name + "_input.csv")
		dumpAsCsv(f, tcase.tab)
		f.Close()
	}
}

var testCases = []struct {
	name string
	tab  [][]*big.Int
}{
	{
		name: "single_256_bits",
		tab: func() [][]*big.Int {

			var (
				tab  = make([][]*big.Int, 5)
				rng  = rand.New(rand.NewChaCha8([32]byte{}))
				inst = createRandomModexp(rng, false)
			)

			pushModexpToInput(inst, tab)
			return tab
		}(),
	},
	{
		name: "single_4096_bits",
		tab: func() [][]*big.Int {

			var (
				tab  = make([][]*big.Int, 5)
				rng  = rand.New(rand.NewChaCha8([32]byte{}))
				inst = createRandomModexp(rng, true)
			)

			pushModexpToInput(inst, tab)
			return tab
		}(),
	},
}

func createRandomModexp(rng *rand.Rand, large bool) [4]*big.Int {

	var (
		maxValue = emparams.Mod1e256{}.Modulus()
		res      = [4]*big.Int{}
	)

	if large {
		maxValue = emparams.Mod1e4096{}.Modulus()
	}

	res[0] = new(big.Int).Rand(rng, maxValue)
	res[1] = new(big.Int).Rand(rng, maxValue)
	res[2] = new(big.Int).Rand(rng, maxValue)
	res[3] = new(big.Int).Exp(res[0], res[1], res[2])

	return res
}

func dumpAsCsv(w io.Writer, tab [][]*big.Int) {

	fmt.Fprintf(w, "LIMBS,IS_MODEXP_BASE,IS_MODEXP_EXPONENT,IS_MODEXP_MODULUS,IS_MODEXP_RESULT\n")

	for i := range tab[0] {
		fmt.Fprintf(w, "0x%v,%v,%v,%v,%v\n", tab[0][i].Text(16), tab[1][i].String(), tab[2][i].String(), tab[3][i].String(), tab[4][i].String())
	}
}

func pushFillerToInput(tab [][]*big.Int, rng *rand.Rand, numRow int) {

	maxValue := emparams.Mod1e256{}.Modulus()

	for i := 0; i < numRow; i++ {

		tab[0] = append(tab[0], new(big.Int).Rand(rng, maxValue))
		tab[1] = append(tab[1], &big.Int{})
		tab[2] = append(tab[2], &big.Int{})
		tab[3] = append(tab[3], &big.Int{})
		tab[4] = append(tab[4], &big.Int{})
	}
}

func pushModexpToInput(inst [4]*big.Int, tab [][]*big.Int) {

	var (
		limbs = splitIn32LimbsOf128Bits(inst[0])
		zero  = &big.Int{}
		one   = big.NewInt(1)
	)

	for i := range limbs {
		tab[0] = append(tab[0], limbs[i])
		tab[1] = append(tab[1], one)
		tab[2] = append(tab[2], zero)
		tab[3] = append(tab[3], zero)
		tab[4] = append(tab[4], zero)
	}

	limbs = splitIn32LimbsOf128Bits(inst[1])

	for i := range limbs {
		tab[0] = append(tab[0], limbs[i])
		tab[1] = append(tab[1], zero)
		tab[2] = append(tab[2], one)
		tab[3] = append(tab[3], zero)
		tab[4] = append(tab[4], zero)
	}

	limbs = splitIn32LimbsOf128Bits(inst[2])

	for i := range limbs {
		tab[0] = append(tab[0], limbs[i])
		tab[1] = append(tab[1], zero)
		tab[2] = append(tab[2], zero)
		tab[3] = append(tab[3], one)
		tab[4] = append(tab[4], zero)
	}

	limbs = splitIn32LimbsOf128Bits(inst[3])

	for i := range limbs {
		tab[0] = append(tab[0], limbs[i])
		tab[1] = append(tab[1], zero)
		tab[2] = append(tab[2], zero)
		tab[3] = append(tab[3], zero)
		tab[4] = append(tab[4], one)
	}
}

func splitIn32LimbsOf128Bits(x *big.Int) [32]*big.Int {

	var (
		res      = [32]*big.Int{}
		extended = make([]byte, 512)
		xBytes   = x.Bytes()
	)

	copy(extended[len(extended)-len(xBytes):], xBytes)

	for i := range res {
		res[i] = new(big.Int).SetBytes(extended[16*i : 16*i+16])
	}

	return res
}
