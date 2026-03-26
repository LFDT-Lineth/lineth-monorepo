package fext

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/go-playground/assert/v2"
)

func TestGenericFieldOp(t *testing.T) {

	type (
		BinaryOpGeneric func(a, b *GenericFieldElem) *GenericFieldElem
		BinaryOpFext    func(r, a, b *Element) *Element
	)

	operations := []struct {
		name      string
		fextOp    BinaryOpFext
		genericOp BinaryOpGeneric
	}{
		{
			name:      "Add",
			fextOp:    (*Element).Add,
			genericOp: (*GenericFieldElem).Add,
		},
		{
			name:      "Mul",
			fextOp:    (*Element).Mul,
			genericOp: (*GenericFieldElem).Mul,
		},
		{
			name:      "Div",
			fextOp:    (*Element).Div,
			genericOp: (*GenericFieldElem).Div,
		},
	}

	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) {

			var (
				// #nosec G404 -- we don't need a cryptographic PRNG for testing purposes
				rng         = rand.New(utils.NewRandSource(43))
				a           = PseudoRand(rng)
				b           = PseudoRand(rng)
				aBase       = a.B0.A0
				bBase       = b.B0.A0
				aBaseLifted = Lift(aBase)
				bBaseLifted = Lift(bBase)

				aBaseGen = NewGenFieldFromBase(aBase)
				bBaseGen = NewGenFieldFromBase(bBase)
				aExtGen  = NewGenFieldFromExt(a)
				bExtGen  = NewGenFieldFromExt(b)
			)

			t.Run("fext-base", func(t *testing.T) {

				var (
					resFext Element
					resGen  GenericFieldElem
				)

				resFext = *op.fextOp(&resFext, &a, &bBaseLifted)
				resGen.Set(&aExtGen)
				resGen = *op.genericOp(&resGen, &bBaseGen)
				assert.Equal(t, resFext.String(), resGen.String())
			})

			t.Run("base-fext", func(t *testing.T) {

				var (
					resFext Element
					resGen  GenericFieldElem
				)

				resFext = *op.fextOp(&resFext, &aBaseLifted, &b)
				resGen.Set(&aBaseGen)
				resGen = *op.genericOp(&resGen, &bExtGen)
				assert.Equal(t, resFext.String(), resGen.String())
			})

			t.Run("base-base", func(t *testing.T) {

				var (
					resFext Element
					resGen  GenericFieldElem
				)

				resFext = *op.fextOp(&resFext, &aBaseLifted, &bBaseLifted)
				resGen.Set(&aBaseGen)
				resGen = *op.genericOp(&resGen, &bBaseGen)
				assert.Equal(t, resFext.B0.A0.String(), resGen.String())
			})

			t.Run("fext-fext", func(t *testing.T) {

				var (
					resFext Element
					resGen  GenericFieldElem
				)

				resFext = *op.fextOp(&resFext, &a, &b)
				resGen.Set(&aExtGen)
				resGen = *op.genericOp(&resGen, &bExtGen)
				assert.Equal(t, resFext.String(), resGen.String())
			})
		})
	}

}
