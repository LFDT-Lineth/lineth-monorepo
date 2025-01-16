//go:build !fuzzlight

package keccakf

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
)

func TestU64FromToBase(t *testing.T) {

	const numCases int = 100

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))
	base1 := field.NewElement(uint64(BaseA))
	base2 := field.NewElement(uint64(BaseB))

	for i := 0; i < numCases; i++ {
		x := rnd.Uint64()

		xBase1 := U64ToBaseX(x, &base1)
		xBase2 := U64ToBaseX(x, &base2)

		backFromBase1 := BaseXToU64(xBase1, &base1)
		backFromBase2 := BaseXToU64(xBase2, &base2)

		assert.Equal(t, x, backFromBase1, "failed for base 1")
		assert.Equal(t, x, backFromBase2, "failed for base 2")

		if t.Failed() {
			t.Fatalf("failed at iteration %d", i)
		}
	}

	// When the Msb are zeroes
	for i := 0; i < numCases; i++ {
		x := rnd.Uint64() & 0xffffffff

		xBase1 := U64ToBaseX(x, &base1)
		xBase2 := U64ToBaseX(x, &base2)

		backFromBase1 := BaseXToU64(xBase1, &base1)
		backFromBase2 := BaseXToU64(xBase2, &base2)

		assert.Equal(t, x, backFromBase1, "failed for base 1")
		assert.Equal(t, x, backFromBase2, "failed for base 2")

		if t.Failed() {
			t.Fatalf("failed at iteration %d", i)
		}
	}

}

func TestBaseDecomposeRecompose(t *testing.T) {

	const numCases int = 100

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))
	base1 := field.NewElement(uint64(BaseA))
	base2 := field.NewElement(uint64(BaseB))

	for i := 0; i < numCases; i++ {

		// Generates correct representations for base1 and base2 from
		// a random U64.
		x := rnd.Uint64()
		xBase1 := U64ToBaseX(x, &base1)
		xBase2 := U64ToBaseX(x, &base2)

		decomposed1 := DecomposeFr(xBase1, BaseA, 64)
		decomposed2 := DecomposeFr(xBase2, BaseB, 64)

		recomposed1 := BaseRecompose(decomposed1, &base1)
		recomposed2 := BaseRecompose(decomposed2, &base2)

		assert.Equal(t, xBase1.String(), recomposed1.String())
		assert.Equal(t, xBase2.String(), recomposed2.String())

		if t.Failed() {
			t.Fatalf("failed at iteration %d", i)
		}
	}
}

func TestDecomposeInSlice(t *testing.T) {

	const numCases int = 100

	// #nosec G404 --we don't need a cryptographic RNG for testing purpose
	rnd := rand.New(rand.NewChaCha8([32]byte{}))
	base1 := field.NewElement(uint64(BaseA))
	base2 := field.NewElement(uint64(BaseB))

	for i := 0; i < numCases; i++ {

		// Generates correct representations for base1 and base2 from
		// a random U64.
		x := rnd.Uint64()
		xBase1 := U64ToBaseX(x, &base1)
		xBase2 := U64ToBaseX(x, &base2)

		decomposed1 := DecomposeFr(xBase1, BaseAPow4, numSlice)
		decomposed2 := DecomposeFr(xBase2, BaseBPow4, numSlice)

		for k := 0; k < numSlice; k++ {
			expected := (x >> (4 * k)) & 0xf
			u := BaseXToU64(decomposed1[k], &BaseAFr)
			v := BaseXToU64(decomposed2[k], &BaseBFr)
			assert.Equalf(t, expected, u, "slice does not match in base 1 [%v]", k)
			assert.Equalf(t, expected, v, "slice does not match in base 2 [%v]", k)
		}

		if t.Failed() {
			t.Fatalf("failed at iteration %d", i)
		}
	}
}

func TestBaseXToU64WithShift(t *testing.T) {

	cases := []struct {
		Inp      field.Element
		Expected uint64
	}{
		{Inp: field.NewElement(0), Expected: 0},
		{Inp: field.NewElement(1), Expected: 0},
		{Inp: field.NewElement(2), Expected: 1},
		{Inp: field.NewElement(3), Expected: 1},
		{Inp: field.NewElement(4), Expected: 0},
		{Inp: field.NewElement(5), Expected: 0},
		{Inp: field.NewElement(6), Expected: 1},
		{Inp: field.NewElement(7), Expected: 1},
		{Inp: field.NewElement(8), Expected: 0},
	}

	for _, cas := range cases {
		actual := BaseXToU64(cas.Inp, &BaseBFr, 1)
		assert.Equal(t, cas.Expected, actual, "input: %v", cas.Inp.String())
	}
}
