package instrumentation

// WizardParameters contains regressable parameters for the wizard which are
// used to estimate the runtime of the prover. These parameters are to be
// estimated PRIOR to compilation so that they can be used by the
// arithmetization directly.
type WizardParameters struct {
	// Total number of columns in the wizard
	NbColumns int
	// Total number of cells in the wizard
	NbCells int
	// The total number of vanishing constraints
	NbVanishingConstraints int
	// The sum of nbRow * log2(nbRow) for every column
	SumOf_Linearithmic_NbRow int
	// The degree of the largest vanishing constraint
	MaxVanishingConstraintDegree int
	// The sum of nbRow * degree for every vanishing constraint
	SumOfVanishing_NbRow_x_Degree int
	// The sum of nbRow * degree**2 for every vanishing constraint
	SumOfVanishing_NbRow_x_Degree_Squared int
	// Total number of lookup queries
	Lookup_Nb int
	// Total number of tables in the lookup queries
	Lookup_NbTable int
	// Total number of rows in the lookup queries
	Lookup_NbTableRows int
	// Total number of cells in the lookup tables
	Lookup_NbTableCells int
	// The total number of columns involved in a lookup
	Lookup_NbColumns int
	// The total number of handles involved in the memory  BUS
	MemoryBus_NbHandles int
	// Total number of rows involved in the memory  BUS
	MemoryBus_NbRows int
	// Total number of cells into the memory  BUS
	MemoryBus_NbCells int
	// Total number of columns into the memory  BUS
	MemoryBus_NbColumns int
}
