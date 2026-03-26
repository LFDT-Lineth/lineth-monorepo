package compiler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/logdata"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/poseidon2"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/internal/testtools"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// StdBenchmarkCase represents a benchmark case for the Arcane and Vortex compilers
type StdBenchmarkCase struct {
	// Name is the name of the benchmark case
	Name string

	Permutations SubModuleParameters
	Lookup       SubModuleParameters
	Projection   SubModuleParameters
	Fibo         SubModuleParameters
}

// selfRecursionParameters stores parameters for self-recursion
type selfRecursionParameters struct {
	TargetRowSize   int
	RsInverseRate   int
	NbOpenedColumns int
}

type selfRecursionIterationParameters struct {
	InitTargetColSize int
	MidTargetRowSize  int
	LastTargetRowSize int
}

// SubModuleParameters represents the parameters of a sub-module in [StdBenchmarkCase]
type SubModuleParameters struct {
	// Count specifies the number of instances of the sub-module
	Count int
	// NumCol specifies the number of columns to define for the module if this
	// is useful to specify.
	NumCol int
	// NumRow specifies the number of rows to define for the module
	NumRow int
	// NumRowAux is an optional parameter for module requiring a second set of
	// columns with a different number of rows such as Lookups or Projections
	NumRowAux int
}

var (
	benchCases = []StdBenchmarkCase{
		// {
		// 	Name: "minimal",
		// 	Permutations: SubModuleParameters{
		// 		Count:  1,
		// 		NumCol: 1,
		// 		NumRow: 1 << 10,
		// 	},
		// 	Lookup: SubModuleParameters{
		// 		Count:     1,
		// 		NumCol:    1,
		// 		NumRow:    1 << 10,
		// 		NumRowAux: 1 << 10,
		// 	},
		// 	Projection: SubModuleParameters{
		// 		Count:     1,
		// 		NumCol:    1,
		// 		NumRow:    1 << 10,
		// 		NumRowAux: 1 << 10,
		// 	},
		// 	Fibo: SubModuleParameters{
		// 		Count:  1,
		// 		NumRow: 1 << 10,
		// 	},
		// },
		{
			// run with GOGC=200 and
			// ensure THP is enabled:
			// cat /sys/kernel/mm/transparent_hugepage/enabled
			// if not:
			// echo always | sudo tee /sys/kernel/mm/transparent_hugepage/enabled
			Name: "realistic-segment",
			Permutations: SubModuleParameters{
				Count:  5,
				NumCol: 3,
				NumRow: 1 << 20,
			},
			Lookup: SubModuleParameters{
				Count:     50,
				NumCol:    3,
				NumRow:    1 << 20,
				NumRowAux: 1 << 20,
			},
			Projection: SubModuleParameters{
				Count:     5,
				NumCol:    3,
				NumRow:    1 << 20,
				NumRowAux: 1 << 20,
			},
			Fibo: SubModuleParameters{
				Count:  200,
				NumRow: 1 << 20,
			},
		},
		// {
		// 	Name: "smaller-segment",
		// 	Permutations: SubModuleParameters{
		// 		Count:  1,
		// 		NumCol: 3,
		// 		NumRow: 1 << 19,
		// 	},
		// 	Lookup: SubModuleParameters{
		// 		Count:     5,
		// 		NumCol:    3,
		// 		NumRow:    1 << 19,
		// 		NumRowAux: 1 << 19,
		// 	},
		// 	Projection: SubModuleParameters{
		// 		Count:     1,
		// 		NumCol:    3,
		// 		NumRow:    1 << 19,
		// 		NumRowAux: 1 << 19,
		// 	},
		// 	Fibo: SubModuleParameters{
		// 		Count:  20,
		// 		NumRow: 1 << 19,
		// 	},
		// },
	}
)

func BenchmarkCompilerWithoutSelfRecursion(b *testing.B) {
	for _, bc := range benchCases {
		b.Run(bc.Name, func(b *testing.B) {
			benchmarkCompilerWithoutSelfRecursion(b, bc)
		})
	}
}

func BenchmarkCompilerWithSelfRecursion(b *testing.B) {
	for _, bc := range benchCases {
		b.Run(bc.Name, func(b *testing.B) {
			benchmarkCompilerWithSelfRecursion(b, bc)
		})
	}
}

func BenchmarkCompilerWithSelfRecursionAndGnarkVerifier(b *testing.B) {
	for _, bc := range benchCases {
		b.Run(bc.Name, func(b *testing.B) {
			benchmarkCompilerWithSelfRecursionAndGnarkVerifier(b, bc)
		})
	}
}
func BenchmarkProfileSelfRecursion(b *testing.B) {
	for _, bc := range benchCases {
		b.Run(bc.Name, func(b *testing.B) {
			profileSelfRecursionCompilation(b, bc)
		})
	}
}

func profileSelfRecursionCompilation(b *testing.B, sbc StdBenchmarkCase) {

	logrus.SetLevel(logrus.FatalLevel)

	const NbOpenedColumns = 64
	const RsInverseRate = 16

	// nbIteration = 2 is the best setting with minimun proof size
	// go test -timeout=10h -test.fullpath=true -benchmem -run=^$ -bench ^BenchmarkProfileSelfRecursion$ github.com/consensys/linea-monorepo/prover/protocol/compiler 2>&1 | tee benchmark_results.txt
	for nbIteration := 1; nbIteration < 5; nbIteration++ {

		fmt.Printf("\n\n\n\n-------------------------------------------\n nbIteration = %v\n\n", nbIteration)

		for lastIterationTargetRowSize := 9; lastIterationTargetRowSize < 14; lastIterationTargetRowSize++ {

			lastIterationParams := selfRecursionParameters{
				NbOpenedColumns: NbOpenedColumns,
				RsInverseRate:   RsInverseRate,
				TargetRowSize:   1 << lastIterationTargetRowSize,
			}

			for midIterationsTargetRowSize := 6; midIterationsTargetRowSize < 14; midIterationsTargetRowSize++ {

				midIterationsParams := selfRecursionParameters{
					NbOpenedColumns: NbOpenedColumns,
					RsInverseRate:   RsInverseRate,
					TargetRowSize:   1 << midIterationsTargetRowSize,
				}

				for initIterationTargetColSize := 13; initIterationTargetColSize < 20; initIterationTargetColSize++ {

					iterationParams := selfRecursionIterationParameters{
						InitTargetColSize: 1 << initIterationTargetColSize,
						MidTargetRowSize:  midIterationsParams.TargetRowSize,
						LastTargetRowSize: lastIterationParams.TargetRowSize,
					}

					b.Run(fmt.Sprintf("%+v", iterationParams), func(b *testing.B) {
						comp := wizard.Compile(
							// Round of recursion 0
							sbc.Define,
							compiler.Arcane(
								compiler.WithTargetColSize(1<<initIterationTargetColSize),
								compiler.WithStitcherMinSize(1<<8),
							),
							vortex.Compile(
								RsInverseRate,
								false,
								vortex.WithOptionalSISHashingThreshold(512),
								vortex.ForceNumOpenedColumns(NbOpenedColumns),
								vortex.WithSISParams(&ringsis.StdParams),
							),
						)

						for i := 0; i < nbIteration-1; i++ {
							applySelfRecursionThenArcane(comp, midIterationsParams)
							applyVortex(comp, midIterationsParams, false)
						}

						statsVortex := logdata.GetWizardStats(comp)

						applySelfRecursionThenArcane(comp, lastIterationParams)

						statsArcane := logdata.GetWizardStats(comp)

						b.ReportMetric(float64(statsArcane.NumCellsCommitted), "#committed-cells")
						b.ReportMetric(float64(statsVortex.NumCellsProof), "#proof-cells")

					})
				}
			}
		}
	}
}

func benchmarkCompilerWithoutSelfRecursion(b *testing.B, sbc StdBenchmarkCase) {

	comp := wizard.Compile(
		sbc.Define,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<18),
			compiler.WithStitcherMinSize(1<<8),
		),
		vortex.Compile(
			2,
			false,
			vortex.WithOptionalSISHashingThreshold(0),
			vortex.ForceNumOpenedColumns(256),
			vortex.WithSISParams(&ringsis.StdParams),
		),
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wizard.Prove(comp, sbc.NewAssigner(b), false)
	}
}

func benchmarkCompilerWithSelfRecursion(b *testing.B, sbc StdBenchmarkCase) {

	// These parameters have been found to give the best result for performances
	params := selfRecursionParameters{
		NbOpenedColumns: 64,
		RsInverseRate:   16,
		TargetRowSize:   1 << 9,
	}

	comp := wizard.Compile(
		// Round of recursion 0
		sbc.Define,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<20),
			compiler.WithStitcherMinSize(1<<1),
		),
		vortex.Compile(
			2,
			false,
			vortex.WithOptionalSISHashingThreshold(512),
			vortex.ForceNumOpenedColumns(256),
			vortex.WithSISParams(&ringsis.StdParams),
		),
	)

	nbIteration := 2

	for i := 0; i < nbIteration; i++ {
		applySelfRecursionThenArcane(comp, params)
		applyVortex(comp, params, false)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = wizard.Prove(comp, sbc.NewAssigner(b), false)
	}
}

func benchmarkCompilerWithSelfRecursionAndGnarkVerifier(b *testing.B, sbc StdBenchmarkCase) {

	isBLS := true
	// These parameters have been found to give the best result for minimum constraints and proof size
	// NbOpenedColumns: 64
	// RsInverseRate:   16
	// nbIteration := 2
	// initTargetColSize := 1 << 16
	// midIterationTargetRowSize := 1 << 8
	// lastIterationTargetRowSize := 1 << 10
	midIterationParams := selfRecursionParameters{
		NbOpenedColumns: 64,
		RsInverseRate:   16,
		TargetRowSize:   1 << 8,
	}

	// RsInverseRate = 2, nbOpenedColumns=256; OR
	// RsInverseRate = 16, nbOpenedColumns=64; BETTER, less constraints
	lastIterationParams := selfRecursionParameters{
		NbOpenedColumns: 64,
		RsInverseRate:   16,
		TargetRowSize:   1 << 10,
	}

	comp := wizard.Compile(
		// Round of recursion 0
		sbc.Define,
		compiler.Arcane(
			compiler.WithTargetColSize(1<<16),
			compiler.WithStitcherMinSize(1<<1),
		),
		// RsInverseRate = 2, nbOpenedColumns=256; OR
		// RsInverseRate = 16, nbOpenedColumns=64; BETTER, less constraints
		vortex.Compile(
			16,
			false,
			vortex.WithOptionalSISHashingThreshold(512),
			vortex.ForceNumOpenedColumns(64),
			vortex.WithSISParams(&ringsis.StdParams),
		),
	)

	nbIteration := 2

	for i := 0; i < nbIteration; i++ {
		if i == nbIteration-1 {
			applySelfRecursionThenArcane(comp, lastIterationParams)
			applyVortex(comp, lastIterationParams, true) // last iteration over BLS
		} else {
			applySelfRecursionThenArcane(comp, midIterationParams)
			applyVortex(comp, midIterationParams, false) // other iteration over koalabear
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		proof := wizard.Prove(comp, sbc.NewAssigner(b), isBLS)
		err := wizard.Verify(comp, proof, isBLS)
		if err != nil {
			b.Fatal(err)
		}

		circuit := verifierCircuit{}
		nbRounds := comp.NumRounds() // comp.NumRounds() //17 // TODO setting this to comp.NumRounds() make the number of constraint explode, need to investigate
		fmt.Printf("using nbRounds=%d instead of %d\n", nbRounds, comp.NumRounds())
		{
			c := wizard.AllocateWizardCircuit(comp, nbRounds, isBLS)
			circuit.C = *c
		}
		// gnarkProfile := profile.Start(profile.WithPath(fmt.Sprintf("./gnark_%d_%d.pprof", nbRounds, time.Now().Unix())))
		ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.WithCapacity(1<<27), frontend.IgnoreUnconstrainedInputs())
		if err != nil {
			b.Fatal(err)
		}
		// gnarkProfile.Stop()
		fmt.Printf("ccs number of constraints: %d\n", ccs.GetNbConstraints())

		assignment := &verifierCircuit{
			C: *wizard.AssignVerifierCircuit(comp, proof, nbRounds, isBLS),
		}

		witness, err := frontend.NewWitness(assignment, ecc.BLS12_377.ScalarField())
		if err != nil {
			b.Fatal(err)
		}

		// Check if solved using the pre-compiled SCS
		if !testing.Short() {
			err = ccs.IsSolved(witness)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

type verifierCircuit struct {
	C wizard.VerifierCircuit
}

func (c *verifierCircuit) Define(api frontend.API) error {
	c.C.Verify(api)
	return nil
}

// Define defines the benchmark case
func (sbc *StdBenchmarkCase) Define(b *wizard.Builder) {

	for n := 0; n < sbc.Permutations.Count; n++ {
		definePermutationModule(b.CompiledIOP, n, sbc.Permutations)
	}

	for n := 0; n < sbc.Lookup.Count; n++ {
		defineLookupModule(b.CompiledIOP, n, sbc.Lookup)
	}

	for n := 0; n < sbc.Projection.Count; n++ {
		defineProjectionModule(b.CompiledIOP, n, sbc.Projection)
	}

	for n := 0; n < sbc.Fibo.Count; n++ {
		defineFiboModule(b.CompiledIOP, n, sbc.Fibo)
	}
}

// NewAssigner returns a function assigning the modules of the benchmark case. The
// function also stops the timer during that time.
func (sbc *StdBenchmarkCase) NewAssigner(b *testing.B) func(run *wizard.ProverRuntime) {
	return func(run *wizard.ProverRuntime) {

		b.StopTimer()

		for n := 0; n < sbc.Permutations.Count; n++ {
			assignPermutationModule(run, n, sbc.Permutations)
		}

		for n := 0; n < sbc.Lookup.Count; n++ {
			assignLookupModule(run, n, sbc.Lookup)
		}

		for n := 0; n < sbc.Projection.Count; n++ {
			assignProjectionModule(run, n, sbc.Projection)
		}

		for n := 0; n < sbc.Fibo.Count; n++ {
			assignFiboModule(run, n, sbc.Fibo)
		}

		b.StartTimer()
	}
}

// applySelfRecursionThenArcane applies the self-recursion step and then the
// arcane step using the provided parameters.
func applySelfRecursionThenArcane(comp *wizard.CompiledIOP, params selfRecursionParameters) {

	selfrecursion.SelfRecurse(comp)

	var (
		stats      = logdata.GetWizardStats(comp)
		totalCells = stats.NumCellsCommitted
		rowSize    = utils.NextPowerOfTwo(utils.DivCeil(totalCells, params.TargetRowSize))
	)

	_ = wizard.ContinueCompilation(
		comp,
		poseidon2.CompilePoseidon2,
		compiler.Arcane(
			compiler.WithTargetColSize(rowSize),
			compiler.WithStitcherMinSize(1<<1),
		),
	)
}

// applyVortex applies the vortex step using the provided parameters.
func applyVortex(comp *wizard.CompiledIOP, params selfRecursionParameters, IsBLS bool) {

	if IsBLS {
		_ = wizard.ContinueCompilation(
			comp,
			vortex.Compile(
				params.RsInverseRate,
				true,
				vortex.ForceNumOpenedColumns(params.NbOpenedColumns),
				vortex.WithOptionalSISHashingThreshold(1<<20),
			),
		)
	} else {

		_ = wizard.ContinueCompilation(
			comp,
			vortex.Compile(
				params.RsInverseRate,
				false,
				vortex.ForceNumOpenedColumns(params.NbOpenedColumns),
				vortex.WithSISParams(&ringsis.StdParams),
			),
		)
	}
}

// defineLookupModule adds a lookup module to the benchmark case
func defineLookupModule(comp *wizard.CompiledIOP, index int, params SubModuleParameters) {

	// This is needed because both tables are assigned using [testtools.RandomFromSeed]
	// and if NumRowAux was smaller, then S (parametrized with NumRow) would be
	// would contain more rows than T and would therefore contain entries not
	// contained in S.
	if params.NumRow < params.NumRowAux {
		utils.Panic("Lookup module requires NumRow < NumRowAux, got %++v for index %v", params, index)
	}

	var (
		s = []ifaces.Column{}
		t = []ifaces.Column{}
	)

	for i := 0; i < params.NumCol; i++ {
		si := comp.InsertCommit(0, formatName[ifaces.ColID]("Lookup", index, "S", i), params.NumRow, true)
		ti := comp.InsertCommit(0, formatName[ifaces.ColID]("Lookup", index, "T", i), params.NumRowAux, true)

		s = append(s, si)
		t = append(t, ti)
	}

	comp.InsertInclusion(0, formatName[ifaces.QueryID]("Lookup", index, "Query"), t, s)
}

// assignLookupModule assigns random values to the columns of a lookup module using
// [testtools.RandomFromSeed].
func assignLookupModule(run *wizard.ProverRuntime, index int, params SubModuleParameters) {

	ti := testtools.RandomFromSeed(params.NumRowAux, int64(index))
	si := testtools.RandomFromSeed(params.NumRow, int64(index))

	for i := 0; i < params.NumCol; i++ {
		run.AssignColumn(formatName[ifaces.ColID]("Lookup", index, "S", i), si)
		run.AssignColumn(formatName[ifaces.ColID]("Lookup", index, "T", i), ti)
	}
}

// definePermutationModule creates a permutation module with the requested number
// rows and columns on each side of the permutation.
func definePermutationModule(comp *wizard.CompiledIOP, index int, params SubModuleParameters) {

	var (
		a = []ifaces.Column{}
		b = []ifaces.Column{}
	)

	for i := 0; i < params.NumCol; i++ {
		ai := comp.InsertCommit(0, formatName[ifaces.ColID]("Permutation", index, "A", i), params.NumRow, true)
		bi := comp.InsertCommit(0, formatName[ifaces.ColID]("Permutation", index, "B", i), params.NumRow, true)

		a = append(a, ai)
		b = append(b, bi)
	}

	comp.InsertPermutation(0, formatName[ifaces.QueryID]("Permutation", index, "Query"), a, b)
}

// assignPermutationModule assigns random values to the columns of a permutation module using
// [testtools.RandomFromSeed] for a and the same reverted vector for b.
func assignPermutationModule(run *wizard.ProverRuntime, index int, params SubModuleParameters) {

	ai := testtools.RandomFromSeed(params.NumRow, int64(index))
	bi := testtools.Reverse(ai)

	for i := 0; i < params.NumCol; i++ {
		run.AssignColumn(formatName[ifaces.ColID]("Permutation", index, "A", i), ai)
		run.AssignColumn(formatName[ifaces.ColID]("Permutation", index, "B", i), bi)
	}
}

// defineProjectionModule adds a projection module to the benchmark case.
func defineProjectionModule(comp *wizard.CompiledIOP, index int, params SubModuleParameters) {

	var (
		a       = []ifaces.Column{}
		b       = []ifaces.Column{}
		aFilter = comp.InsertCommit(0, formatName[ifaces.ColID]("Projection", index, "AFilter"), params.NumRow, true)
		bFilter = comp.InsertCommit(0, formatName[ifaces.ColID]("Projection", index, "BFilter"), params.NumRowAux, true)
	)

	for i := 0; i < params.NumCol; i++ {
		ai := comp.InsertCommit(0, formatName[ifaces.ColID]("Projection", index, "A", i), params.NumRow, true)
		bi := comp.InsertCommit(0, formatName[ifaces.ColID]("Projection", index, "B", i), params.NumRowAux, true)

		a = append(a, ai)
		b = append(b, bi)
	}

	comp.InsertProjection(
		formatName[ifaces.QueryID]("Projection", index, "Query"),
		query.ProjectionInput{
			ColumnA: a,
			ColumnB: b,
			FilterA: aFilter,
			FilterB: bFilter,
		})
}

// assignProjectionModule assigns random values to the columns of a projection
// using [testtools.RandomFromSeed]. The large side of the projection is zero-padded
// to match the length of the small side and make the projection work.
func assignProjectionModule(run *wizard.ProverRuntime, index int, params SubModuleParameters) {

	var (
		sizeSmall        = min(params.NumRow, params.NumRowAux)
		sizeLarge        = max(params.NumRow, params.NumRowAux)
		valueSmall       = testtools.RandomFromSeed(sizeSmall, int64(index))
		valueLarge       = smartvectors.RightZeroPadded(valueSmall.IntoRegVecSaveAlloc(), sizeLarge)
		filterSmall      = smartvectors.NewConstant(field.One(), sizeSmall)
		filterLarge      = smartvectors.RightZeroPadded(filterSmall.IntoRegVecSaveAlloc(), sizeLarge)
		ai, bi           smartvectors.SmartVector
		aFilter, bFilter smartvectors.SmartVector
	)

	if params.NumRow < params.NumRowAux {
		ai = valueSmall
		bi = valueLarge
		aFilter = filterSmall
		bFilter = filterLarge
	} else {
		ai = valueLarge
		bi = valueSmall
		aFilter = filterLarge
		bFilter = filterSmall
	}

	run.AssignColumn(formatName[ifaces.ColID]("Projection", index, "AFilter"), aFilter)
	run.AssignColumn(formatName[ifaces.ColID]("Projection", index, "BFilter"), bFilter)

	for i := 0; i < params.NumCol; i++ {
		run.AssignColumn(formatName[ifaces.ColID]("Projection", index, "A", i), ai)
		run.AssignColumn(formatName[ifaces.ColID]("Projection", index, "B", i), bi)
	}
}

// defineFibo creates a single column constrained to be the fibonacci sequence
// starting at 1, 1
func defineFiboModule(comp *wizard.CompiledIOP, index int, params SubModuleParameters) {

	a := comp.InsertCommit(0, formatName[ifaces.ColID]("Fibo", index), params.NumRow, true)

	comp.InsertGlobal(0, formatName[ifaces.QueryID]("Fibo", index, "Global"), sym.Sub(
		a,
		column.Shift(a, -1),
		column.Shift(a, -2),
	))

	comp.InsertLocal(
		0,
		formatName[ifaces.QueryID]("Fibo", index, "Local_0"),
		sym.Sub(a, 1),
	)

	comp.InsertLocal(
		0,
		formatName[ifaces.QueryID]("Fibo", index, "Local_1"),
		sym.Sub(column.Shift(a, 1), 1),
	)
}

func assignFiboModule(run *wizard.ProverRuntime, index int, params SubModuleParameters) {

	fibo := make([]field.Element, 0, params.NumRow)
	fibo = append(fibo, field.One(), field.One())

	for i := 2; i < params.NumRow; i++ {
		var next field.Element
		next.Add(&fibo[i-1], &fibo[i-2])
		fibo = append(fibo, next)
	}

	run.AssignColumn(
		formatName[ifaces.ColID]("Fibo", index),
		smartvectors.NewRegular(fibo),
	)
}

func formatName[T ~string](args ...any) T {
	argsStr := []string{"BENCHMARK"}
	for _, arg := range args {
		argStr := fmt.Sprintf("%v", arg)
		argStr = strings.ToUpper(argStr)
		argsStr = append(argsStr, argStr)
	}
	return T(strings.Join(argsStr, "_"))
}
