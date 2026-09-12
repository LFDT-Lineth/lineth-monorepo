package instrumentation

import (
	"math/bits"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// MeasureSystem derives WizardParameters from a fully-declared System.
//
// All modules must be sized (static modules via [wiop.Module.SetSize], dynamic
// modules via a prior [wiop.Runtime] whose sizes are now fixed). Unsized and
// dynamic modules contribute only their column count to [WizardParameters.NbColumns];
// all row-dependent fields (NbCells, linearithmic sums, vanishing sums) are
// computed only for modules where [wiop.Module.IsSized] returns true.
func MeasureSystem(sys *wiop.System) WizardParameters {
	var p WizardParameters

	for _, m := range sys.Modules {
		nbCols := len(m.Columns)
		p.NbColumns += nbCols

		if m.IsDynamic() || !m.IsSized() {
			continue
		}
		n := m.Size()
		log2n := bits.Len(uint(n)) - 1 // valid because n is always a power of two
		p.NbCells += n * nbCols
		p.SumOf_Linearithmic_NbRow += nbCols * n * log2n

		for _, v := range m.Vanishings {
			p.NbVanishingConstraints++
			d := v.Expression.DegreeFactor()
			p.MaxVanishingConstraintDegree = max(p.MaxVanishingConstraintDegree, d)
			p.SumOfVanishing_NbRow_x_Degree += n * d
			p.SumOfVanishing_NbRow_x_Degree_Squared += n * d * d
		}
	}

	for _, tr := range sys.TableRelations {
		if tr.Kind != wiop.KindInclusion {
			continue
		}
		p.Lookup_Nb++
		p.Lookup_NbTable += len(tr.B)
		for _, tab := range tr.A {
			p.Lookup_NbColumns += tab.Width()
		}
		for _, tab := range tr.B {
			p.Lookup_NbColumns += tab.Width()
			m := tab.Module()
			if m.IsDynamic() || !m.IsSized() {
				continue
			}
			p.Lookup_NbTableRows += m.Size()
			p.Lookup_NbTableCells += m.Size() * tab.Width()
		}
	}

	p.MemoryBus_NbHandles = len(sys.MessageBusHandles())
	for _, mb := range sys.MessageBuses {
		m := mb.Tab.Module()
		p.MemoryBus_NbColumns += mb.Tab.Width()
		if m.IsDynamic() || !m.IsSized() {
			continue
		}
		p.MemoryBus_NbRows += m.Size()
		p.MemoryBus_NbCells += m.Size() * mb.Tab.Width()
	}

	return p
}
