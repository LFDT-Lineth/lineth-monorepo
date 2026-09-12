package instrumentation

import "time"

// LinearModel estimates prover wall-clock time as a linear combination of
// WizardParameters fields:
//
//	estimated_seconds = Bias + sum_i(Coeff_i * field_i)
//
// Coefficients carry units of seconds per unit of the corresponding parameter.
// Field names map to WizardParameters fields by position; see [featureVector].
// Use [Fit] to obtain a model from benchmark data.
type LinearModel struct {
	Bias float64

	NbColumns                         float64
	NbCells                           float64
	NbVanishingConstraints            float64
	SumOfLinearithmicNbRow            float64
	MaxVanishingConstraintDegree      float64
	SumOfVanishingNbRowXDegree        float64
	SumOfVanishingNbRowXDegreeSquared float64
	LookupNb                          float64
	LookupNbTable                     float64
	LookupNbTableRows                 float64
	LookupNbTableCells                float64
	LookupNbColumns                   float64
	MemoryBusNbHandles                float64
	MemoryBusNbRows                   float64
	MemoryBusNbCells                  float64
	MemoryBusNbColumns                float64
}

// Contribution is a named time contribution from one WizardParameters field.
type Contribution struct {
	Name  string
	Value time.Duration
}

// Estimate returns the predicted proof-generation time for p under model m.
func (m LinearModel) Estimate(p WizardParameters) time.Duration {
	return secondsToDuration(
		m.Bias +
			float64(p.NbColumns)*m.NbColumns +
			float64(p.NbCells)*m.NbCells +
			float64(p.NbVanishingConstraints)*m.NbVanishingConstraints +
			float64(p.SumOf_Linearithmic_NbRow)*m.SumOfLinearithmicNbRow +
			float64(p.MaxVanishingConstraintDegree)*m.MaxVanishingConstraintDegree +
			float64(p.SumOfVanishing_NbRow_x_Degree)*m.SumOfVanishingNbRowXDegree +
			float64(p.SumOfVanishing_NbRow_x_Degree_Squared)*m.SumOfVanishingNbRowXDegreeSquared +
			float64(p.Lookup_Nb)*m.LookupNb +
			float64(p.Lookup_NbTable)*m.LookupNbTable +
			float64(p.Lookup_NbTableRows)*m.LookupNbTableRows +
			float64(p.Lookup_NbTableCells)*m.LookupNbTableCells +
			float64(p.Lookup_NbColumns)*m.LookupNbColumns +
			float64(p.MemoryBus_NbHandles)*m.MemoryBusNbHandles +
			float64(p.MemoryBus_NbRows)*m.MemoryBusNbRows +
			float64(p.MemoryBus_NbCells)*m.MemoryBusNbCells +
			float64(p.MemoryBus_NbColumns)*m.MemoryBusNbColumns,
	)
}

// Contributions returns the per-field time breakdown for p under model m.
// Entries are listed in the same order as [WizardParameters] fields, with the
// Bias appended last. Their sum equals [LinearModel.Estimate](p).
//
// This is useful for identifying which constraint families dominate prover cost.
func (m LinearModel) Contributions(p WizardParameters) []Contribution {
	fc := fieldContrib
	return []Contribution{
		fc("NbColumns", p.NbColumns, m.NbColumns),
		fc("NbCells", p.NbCells, m.NbCells),
		fc("NbVanishingConstraints", p.NbVanishingConstraints, m.NbVanishingConstraints),
		fc("SumOf_Linearithmic_NbRow",
			p.SumOf_Linearithmic_NbRow, m.SumOfLinearithmicNbRow),
		fc("MaxVanishingConstraintDegree",
			p.MaxVanishingConstraintDegree, m.MaxVanishingConstraintDegree),
		fc("SumOfVanishing_NbRow_x_Degree",
			p.SumOfVanishing_NbRow_x_Degree, m.SumOfVanishingNbRowXDegree),
		fc("SumOfVanishing_NbRow_x_Degree_Squared",
			p.SumOfVanishing_NbRow_x_Degree_Squared, m.SumOfVanishingNbRowXDegreeSquared),
		fc("Lookup_Nb", p.Lookup_Nb, m.LookupNb),
		fc("Lookup_NbTable", p.Lookup_NbTable, m.LookupNbTable),
		fc("Lookup_NbTableRows", p.Lookup_NbTableRows, m.LookupNbTableRows),
		fc("Lookup_NbTableCells", p.Lookup_NbTableCells, m.LookupNbTableCells),
		fc("Lookup_NbColumns", p.Lookup_NbColumns, m.LookupNbColumns),
		fc("MemoryBus_NbHandles", p.MemoryBus_NbHandles, m.MemoryBusNbHandles),
		fc("MemoryBus_NbRows", p.MemoryBus_NbRows, m.MemoryBusNbRows),
		fc("MemoryBus_NbCells", p.MemoryBus_NbCells, m.MemoryBusNbCells),
		fc("MemoryBus_NbColumns", p.MemoryBus_NbColumns, m.MemoryBusNbColumns),
		{"Bias", secondsToDuration(m.Bias)},
	}
}

func fieldContrib(name string, fieldVal int, coeff float64) Contribution {
	return Contribution{name, secondsToDuration(float64(fieldVal) * coeff)}
}

func secondsToDuration(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}
