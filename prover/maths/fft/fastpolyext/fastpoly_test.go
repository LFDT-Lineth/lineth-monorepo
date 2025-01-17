package fastpolyext_test

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vectorext"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpolyext"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/stretchr/testify/require"
)

func TestMultiplication(t *testing.T) {

	n := 8
	// smallN := 6
	domain := fft.NewDomain(n)

	// Unitary X : multiplying by it results in identity
	{
		unit := make([]fext.Element, n)
		unit[0].SetOne()

		vec := vectorext.Rand(n)
		vecBackup := vectorext.DeepCopy(vec)
		res := make([]fext.Element, n)
		fastpolyext.MultModXMinus1(domain, res, vec, unit)

		require.Equal(t, vectorext.Prettify(vecBackup), vectorext.Prettify(res))
	}

	// Unitary X : multiplying by it results in identity
	// With precomputation
	{
		unit := make([]fext.Element, n)
		unit[0].SetOne()
		domain.FFTExt(unit, fft.DIF)

		vec := vectorext.Rand(n)
		vecBackup := vectorext.DeepCopy(vec)
		res := make([]fext.Element, n)
		fastpolyext.MultModXnMinus1Precomputed(domain, res, vec, unit)

		require.Equal(t, vectorext.Prettify(vecBackup), vectorext.Prettify(res))
	}

	// Polynomial X : multiplying by it results in "circular" permutation
	{
		shift := make([]fext.Element, n)
		shift[1].SetOne()

		vec := vectorext.Rand(n)
		vecBackup := vectorext.DeepCopy(vec)
		res := make([]fext.Element, n)
		fastpolyext.MultModXMinus1(domain, res, vec, shift)

		require.Equal(t, vectorext.Prettify(res[1:]), vectorext.Prettify(vecBackup[:n-1]))
		require.Equal(t, res[0].String(), vecBackup[n-1].String())
	}

	// Polynomial X : multiplication with precomputation
	{
		shift := make([]fext.Element, n)
		shift[1].SetOne()
		domain.FFTExt(shift, fft.DIF)

		vec := vectorext.Rand(n)
		vecBackup := vectorext.DeepCopy(vec)
		res := make([]fext.Element, n)
		fastpolyext.MultModXnMinus1Precomputed(domain, res, vec, shift)

		require.Equal(t, vectorext.Prettify(res[1:]), vectorext.Prettify(vecBackup[:n-1]))
		require.Equal(t, res[0].String(), vecBackup[n-1].String())

	}
}
