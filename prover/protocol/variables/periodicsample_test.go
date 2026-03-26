package variables_test

import (
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriodicSampleGlobalConstraint(t *testing.T) {

	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P(X) = P(X/w) + P(X/w^2)
		expr := ifaces.ColumnAsVariable(P).Mul(variables.NewPeriodicSample(4, 1))

		_ = build.GlobalConstraint("Q", expr)
	}

	comp := wizard.Compile(
		definer,
		dummy.Compile,
	)

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", smartvectors.ForTest(1, 0, 4, 8, 16, 0, 64, 128))
	}

	proof := wizard.Prove(comp, prover, false)
	err := wizard.Verify(comp, proof, false)
	require.NoError(t, err)

}

func TestPeriodicSampleAsLagrange(t *testing.T) {

	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P(X) = P(X/w) + P(X/w^2)
		expr := ifaces.ColumnAsVariable(P).Mul(variables.NewPeriodicSample(8, 0))
		_ = build.GlobalConstraint("Q", expr)
	}

	comp := wizard.Compile(
		definer,
		dummy.Compile,
	)

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", smartvectors.ForTest(0, 2, 4, 8, 16, 32, 64, 128))
	}

	proof := wizard.Prove(comp, prover, false)
	err := wizard.Verify(comp, proof, false)
	require.NoError(t, err)

}

func TestPeriodicSampleShouldFail(t *testing.T) {
	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P(X) = P(X/w) + P(X/w^2)
		expr := ifaces.ColumnAsVariable(P).Mul(variables.NewPeriodicSample(8, 0))

		_ = build.GlobalConstraint("Q", expr)
	}

	comp := wizard.Compile(
		definer,
		dummy.Compile,
	)

	prover := func(run *wizard.ProverRuntime) {
		// This is invalid because the first entry is non zero
		run.AssignColumn("P", smartvectors.ForTest(14, 2, 4, 8, 16, 32, 64, 128))
	}

	proof := wizard.Prove(comp, prover, false)
	err := wizard.Verify(comp, proof, false)
	require.Error(t, err)
}

func TestPeriodicSampleVanillaEval(t *testing.T) {

	domains := []int{16, 32, 64, 128}

	for _, domain := range domains {
		for period := 2; period <= domain; period *= 2 {
			for offset := 0; offset < period; offset++ {
				// Since NewPeriodicSample returns a variable directly (it's the only use-case)
				// We need to do a bunch of unwrapping to get to the real PeriodicSampling that
				// we want to test
				sampling := variables.NewPeriodicSample(period, offset).
					Operator.(symbolic.Variable).
					Metadata.(variables.PeriodicSample)

				vanillaEval := sampling.EvalCoset(domain, 0, 1, false)

				// Test the vanilla evaluation
				one := field.One()
				zero := field.Zero()
				for i := 0; i < vanillaEval.Len(); i++ {
					y := vanillaEval.Get(i)
					if i%period == offset {
						require.Equal(t, one.String(), y.String(), "i=%v, period=%v, offset=%v, domain=%v, fullEval=%v", i, period, offset, domain, vanillaEval.Pretty())
					} else {
						require.Equal(t, zero.String(), y.String(), "i=%v, period=%v, offset=%v, domain=%v, fullEval=%v", i, period, offset, domain, vanillaEval.Pretty())
					}
				}
			}
		}
	}

}

func TestPeriodicSampleCoset(t *testing.T) {

	domains := []int{16, 32, 64, 128}
	ratios := []int{1, 2, 4, 8, 16}

	for _, domain := range domains {
		for period := 2; period <= domain; period *= 2 {
			for offset := 0; offset < period; offset++ {
				// Since NewPeriodicSample returns a variable directly (it's the only use-case)
				// We need to do a bunch of unwrapping to get to the real PeriodicSampling that
				// we want to test
				sampling := variables.NewPeriodicSample(period, offset).
					Operator.(symbolic.Variable).
					Metadata.(variables.PeriodicSample)

				vanillaEval := sampling.EvalCoset(domain, 0, 1, false)

				// Test EvalOnCoset
				for _, ratio := range ratios {
					for cosetID := 0; cosetID < ratio; cosetID++ {
						testEval := sampling.EvalCoset(domain, cosetID, ratio, true)

						d := fft.NewDomain(uint64(testEval.Len()), fft.WithShift(computeShift(uint64(testEval.Len()), ratio, cosetID)), fft.WithCache())
						v := testEval.(*smartvectors.Regular)
						d.FFTInverse(*v, fft.DIF, fft.OnCoset())
						d.FFT(*v, fft.DIT)

						require.Equal(t, vanillaEval.Pretty(), testEval.Pretty(),
							"domain %v, period %v, offset %v, ratio %v, cosetID %v",
							domain, period, offset, ratio, cosetID,
						)
					}
				}

			}
		}
	}

}

func computeShift(n uint64, cosetRatio int, cosetID int) field.Element {
	var shift field.Element
	cardinality := ecc.NextPowerOfTwo(uint64(n))
	frMulGen := fft.GeneratorFullMultiplicativeGroup()
	omega, _ := fft.Generator(cardinality * uint64(cosetRatio))
	omega.Exp(omega, big.NewInt(int64(cosetID)))
	shift.Mul(&frMulGen, &omega)
	return shift
}

func TestPeriodicSampleEvalAtConsistentWithEval(t *testing.T) {

	domains := []int{16, 32, 64, 128}

	for _, domain := range domains {
		for period := 2; period <= domain; period *= 2 {
			for offset := 0; offset < period; offset++ {
				// Since NewPeriodicSample returns a variable directly (it's the only use-case)
				// We need to do a bunch of unwrapping to get to the real PeriodicSampling that
				// we want to test
				sampling := variables.NewPeriodicSample(period, offset).
					Operator.(symbolic.Variable).
					Metadata.(variables.PeriodicSample)

				vanillaEval := sampling.EvalCoset(domain, 0, 1, false)

				x := fext.NewFromUintBase(420691966156)
				yExpected := smartvectors.EvaluateBasePolyLagrange(vanillaEval, x)
				yActual := sampling.EvalAtOutOfDomainExt(domain, x)

				require.Equal(t, yExpected.String(), yActual.String())

			}
		}
	}
}

func TestPeriodicSampleEvalAtOnDomain(t *testing.T) {

	domains := []int{16, 32, 64, 128}
	for _, domain := range domains {
		for period := 2; period <= domain; period *= 2 {
			for offset := 0; offset < period; offset++ {
				// Since NewPeriodicSample returns a variable directly (it's the only use-case)
				// We need to do a bunch of unwrapping to get to the real PeriodicSampling that
				// we want to test
				sampling := variables.NewPeriodicSample(period, offset).
					Operator.(symbolic.Variable).
					Metadata.(variables.PeriodicSample)

				for pos := 0; pos < period; pos++ {

					// Eval at should not work
					if pos == offset {
						require.Panics(t, func() {
							_x, err := fft.Generator(uint64(domain))
							if err != nil {
								panic(err)
							}
							var x fext.Element
							fext.SetFromBase(&x, &_x)
							x.Exp(x, big.NewInt(int64(pos)))

							// This should equates 0/1
							_ = sampling.EvalAtOutOfDomainExt(domain, x)
						}, "domain %v, pos %v, offset %v", domain, pos, offset)
					}

					// But EvalAt on domain should
					v := sampling.EvalAtOnDomain(pos)
					expectedV := field.Zero()
					if offset == pos {
						expectedV.SetOne()
					}

					assert.Equalf(t, expectedV.String(), v.String(), "domain %v, pos %v, offset %v", domain, pos, offset)
				}
			}
		}
	}

}
