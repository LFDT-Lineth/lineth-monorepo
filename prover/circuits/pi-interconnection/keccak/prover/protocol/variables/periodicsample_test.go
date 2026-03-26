package variables_test

import (
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPeriodicSampleGlobalConstraint(t *testing.T) {

	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P * PeriodicSample(4, 1) = 0
		expr := sym.Mul(P, variables.NewPeriodicSample(4, 1))

		_ = build.GlobalConstraint("Q", expr)
	}

	comp := wizard.Compile(
		definer,
		dummy.Compile,
	)

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", smartvectors.ForTest(1, 0, 4, 8, 16, 0, 64, 128))
	}

	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}

func TestPeriodicSampleAsLagrange(t *testing.T) {

	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P * PeriodicSample(8, 0) = 0
		expr := sym.Mul(P, variables.NewPeriodicSample(8, 0))
		_ = build.GlobalConstraint("Q", expr)
	}

	comp := wizard.Compile(
		definer,
		dummy.Compile,
	)

	prover := func(run *wizard.ProverRuntime) {
		run.AssignColumn("P", smartvectors.ForTest(0, 2, 4, 8, 16, 32, 64, 128))
	}

	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}

func TestPeriodicSampleShouldFail(t *testing.T) {
	definer := func(build *wizard.Builder) {

		// Number of rows
		n := 1 << 3
		P := build.RegisterCommit("P", n) // overshadows P

		// P * PeriodicSample(8, 0) = 0
		expr := sym.Mul(P, variables.NewPeriodicSample(8, 0))

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

	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)
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
					Operator.(sym.Variable).
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
					Operator.(sym.Variable).
					Metadata.(variables.PeriodicSample)

				vanillaEval := sampling.EvalCoset(domain, 0, 1, false)

				// Test EvalOnCoset
				for _, ratio := range ratios {
					for cosetID := 0; cosetID < ratio; cosetID++ {

						testEval := sampling.EvalCoset(domain, cosetID, ratio, true)
						testEval = smartvectors.FFTInverse(testEval, fft.DIF, true, ratio, cosetID, nil)
						testEval = smartvectors.FFT(testEval, fft.DIT, true, 0, 0, nil)

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

func TestPeriodicSampleEvalAtConsistentWithEval(t *testing.T) {

	domains := []int{16, 32, 64, 128}

	for _, domain := range domains {
		for period := 2; period <= domain; period *= 2 {
			for offset := 0; offset < period; offset++ {
				// Since NewPeriodicSample returns a variable directly (it's the only use-case)
				// We need to do a bunch of unwrapping to get to the real PeriodicSampling that
				// we want to test
				sampling := variables.NewPeriodicSample(period, offset).
					Operator.(sym.Variable).
					Metadata.(variables.PeriodicSample)

				vanillaEval := sampling.EvalCoset(domain, 0, 1, false)

				x := field.NewElement(420691966156)
				yExpected := smartvectors.Interpolate(vanillaEval, x)
				yActual := sampling.EvalAtOutOfDomain(domain, x)

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
					Operator.(sym.Variable).
					Metadata.(variables.PeriodicSample)

				for pos := 0; pos < period; pos++ {

					// Eval at should not work
					if pos == offset {
						require.Panics(t, func() {
							x := fft.GetOmega(domain)
							x.Exp(x, big.NewInt(int64(pos)))

							// This should equates 0/1
							_ = sampling.EvalAtOutOfDomain(domain, x)
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
