package compilers_test

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"

// columnSnapshot records, per module, the set of columns present at some
// point of compilation.
type columnSnapshot map[*wiop.Module]map[*wiop.Column]struct{}

// snapshotModuleColumns records the columns present before a compiler pass
// runs. Pair with [newBaseColumns] to identify the
// columns a pass adds.
func snapshotModuleColumns(sys *wiop.System) columnSnapshot {
	before := make(columnSnapshot, len(sys.Modules))
	for _, m := range sys.Modules {
		seen := make(map[*wiop.Column]struct{}, len(m.Columns))
		for _, c := range m.Columns {
			seen[c] = struct{}{}
		}
		before[m] = seen
	}
	return before
}

// newBaseColumns returns the base-field columns added to any module since the
// snapshot, such as the multiplicity columns M of the lookuptologderivsum pass.
func newBaseColumns(sys *wiop.System, before columnSnapshot) []*wiop.Column {
	return newColumns(sys, before, false)
}

// newColumns returns the columns added since the snapshot whose field kind
// matches isExtension, in module then declaration order.
func newColumns(sys *wiop.System, before columnSnapshot, isExtension bool) []*wiop.Column {
	var cols []*wiop.Column
	for _, m := range sys.Modules {
		for _, c := range m.Columns {
			if _, existed := before[m][c]; !existed && c.IsExtension == isExtension {
				cols = append(cols, c)
			}
		}
	}
	return cols
}
