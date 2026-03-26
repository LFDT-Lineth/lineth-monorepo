package distributed_test

import (
	"fmt"
	"testing"

	"github.com/consensys/linea-monorepo/prover/backend/files"
	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

const (
	NbRow = 8
)

// testCompilationParams defines compilation parameters for testing segment compilation
// Note: ColumnProfileMPTS is left nil to avoid profile size constraints during testing
var testCompilationParams = distributed.CompilationParams{
	FixedNbRowPlonkCircuit:       1 << 25,
	FixedNbRowExternalHasher:     1 << 19, // Increased from 1<<22 to handle hash claims
	FixedNbPublicInput:           1 << 10,
	InitialCompilerSize:          1 << 18,
	InitialCompilerSizeConglo:    1 << 18,
	ColumnProfileMPTS:            []int{264, 1400, 256, 24, 12, 28, 8, 8},
	ColumnProfileMPTSPrecomputed: 45,
	FullDebugMode:                false,
}

// DistributedTestCase is an interface for test cases that can be run through
// the distributed wizard.
type DistributedTestCase interface {
	// Name returns the name of the test case
	Name() string
	// Define defines the structure of the wizard (columns, constraints, queries)
	Define(comp *wizard.CompiledIOP)
	// Assign assigns values to the columns at runtime
	Assign(run *wizard.ProverRuntime)
	// Advices returns a list of advices for the module discovery.
	Advices() []*distributed.ModuleDiscoveryAdvice
}

// TestDistributedWizard runs the distributed wizard test over multiple test cases.
// Each test case defines a different set of queries (inclusion, projection, permutation).
func TestDistributedWizard(t *testing.T) {

	testCases := []DistributedTestCase{
		&LookupTestCase{numRow: 1 << NbRow},
		&ProjectionTestCase{numRow: 1 << NbRow},
		&PermutationTestCase{numRow: 1 << NbRow},
		&FibExtTestCase{numRow: 1 << NbRow},
	}

	for _, tc := range testCases {
		t.Run(tc.Name(), func(t *testing.T) {
			runDistributedWizardTest(t, tc, false)
		})
	}
}

// TestCompileOneSegment tests the compilation of a single segment.
func TestCompileOneSegment(t *testing.T) {

	t.SkipNow()

	var (
		tc      = &PermutationTestCase{numRow: 1 << NbRow}
		defFunc = func(build *wizard.Builder) { tc.Define(build.CompiledIOP) }
		wiop    = wizard.Compile(defFunc)
		disc    = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << NbRow,
			Advices:      tc.Advices(),
		}
		distWizard = distributed.DistributeWizard(wiop, disc)
		compiled   = distributed.CompileSegment(distWizard.GLs[0], testCompilationParams)
	)

	profileTree, err := serde.Profile(compiled)
	if err != nil {
		t.Fatal(err)
	}

	profileTree.PruneTree(1 << 20)
	serde.WriteProfileTo(profileTree, files.MustOverwrite("./profiling/comp-profile.txt"))
}

// runDistributedWizardTest runs a single distributed wizard test case.
func runDistributedWizardTest(t *testing.T, tc DistributedTestCase, segmentCompilation bool) {

	var (
		defFunc = func(build *wizard.Builder) { tc.Define(build.CompiledIOP) }
		wiop    = wizard.Compile(defFunc)
		disc    = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << NbRow,
			Advices:      tc.Advices(),
		}
		distWizard *distributed.DistributedWizard
	)

	// This tests the compilation of the compiled-IOP
	if segmentCompilation {
		// compile with vortex/ self-recursion/recursion
		distWizard = distributed.DistributeWizard(wiop, disc).
			CompileSegments(testCompilationParams).
			Conglomerate(testCompilationParams)
	} else {
		// dummy compilation
		distWizard = distributed.DistributeWizard(wiop, disc)
	}

	// This compilation step is needed for sanity-checking the bootstrapper
	dummy.Compile(distWizard.Bootstrapper)

	// This applies the dummy.Compiler to all parts of the distributed wizard.
	for i := range distWizard.GLs {
		dummy.CompileAtProverLvl()(distWizard.GLs[i].Wiop)
		// Add dummy LPP merkle root public inputs (normally added by Vortex compiler)
		for j := 0; j < 8; j++ {
			name := fmt.Sprintf("LPP_COLUMNS_MERKLE_ROOTS_0_%d", j)
			distWizard.GLs[i].Wiop.InsertPublicInput(name, accessors.NewConstant(field.Zero()))
		}
	}

	for i := range distWizard.LPPs {
		dummy.CompileAtProverLvl()(distWizard.LPPs[i].Wiop)
		// Add dummy LPP merkle root public inputs (normally added by Vortex compiler)
		for j := 0; j < 8; j++ {
			name := fmt.Sprintf("LPP_COLUMNS_MERKLE_ROOTS_0_%d", j)
			distWizard.LPPs[i].Wiop.InsertPublicInput(name, accessors.NewConstant(field.Zero()))
		}
	}

	var (
		runtimeBoot = wizard.RunProver(distWizard.Bootstrapper, tc.Assign, false)
		proof       = runtimeBoot.ExtractProof()
		verBootErr  = wizard.Verify(distWizard.Bootstrapper, proof)
	)

	if verBootErr != nil {
		t.Fatalf("Bootstrapper failed because: %v", verBootErr)
	}

	var (
		allGrandProduct     = fext.One()
		allLogDerivativeSum = fext.Element{}
		allHornerSum        = fext.Element{}
		generalMSet         = multisethashing.MSetHash{}
	)

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		distWizard.Disc,
		distWizard.BlueprintGLs,
		distWizard.BlueprintLPPs,
		field.Octuplet{},
	)

	for i := range witnessGLs {

		var (
			witnessGL = witnessGLs[i]
			moduleGL  *distributed.ModuleGL
		)

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessGL.ModuleName, witnessGL.ModuleIndex)

		for k := range distWizard.ModuleNames {
			if distWizard.ModuleNames[k] != witnessGLs[i].ModuleName {
				continue
			}

			moduleGL = distWizard.GLs[k]
			break
		}

		if moduleGL == nil {
			t.Fatalf("module does not exists")
		}

		var (
			proverRunGL         = wizard.RunProver(moduleGL.Wiop, moduleGL.GetMainProverStep(witnessGLs[i]), false)
			proofGL             = proverRunGL.ExtractProof()
			verRun, verGLErr    = wizard.VerifyWithRuntime(moduleGL.Wiop, proofGL, false)
			generalMSetFromGLFr = distributed.GetPublicInputList(verRun, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
			generalMSetFromGL   = multisethashing.MSetHash(generalMSetFromGLFr)
		)

		if verGLErr != nil {
			t.Errorf("verifier failed for segment %v, reason=%v", i, verGLErr)
		}

		generalMSet.Add(generalMSetFromGL)
	}

	for i := range witnessLPPs {

		var (
			witnessLPP  = witnessLPPs[i]
			moduleIndex = witnessLPPs[i].ModuleIndex
			moduleLPP   = distWizard.LPPs[moduleIndex]
		)

		// The test uses stubbed values for the initial Fiat-Shamir state for
		// replicability reasons. This is important to help debugging.
		witnessLPP.InitialFiatShamirState = field.NewOctupletFromStrings(
			[8]string{
				"123456789",
				"987654321",
				"111111111",
				"222222222",
				"333333333",
				"444444444",
				"555555555",
				"666666666",
			})

		t.Logf("segment(total)=%v module=%v segment.index=%v", i, witnessLPP.ModuleName, witnessLPP.ModuleIndex)

		var (
			proverRunLPP         = wizard.RunProver(moduleLPP.Wiop, moduleLPP.GetMainProverStep(witnessLPP), false)
			proofLPP             = proverRunLPP.ExtractProof()
			verRun, verLPPErr    = wizard.VerifyWithRuntime(moduleLPP.Wiop, proofLPP, false)
			generalMSetFromLPPFr = distributed.GetPublicInputList(verRun, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
			generalMSetFromLPP   = multisethashing.MSetHash(generalMSetFromLPPFr)
		)

		if verLPPErr != nil {
			t.Errorf("verifier failed for segment %v, reason=%v", i, verLPPErr)
		}

		generalMSet.Add(generalMSetFromLPP)

		var (
			logDerivativeSum = verRun.GetPublicInput(distributed.LogDerivativeSumPublicInput).Ext
			grandProduct     = verRun.GetPublicInput(distributed.GrandProductPublicInput).Ext
			hornerSum        = verRun.GetPublicInput(distributed.HornerPublicInput).Ext
		)

		t.Logf("log-derivative-sum=%v grand-product=%v horner-sum=%v", logDerivativeSum.String(), grandProduct.String(), hornerSum.String())

		allGrandProduct.Mul(&allGrandProduct, &grandProduct)
		allHornerSum.Add(&allHornerSum, &hornerSum)
		allLogDerivativeSum.Add(&allLogDerivativeSum, &logDerivativeSum)
	}

	if !allGrandProduct.IsOne() {
		t.Errorf("grand-product does not cancel")
	}

	if !allHornerSum.IsZero() {
		t.Errorf("horner does not cancel")
	}

	if !allLogDerivativeSum.IsZero() {
		t.Errorf("log-derivative-sum does not cancel. Has %v", allLogDerivativeSum.String())
	}

	if !generalMSet.IsEmpty() {
		t.Errorf("generalMSet does not cancel: ")
	}
}

// LookupTestCase tests the basic distributed wizard with inclusion query (lookup).
// The testcase generates 2 triplets of columns a, b, c such that a + b = c
// and the two modules are joined by a lookup.
type LookupTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *LookupTestCase) Name() string {
	return "Lookup"
}

// Define defines the structure of the distributed wizard. The structure is
// composed of 2 modules that are connected by a lookup. The two modules are
// identical and are defined as a + b = c.
func (d *LookupTestCase) Define(comp *wizard.CompiledIOP) {

	d.wiop = comp

	// Define the first module
	a0 := comp.InsertCommit(0, "a0", d.numRow, true)
	b0 := comp.InsertCommit(0, "b0", d.numRow, true)
	c0 := comp.InsertCommit(0, "c0", d.numRow, true)

	// Importantly, the second module must be slightly different than the first
	// one because else it will create a wierd edge case in the conglomeration:
	// as we would have two GL modules with the same verifying key and we would
	// not be able to infer a module from a VK.
	//
	// We differentiate the modules by adding a duplicate constraints for GL0
	a1 := comp.InsertCommit(0, "a1", d.numRow, true)
	b1 := comp.InsertCommit(0, "b1", d.numRow, true)
	c1 := comp.InsertCommit(0, "c1", d.numRow, true)

	comp.InsertGlobal(0, "global-0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-duplicate", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-1", symbolic.Sub(c1, b1, a1))

	comp.InsertInclusion(0, "inclusion-0", []ifaces.Column{c0, b0, a0}, []ifaces.Column{c1, b1, a1})
}

// Assign sets up the column assignments for the LookupTestCase.
func (d *LookupTestCase) Assign(run *wizard.ProverRuntime) {
	run.AssignColumn("a0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c0", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
	run.AssignColumn("a1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(1), d.numRow-2), d.numRow))
	run.AssignColumn("b1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(2), d.numRow-2), d.numRow))
	run.AssignColumn("c1", smartvectors.RightZeroPadded(vector.Repeat(field.NewElement(3), d.numRow-2), d.numRow))
}

// Advices returns the advices for the LookupTestCase.
func (d *LookupTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a1")),
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a0")),
	}
}

// ProjectionTestCase includes a projection query in addition to
// the global and inclusion constraints. Projection queries are compiled
// into Horner evaluations.
type ProjectionTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *ProjectionTestCase) Name() string {
	return "Projection"
}

// Define defines the structure with projection query. It has:
// - Global constraints a + b = c for both modules
// - An inclusion query between modules
// - A projection query between filtered columns
func (d *ProjectionTestCase) Define(comp *wizard.CompiledIOP) {

	d.wiop = comp

	// Define the first module
	a0 := comp.InsertCommit(0, "a0", d.numRow/2, true)
	b0 := comp.InsertCommit(0, "b0", d.numRow/2, true)
	c0 := comp.InsertCommit(0, "c0", d.numRow/2, true)

	// Second module with slight differences
	a1 := comp.InsertCommit(0, "a1", d.numRow, true)
	b1 := comp.InsertCommit(0, "b1", d.numRow, true)
	c1 := comp.InsertCommit(0, "c1", d.numRow, true)

	// Projection columns: projA with filterA projects to projB with filterB
	filterA := comp.InsertCommit(0, "filterA", d.numRow/2, true)
	filterB := comp.InsertCommit(0, "filterB", d.numRow, true)

	comp.InsertGlobal(0, "global-0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-1", symbolic.Sub(c1, b1, a1))

	// Projection query: filtered values from projA should match filtered values from projB
	comp.InsertProjection(
		"projection-0",
		query.ProjectionInput{
			ColumnA: []ifaces.Column{a0, b0, c0},
			ColumnB: []ifaces.Column{a1, b1, c1},
			FilterA: filterA,
			FilterB: filterB,
		},
	)
}

// Assign assigns the columns for the projection test case.
// The projection query maps filtered rows from (a0, b0, c0) to (a1, b1, c1).
// Each column has sequential values 1, 2, 3, ...
func (d *ProjectionTestCase) Assign(run *wizard.ProverRuntime) {
	sizeSmall := d.numRow / 2
	sizeLarge := d.numRow

	// Create sequential values for the small table (a0, b0, c0)
	// a0: 1, 2, 3, ..., sizeSmall
	// b0: 1, 2, 3, ..., sizeSmall
	// c0: 2, 4, 6, ..., 2*sizeSmall (since c = a + b for the global constraint)
	a0Vals := make([]field.Element, sizeSmall)
	b0Vals := make([]field.Element, sizeSmall)
	c0Vals := make([]field.Element, sizeSmall)
	for i := 0; i < sizeSmall; i++ {
		a0Vals[i] = field.NewElement(uint64(i + 1))
		b0Vals[i] = field.NewElement(uint64(i + 1))
		c0Vals[i] = field.NewElement(uint64(2 * (i + 1))) // c = a + b
	}

	// Create values for the large table (a1, b1, c1)
	// The first sizeSmall rows contain the same values as (a0, b0, c0)
	// The rest are padded with zeros
	a1Vals := make([]field.Element, sizeLarge)
	b1Vals := make([]field.Element, sizeLarge)
	c1Vals := make([]field.Element, sizeLarge)
	for i := 0; i < sizeSmall; i++ {
		a1Vals[i] = field.NewElement(uint64(i + 1))
		b1Vals[i] = field.NewElement(uint64(i + 1))
		c1Vals[i] = field.NewElement(uint64(2 * (i + 1))) // c = a + b
	}
	// Remaining positions are zero (default)

	run.AssignColumn("a0", smartvectors.NewRegular(a0Vals))
	run.AssignColumn("b0", smartvectors.NewRegular(b0Vals))
	run.AssignColumn("c0", smartvectors.NewRegular(c0Vals))
	run.AssignColumn("a1", smartvectors.NewRegular(a1Vals))
	run.AssignColumn("b1", smartvectors.NewRegular(b1Vals))
	run.AssignColumn("c1", smartvectors.NewRegular(c1Vals))

	// Assign filters:
	// filterA: all 1s (select all rows from the small table)
	// filterB: 1 for the first sizeSmall rows, 0 for the rest
	filterAVals := make([]field.Element, sizeSmall)
	filterBVals := make([]field.Element, sizeLarge)

	for i := 0; i < sizeSmall; i++ {
		filterAVals[i] = field.One()
		filterBVals[i] = field.One()
	}
	// filterBVals[sizeSmall:] remains zero

	run.AssignColumn("filterA", smartvectors.NewRegular(filterAVals))
	run.AssignColumn("filterB", smartvectors.NewRegular(filterBVals))
}

// Advices returns a list of advices for the module discovery
func (d *ProjectionTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a0")),
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a1")),
	}
}

// PermutationTestCase includes a permutation query in addition to
// the global constraints. Permutation queries are compiled into
// grand product arguments.
type PermutationTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *PermutationTestCase) Name() string {
	return "Permutation"
}

// Define defines the structure with permutation query. It has:
// - Global constraints a + b = c for both modules
// - A permutation query asserting that (a0, b0, c0) and (a1, b1, c1) contain the same rows
func (d *PermutationTestCase) Define(comp *wizard.CompiledIOP) {

	d.wiop = comp

	// Define the first module
	a0 := comp.InsertCommit(0, "a0", d.numRow, true)
	b0 := comp.InsertCommit(0, "b0", d.numRow, true)
	c0 := comp.InsertCommit(0, "c0", d.numRow, true)

	// Second module
	a1 := comp.InsertCommit(0, "a1", d.numRow, true)
	b1 := comp.InsertCommit(0, "b1", d.numRow, true)
	c1 := comp.InsertCommit(0, "c1", d.numRow, true)

	comp.InsertGlobal(0, "global-0", symbolic.Sub(c0, b0, a0))
	comp.InsertGlobal(0, "global-1", symbolic.Sub(c1, b1, a1))

	// Permutation query: (a0, b0, c0) and (a1, b1, c1) should contain the same rows (possibly reordered)
	comp.InsertPermutation(0, "permutation-0", []ifaces.Column{a0, b0, c0}, []ifaces.Column{a1, b1, c1})
}

// Assign assigns the columns for the permutation test case.
// Each column has sequential values 1, 2, 3, ... with c = a + b.
// The second table (a1, b1, c1) is a permutation (reversed) of the first (a0, b0, c0).
func (d *PermutationTestCase) Assign(run *wizard.ProverRuntime) {
	// Create sequential values for the first table (a0, b0, c0)
	// a0: 1, 2, 3, ..., numRow
	// b0: 1, 2, 3, ..., numRow
	// c0: 2, 4, 6, ..., 2*numRow (since c = a + b)
	a0Vals := make([]field.Element, d.numRow)
	b0Vals := make([]field.Element, d.numRow)
	c0Vals := make([]field.Element, d.numRow)

	for i := 0; i < d.numRow; i++ {
		a0Vals[i] = field.NewElement(uint64(i + 1))
		b0Vals[i] = field.NewElement(uint64(i + 1))
		c0Vals[i] = field.NewElement(uint64(2 * (i + 1))) // c = a + b
	}

	// Create the second table (a1, b1, c1) as a permutation (reversed) of the first
	a1Vals := make([]field.Element, d.numRow)
	b1Vals := make([]field.Element, d.numRow)
	c1Vals := make([]field.Element, d.numRow)

	for i := 0; i < d.numRow; i++ {
		// Reverse the order - this is a general permutation that crosses segments
		a1Vals[d.numRow-1-i] = field.NewElement(uint64(i + 1))
		b1Vals[d.numRow-1-i] = field.NewElement(uint64(i + 1))
		c1Vals[d.numRow-1-i] = field.NewElement(uint64(2 * (i + 1)))
	}

	run.AssignColumn("a0", smartvectors.NewRegular(a0Vals))
	run.AssignColumn("b0", smartvectors.NewRegular(b0Vals))
	run.AssignColumn("c0", smartvectors.NewRegular(c0Vals))
	run.AssignColumn("a1", smartvectors.NewRegular(a1Vals))
	run.AssignColumn("b1", smartvectors.NewRegular(b1Vals))
	run.AssignColumn("c1", smartvectors.NewRegular(c1Vals))
}

// Advices returns a list of advices for the module discovery
func (d *PermutationTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a0")),
		distributed.SameSizeAdvice("module-0", d.wiop.Columns.GetHandle("a1")),
	}
}

// FibExtTestCase tests that ReceivedValuesGlobal correctly handles extension-field
// values across segment boundaries.
//
// The column extFib forms an arithmetic sequence over F_p^4:
//
//	extFib[i] = (i+1) * delta   where delta = 1 + 2u
//
// The global constraint extFib[i] - extFib[i-1] - delta = 0 forces each segment
// to receive the last value of the previous segment as a genuine extension-field
// element (non-zero upper coordinates), exercising the non-base code path in
// ReceivedValuesGlobal.
//
// At segment 0 the implicit "received" value is 0_fext, so extFib[0] = delta.
type FibExtTestCase struct {
	numRow int
	wiop   *wizard.CompiledIOP
}

func (d *FibExtTestCase) Name() string {
	return "FibExt"
}

// Define registers the single extension-field column and the step constraint.
func (d *FibExtTestCase) Define(comp *wizard.CompiledIOP) {
	d.wiop = comp

	// extFib lives in F_p^4 (isBase = false).
	extFib := comp.InsertCommit(0, "extFib", d.numRow, false)

	// delta = 1 + 2u  — a non-trivial extension-field element so that the
	// values sent across segment boundaries are genuinely extension-field and
	// not merely lifted base-field elements.
	delta := fext.NewFromInt(1, 2, 0, 0)

	// extFib[i] - extFib[i-1] - delta = 0  for all i.
	// At row 0 of each segment extFib[-1] is the last value of the previous
	// segment, transmitted as a ReceivedValuesGlobal extension-field entry.
	comp.InsertGlobal(0, "extFib-step",
		symbolic.Sub(
			ifaces.ColumnAsVariable(extFib),
			ifaces.ColumnAsVariable(column.Shift(extFib, -1)),
			symbolic.NewConstant(delta),
		),
	)
}

// Assign fills extFib with extFib[i] = (i+1)*delta.
//
// The first segment implicitly receives 0_fext, so the constraint
// extFib[0] - 0 - delta = 0  forces extFib[0] = delta.  By induction
// every subsequent value follows from the step constraint.
func (d *FibExtTestCase) Assign(run *wizard.ProverRuntime) {
	delta := fext.NewFromInt(1, 2, 0, 0)

	extFibVals := make([]fext.Element, d.numRow)
	extFibVals[0] = delta
	for i := 1; i < d.numRow; i++ {
		extFibVals[i].Add(&extFibVals[i-1], &delta)
	}

	run.AssignColumn("extFib", smartvectors.NewRegularExt(extFibVals))
}

// Advices returns the module-discovery advice for FibExtTestCase.
func (d *FibExtTestCase) Advices() []*distributed.ModuleDiscoveryAdvice {
	return []*distributed.ModuleDiscoveryAdvice{
		distributed.SameSizeAdvice("fib-ext-module", d.wiop.Columns.GetHandle("extFib")),
	}
}
