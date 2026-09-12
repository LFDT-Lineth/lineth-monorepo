package instrumentation

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// nbFeatures is the number of columns in the design matrix:
// one per WizardParameters field plus one bias column.
const nbFeatures = 17

// Sample is an observed (parameters, wall-clock duration) pair used to fit a
// LinearModel. Collect samples by running the prover on representative inputs
// and recording the elapsed time alongside the WizardParameters for that run.
type Sample struct {
	Params   WizardParameters
	Duration time.Duration
}

// Fit solves the ordinary-least-squares problem
//
//	min_β  ||X β - y||²
//
// where X is the (len(samples) × nbFeatures) design matrix — a bias column
// of ones followed by the WizardParameters fields — and y is the vector of
// observed durations in seconds.
//
// Panics if len(samples) < nbFeatures (under-determined system).
// Returns an error if the design matrix is numerically singular, which
// happens when one or more parameters are constant or linearly dependent
// across all samples.
func Fit(samples []Sample) (LinearModel, error) {
	if len(samples) < nbFeatures {
		panic(fmt.Sprintf(
			"instrumentation: Fit requires at least %d samples, got %d",
			nbFeatures, len(samples),
		))
	}

	var xtx [nbFeatures][nbFeatures]float64
	var xty [nbFeatures]float64

	for _, s := range samples {
		x := featureVector(s.Params)
		y := s.Duration.Seconds()
		for i := range nbFeatures {
			xty[i] += x[i] * y
			for j := range nbFeatures {
				xtx[i][j] += x[i] * x[j]
			}
		}
	}

	beta, err := solveNormalEquations(xtx, xty)
	if err != nil {
		return LinearModel{}, err
	}

	return betaToModel(beta), nil
}

// featureVector returns the design-matrix row for p.
// Index 0 is always the bias term (1.0); indices 1..16 follow the field
// order of WizardParameters. This order must be kept consistent with
// [betaToModel] and [LinearModel.Contributions].
func featureVector(p WizardParameters) [nbFeatures]float64 {
	return [nbFeatures]float64{
		1, // bias
		float64(p.NbColumns),
		float64(p.NbCells),
		float64(p.NbVanishingConstraints),
		float64(p.SumOf_Linearithmic_NbRow),
		float64(p.MaxVanishingConstraintDegree),
		float64(p.SumOfVanishing_NbRow_x_Degree),
		float64(p.SumOfVanishing_NbRow_x_Degree_Squared),
		float64(p.Lookup_Nb),
		float64(p.Lookup_NbTable),
		float64(p.Lookup_NbTableRows),
		float64(p.Lookup_NbTableCells),
		float64(p.Lookup_NbColumns),
		float64(p.MemoryBus_NbHandles),
		float64(p.MemoryBus_NbRows),
		float64(p.MemoryBus_NbCells),
		float64(p.MemoryBus_NbColumns),
	}
}

// betaToModel maps a coefficient vector (in featureVector order) to a LinearModel.
func betaToModel(b [nbFeatures]float64) LinearModel {
	return LinearModel{
		Bias:                              b[0],
		NbColumns:                         b[1],
		NbCells:                           b[2],
		NbVanishingConstraints:            b[3],
		SumOfLinearithmicNbRow:            b[4],
		MaxVanishingConstraintDegree:      b[5],
		SumOfVanishingNbRowXDegree:        b[6],
		SumOfVanishingNbRowXDegreeSquared: b[7],
		LookupNb:                          b[8],
		LookupNbTable:                     b[9],
		LookupNbTableRows:                 b[10],
		LookupNbTableCells:                b[11],
		LookupNbColumns:                   b[12],
		MemoryBusNbHandles:                b[13],
		MemoryBusNbRows:                   b[14],
		MemoryBusNbCells:                  b[15],
		MemoryBusNbColumns:                b[16],
	}
}

// solveNormalEquations solves xtx·β = xty using Gaussian elimination with
// partial pivoting. Returns an error when the matrix is singular or
// near-singular (pivot < 1e-12 in normalised units).
func solveNormalEquations(
	xtx [nbFeatures][nbFeatures]float64,
	xty [nbFeatures]float64,
) ([nbFeatures]float64, error) {
	n := nbFeatures

	// Build the augmented matrix [xtx | xty] as row slices so we can swap
	// rows cheaply.
	aug := make([][]float64, n)
	for i := range n {
		row := make([]float64, n+1)
		copy(row, xtx[i][:])
		row[n] = xty[i]
		aug[i] = row
	}

	// Forward elimination with partial pivoting.
	for col := range n {
		maxRow, maxVal := col, math.Abs(aug[col][col])
		for row := col + 1; row < n; row++ {
			if v := math.Abs(aug[row][col]); v > maxVal {
				maxVal, maxRow = v, row
			}
		}
		if maxVal < 1e-12 {
			return [nbFeatures]float64{}, errors.New(
				"instrumentation: singular or near-singular design matrix — " +
					"one or more features are linearly dependent across all samples",
			)
		}
		aug[col], aug[maxRow] = aug[maxRow], aug[col]

		pivot := aug[col][col]
		for row := col + 1; row < n; row++ {
			f := aug[row][col] / pivot
			for k := col; k <= n; k++ {
				aug[row][k] -= f * aug[col][k]
			}
		}
	}

	// Back substitution.
	var beta [nbFeatures]float64
	for i := n - 1; i >= 0; i-- {
		beta[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			beta[i] -= aug[i][j] * beta[j]
		}
		beta[i] /= aug[i][i]
	}

	return beta, nil
}
