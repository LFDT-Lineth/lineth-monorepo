package instrumentation_test

import (
	"math"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/instrumentation"
	"github.com/stretchr/testify/require"
)

// syntheticModel is the ground-truth model used in recovery tests.
var syntheticModel = instrumentation.LinearModel{
	Bias:                              2.5,
	NbCells:                           1e-7,
	SumOfLinearithmicNbRow:            3e-9,
	SumOfVanishingNbRowXDegree:        5e-8,
	SumOfVanishingNbRowXDegreeSquared: 1e-9,
	LookupNbTableCells:                4e-8,
	MemoryBusNbCells:                  6e-8,
	// All other coefficients are zero.
}

// makeSamples generates n synthetic (noise-free) samples from model m.
//
// Each sample activates exactly one WizardParameters field (cycling through
// all 16), with a magnitude that grows with the sample index. This ensures
// the design-matrix columns are mutually orthogonal, so XᵀX is invertible
// regardless of which model coefficients are zero.
func makeSamples(m instrumentation.LinearModel, n int) []instrumentation.Sample {
	samples := make([]instrumentation.Sample, n)
	for i := range n {
		v := (i + 1) * 10000
		p := instrumentation.WizardParameters{}
		switch i % 16 {
		case 0:
			p.NbColumns = v
		case 1:
			p.NbCells = v
		case 2:
			p.NbVanishingConstraints = v
		case 3:
			p.SumOf_Linearithmic_NbRow = v
		case 4:
			p.MaxVanishingConstraintDegree = v
		case 5:
			p.SumOfVanishing_NbRow_x_Degree = v
		case 6:
			p.SumOfVanishing_NbRow_x_Degree_Squared = v
		case 7:
			p.Lookup_Nb = v
		case 8:
			p.Lookup_NbTable = v
		case 9:
			p.Lookup_NbTableRows = v
		case 10:
			p.Lookup_NbTableCells = v
		case 11:
			p.Lookup_NbColumns = v
		case 12:
			p.MemoryBus_NbHandles = v
		case 13:
			p.MemoryBus_NbRows = v
		case 14:
			p.MemoryBus_NbCells = v
		case 15:
			p.MemoryBus_NbColumns = v
		}
		samples[i] = instrumentation.Sample{
			Params:   p,
			Duration: m.Estimate(p),
		}
	}
	return samples
}

// TestFit_RecoversTrueCoefficients verifies that Fit recovers a known model
// when given noise-free synthetic samples.
func TestFit_RecoversTrueCoefficients(t *testing.T) {
	samples := makeSamples(syntheticModel, 50)
	got, err := instrumentation.Fit(samples)
	require.NoError(t, err)

	// Each predicted duration must match within 1 ms (floating point residual).
	for i, s := range samples {
		want := s.Duration
		pred := got.Estimate(s.Params)
		diff := math.Abs(float64(pred - want))
		require.Less(t, diff, float64(time.Millisecond),
			"sample %d: predicted %v, want %v", i, pred, want)
	}
}

// TestFit_SingularMatrix verifies that Fit returns an error when the design
// matrix is singular (e.g. all samples have identical parameters).
func TestFit_SingularMatrix(t *testing.T) {
	fixed := instrumentation.WizardParameters{NbCells: 1000}
	samples := make([]instrumentation.Sample, 20)
	for i := range samples {
		samples[i] = instrumentation.Sample{
			Params:   fixed,
			Duration: time.Duration(i) * time.Second,
		}
	}
	_, err := instrumentation.Fit(samples)
	require.Error(t, err, "Fit must fail with a singular design matrix")
}

// TestFit_PanicsWhenUnderDetermined verifies that Fit panics when given fewer
// samples than features.
func TestFit_PanicsWhenUnderDetermined(t *testing.T) {
	require.Panics(t, func() {
		_, _ = instrumentation.Fit([]instrumentation.Sample{{
			Params:   instrumentation.WizardParameters{NbCells: 1},
			Duration: time.Second,
		}})
	}, "Fit must panic when samples < nbFeatures")
}

// TestContributions_SumEqualsEstimate verifies that the sum of all
// Contributions equals Estimate for an arbitrary model and params.
func TestContributions_SumEqualsEstimate(t *testing.T) {
	p := instrumentation.WizardParameters{
		NbColumns:                500,
		NbCells:                  1 << 20,
		NbVanishingConstraints:   300,
		SumOf_Linearithmic_NbRow: 1 << 24,
		Lookup_NbTableCells:      1 << 18,
		MemoryBus_NbCells:        1 << 16,
	}

	total := syntheticModel.Estimate(p)
	var sum time.Duration
	for _, c := range syntheticModel.Contributions(p) {
		sum += c.Value
	}

	diff := math.Abs(float64(total - sum))
	require.Less(t, diff, float64(time.Microsecond),
		"sum of contributions (%v) must equal Estimate (%v)", sum, total)
}

// TestEstimate_ZeroModelZeroParams verifies that a zeroed model on zeroed
// params returns zero duration.
func TestEstimate_ZeroModelZeroParams(t *testing.T) {
	var m instrumentation.LinearModel
	var p instrumentation.WizardParameters
	require.Equal(t, time.Duration(0), m.Estimate(p))
}
