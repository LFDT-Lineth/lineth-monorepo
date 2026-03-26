package serde_test

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// Shared structure for test cases
type serdeTestCase struct {
	Name string
	V    any
}

// getSerdeTestCases generates fresh instances of the test objects.
// We use a function so 'Store' and 'Load' get their own distinct pointers/memory.
func getSerdeTestCases() []serdeTestCase {
	// Ensure types are registered (idempotent)
	serde.RegisterImplementation(string(""))
	serde.RegisterImplementation(ifaces.ColID(""))
	serde.RegisterImplementation(column.Natural{})
	serde.RegisterImplementation(column.Shifted{})
	serde.RegisterImplementation(verifiercol.ConstCol{})
	serde.RegisterImplementation(verifiercol.FromYs{})
	serde.RegisterImplementation(verifiercol.FromAccessors{})
	serde.RegisterImplementation(accessors.FromPublicColumn{})
	serde.RegisterImplementation(accessors.FromConstAccessor{})
	serde.RegisterImplementation(query.UnivariateEval{})

	return []serdeTestCase{
		{
			Name: "random-string",
			V:    "someRandomString",
		},
		{
			Name: "random-column-id-ptr",
			V: func() any {
				var s = ifaces.ColID("someIndirectedString")
				return &s
			}(),
		},
		{
			Name: "some-string-ptr",
			V: func() any {
				// It's important to not provide an untyped string under
				// the interface because the type cannot be serialized.
				var s any = string("someStringUnderIface")
				return &s
			}(),
		},
		{
			Name: "some-col-id-ptr",
			V: func() any {
				var id = ifaces.ColID("newTypeUnderIface")
				var s any = &id
				return &s
			}(),
		},
		{
			Name: "query-id",
			V:    ifaces.QueryID("QueryID"),
		},
		{
			Name: "dedup-id",
			V: func() any {
				var (
					colID    = ifaces.ColID("COL_ID")
					coinName = coin.Name("COIN_NAME")
					queryID  = ifaces.QueryID("QUERY_ID")
				)

				dupStruct := struct {
					ColID1    ifaces.ColID
					CoinName1 coin.Name
					QueryID1  ifaces.QueryID
					ColID2    ifaces.ColID
					CoinName2 coin.Name
					QueryID2  ifaces.QueryID
				}{
					ColID1:    colID,
					CoinName1: coinName,
					QueryID1:  queryID,
					ColID2:    colID,
					CoinName2: coinName,
					QueryID2:  queryID,
				}

				return dupStruct
			}(),
		},
		{
			Name: "natural-column",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				return comp.InsertColumn(0, "myNaturalColumn", 16, column.Committed, true)
			}(),
		},
		{
			Name: "shifted-column",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				nat := comp.InsertColumn(0, "myNaturalColumn", 16, column.Committed, true)
				return column.Shift(nat, 2)
			}(),
		},
		{
			Name: "concat-tiny-columns",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				col := verifiercol.NewConcatTinyColumns(
					comp,
					8,
					fext.Element{},
					comp.InsertColumn(0, "a", 1, column.Proof, true),
					comp.InsertColumn(0, "b", 1, column.Proof, true),
					comp.InsertColumn(0, "c", 1, column.Proof, true),
				)
				return &col
			}(),
		},
		{
			Name: "from-public-column",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				nat := comp.InsertColumn(0, "myNaturalColumn", 16, column.Proof, true)
				return accessors.NewFromPublicColumn(nat, 2)
			}(),
		},
		{
			Name: "from-const-accessor",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				c := comp.InsertCoin(0, "myCoin", coin.FieldExt)
				return accessors.NewFromCoin(c)
			}(),
		},
		{
			Name: "univariate-eval",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				q := comp.InsertUnivariate(0, "q", []ifaces.Column{a, b})
				return q
			}(),
		},
		{
			Name: "from-ys",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				q := comp.InsertUnivariate(0, "q", []ifaces.Column{a, b})
				return verifiercol.NewFromYs(comp, q, []ifaces.ColID{a.GetColID(), b.GetColID()})
			}(),
		},
		{
			Name: "integer-vec-coin",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				c := comp.InsertCoin(0, "myCoin", coin.IntegerVec, 16, 16)
				return c
			}(),
		},
		{
			Name: "from-integer-vec-coin",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				c := comp.InsertCoin(0, "myCoin", coin.IntegerVec, 16, 16)
				return verifiercol.NewFromIntVecCoin(comp, c)
			}(),
		},
		{
			Name: "verifier-col-from-accessors",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Proof, true)
				b := comp.InsertColumn(0, "b", 16, column.Proof, true)
				return verifiercol.NewFromAccessors(
					[]ifaces.Accessor{
						accessors.NewFromPublicColumn(a, 2),
						accessors.NewFromPublicColumn(b, 2),
					},
					fext.Element{}, 16)
			}(),
		},
		{
			Name: "const-col",
			V:    verifiercol.NewConstantCol(field.Element{}, 16, ""),
		},
		{
			Name: "new-coin",
			V:    coin.NewInfo("foo", coin.IntegerVec, 16, 16, 1),
		},
		{
			Name: "bigint-zero",
			V:    big.NewInt(0),
		},
		{
			Name: "bigint-one",
			V:    big.NewInt(1),
		},
		{
			Name: "bigint-minus-one",
			V:    big.NewInt(-1),
		},
		{
			Name: "bigint-max-uint256",
			V: func() any {
				v, ok := new(big.Int).SetString("0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 0)
				if !ok {
					panic("bigint does not work")
				}
				return v
			}(),
		},
		{
			Name: "field-zero",
			V:    field.NewElement(0),
		},
		{
			Name: "field-one",
			V:    field.NewElement(1),
		},
		{
			Name: "field-2**248-1",
			V: func() any {
				v, err := new(field.Element).SetString("0x00ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
				if err != nil {
					utils.Panic("field does not work: %v", err)
				}
				return v
			}(),
		},
		{
			Name: "vector-1234",
			V:    vector.ForTest(0, 1, 2, 3, 4, 5, 5, 6, 7),
		},
		{
			Name: "symbolic-add-with-shifted",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				aNext := column.Shift(a, 2)
				return symbolic.Add(a, aNext)
			}(),
		},
		{
			Name: "symbolic-add",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				return symbolic.Add(a, b)
			}(),
		},
		{
			Name: "symbolic-mul",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				return symbolic.Mul(a, b)
			}(),
		},
		{
			Name: "symbolic-new-variable",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				return symbolic.NewVariable(a)
			}(),
		},
		{
			Name: "symbolic-constant-0",
			V:    symbolic.NewConstant(0),
		},
		{
			Name: "symbolic-constant-1",
			V:    symbolic.NewConstant(1),
		},
		{
			Name: "symbolic-constant-minus-1",
			V:    symbolic.NewConstant(-1),
		},
		{
			Name: "symbolic-intricate-expression",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				c := comp.InsertColumn(0, "c", 16, column.Committed, true)
				d := comp.InsertColumn(0, "d", 16, column.Committed, true)
				return symbolic.Add(symbolic.Mul(symbolic.Add(a, b), symbolic.Add(c, d)), symbolic.NewConstant(1))
			}(),
		},
		{
			Name: "mimc-query",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := [8]ifaces.Column{
					comp.InsertColumn(0, "a0", 16, column.Committed, true),
					comp.InsertColumn(0, "a1", 16, column.Committed, true),
					comp.InsertColumn(0, "a2", 16, column.Committed, true),
					comp.InsertColumn(0, "a3", 16, column.Committed, true),
					comp.InsertColumn(0, "a4", 16, column.Committed, true),
					comp.InsertColumn(0, "a5", 16, column.Committed, true),
					comp.InsertColumn(0, "a6", 16, column.Committed, true),
					comp.InsertColumn(0, "a7", 16, column.Committed, true),
				}
				b := [8]ifaces.Column{
					comp.InsertColumn(0, "b0", 16, column.Committed, true),
					comp.InsertColumn(0, "b1", 16, column.Committed, true),
					comp.InsertColumn(0, "b2", 16, column.Committed, true),
					comp.InsertColumn(0, "b3", 16, column.Committed, true),
					comp.InsertColumn(0, "b4", 16, column.Committed, true),
					comp.InsertColumn(0, "b5", 16, column.Committed, true),
					comp.InsertColumn(0, "b6", 16, column.Committed, true),
					comp.InsertColumn(0, "b7", 16, column.Committed, true),
				}
				c := [8]ifaces.Column{
					comp.InsertColumn(0, "c0", 16, column.Committed, true),
					comp.InsertColumn(0, "c1", 16, column.Committed, true),
					comp.InsertColumn(0, "c2", 16, column.Committed, true),
					comp.InsertColumn(0, "c3", 16, column.Committed, true),
					comp.InsertColumn(0, "c4", 16, column.Committed, true),
					comp.InsertColumn(0, "c5", 16, column.Committed, true),
					comp.InsertColumn(0, "c6", 16, column.Committed, true),
					comp.InsertColumn(0, "c7", 16, column.Committed, true),
				}
				return comp.InsertPoseidon2(0, "mimc", a, b, c, nil)
			}(),
		},
		{
			Name: "nil-expression",
			V: func() any {
				return struct {
					E *symbolic.Expression
				}{}
			}(),
		},
		{
			Name: "map-with-column-as-key",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true).(column.Natural)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true).(column.Natural)
				return map[column.Natural]string{a: "a", b: "b"}
			}(),
		},
		{
			Name: "frontend-variables",
			V:    frontend.Variable(0),
		},
		{
			Name: "frontend-variables",
			V:    frontend.Variable(12),
		},
		{
			Name: "frontend-variables",
			V:    frontend.Variable(-10),
		},
		{
			Name: "two-wiop-in-a-struct",
			V: func() any {

				res := struct {
					A, B *wizard.CompiledIOP
				}{
					A: wizard.NewCompiledIOP(),
					B: wizard.NewCompiledIOP(),
				}

				res.A.InsertColumn(0, "a", 16, column.Committed, true)
				res.B.InsertColumn(0, "b", 16, column.Committed, true)

				return res
			}(),
		},
		{
			Name: "recursion",
			V: func() any {

				wiop := wizard.NewCompiledIOP()
				a := wiop.InsertCommit(0, "a", 1<<10, true)
				wiop.InsertUnivariate(0, "u", []ifaces.Column{a})

				wizard.ContinueCompilation(wiop,
					vortex.Compile(
						2, false,
						vortex.WithOptionalSISHashingThreshold(0),
						vortex.ForceNumOpenedColumns(2),
						vortex.PremarkAsSelfRecursed(),
					),
				)

				rec := wizard.NewCompiledIOP()
				recursion.DefineRecursionOf(rec, wiop, recursion.Parameters{
					MaxNumProof: 1,
					Name:        "recursion",
				})

				return rec
			}(),
		},
		{
			Name: "mutex",
			V:    []*sync.Mutex{{}, {}},
		},
		{
			Name: "nil-horner-query",
			V:    (*query.Horner)(nil),
		},
		{
			Name: "nil-horner-query-in-a-struct",
			V: struct {
				G *query.Horner // G is left as a nil by default
			}{},
		},
		{
			Name: "de-dup-test",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				col := comp.InsertColumn(0, "a", 16, column.Committed, true)

				type S struct {
					A any
					B any
				}

				return S{A: col, B: col}
			}(),
		},
		{
			Name: "de-dup-expr",
			V: func() any {
				comp := wizard.NewCompiledIOP()
				a := comp.InsertColumn(0, "a", 16, column.Committed, true)
				b := comp.InsertColumn(0, "b", 16, column.Committed, true)
				c := comp.InsertColumn(0, "c", 16, column.Committed, true)
				d := comp.InsertColumn(0, "d", 16, column.Committed, true)
				expr := symbolic.Add(symbolic.Mul(symbolic.Add(a, b), symbolic.Add(c, d)), symbolic.NewConstant(1))

				type S struct {
					A *symbolic.Expression
					B *symbolic.Expression
				}

				return S{A: expr, B: expr}

			}(),
		},
		{
			Name: "Self-recursion-compiled-iop",
			V: func() any {
				wiop := wizard.NewCompiledIOP()
				a := wiop.InsertCommit(0, "a", 4, true)
				wiop.InsertUnivariate(0, "u", []ifaces.Column{a})

				// 2. Compile with SelfRecursion enabled
				wizard.ContinueCompilation(wiop,
					vortex.Compile(
						2,
						false,
					),
					selfrecursion.SelfRecurse,
				)
				return wiop
			}(),
		},
	}
}

// PHASE 1: STORE
// Iterates through all test cases and writes them to the "files/" directory.
func TestSerdeValue_Store(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	testDir := "files"
	// Cleanup and recreate directory
	_ = os.RemoveAll(testDir)
	require.NoError(t, os.MkdirAll(testDir, 0755))

	testCases := getSerdeTestCases()

	for i, tc := range testCases {

		t.Run(fmt.Sprintf("Store/%d_%s", i, tc.Name), func(t *testing.T) {
			path := filepath.Join(testDir, fmt.Sprintf("%d_%s.bin", i, tc.Name))

			// Write to disk using standard API (no compression to test raw serde)
			err := serde.StoreToDisk(path, tc.V, false)
			require.NoError(t, err, "StoreToDisk failed")
		})
	}
}

// PHASE 2: LOAD
// Reads the files created by Store, deserializes them, verifies correctness, and deletes them.
func TestSerdeValue_Load(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	testDir := "files"
	testCases := getSerdeTestCases()

	for i, tc := range testCases {

		t.Run(fmt.Sprintf("Load/%d_%s", i, tc.Name), func(t *testing.T) {
			path := filepath.Join(testDir, fmt.Sprintf("%d_%s.bin", i, tc.Name))

			// 1. Prepare destination pointer
			// LoadFromDisk requires a pointer to the type being loaded.
			// Since tc.V is 'any', we use reflection to create a new pointer of that type.
			destPtr := reflect.New(reflect.TypeOf(tc.V))

			// 2. Load
			closer, err := serde.LoadFromDisk(path, destPtr.Interface(), false)
			require.NoError(t, err, "LoadFromDisk failed")

			logrus.Infof("****** Decoding dst:%v", destPtr.Elem().Interface())

			// 3. Compare (DeepCmp)
			// We check destPtr.Elem() because destPtr is *T, but tc.V is T.
			isMatch := serde.DeepCmp(tc.V, destPtr.Elem().Interface(), true)
			if !isMatch {
				t.Errorf("DeepCmp failed: loaded object does not match original")
			}

			// 4. Cleanup
			// CRITICAL: Close the mmap before attempting to delete the file.
			// On Windows, deleting an open file is forbidden. On Linux, it works but leaks resources.
			require.NoError(t, closer.Close(), "Failed to close mmap")
			require.NoError(t, os.Remove(path), "Failed to delete test file")
		})
	}

	// Final cleanup of directory
	_ = os.RemoveAll(testDir)
}

const (
	// Shared directory and filename for the ZkEVM artifact
	testDir       = "files"
	zkEvmFileName = "zkevm.bin"
)

// PHASE 1: STORE
// This test serializes the complex ZkEVM object to disk.
func TestZkEVM_Store(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// 1. Setup directory
	// We clean up before storing to ensure a fresh environment
	_ = os.RemoveAll(testDir)
	require.NoError(t, os.MkdirAll(testDir, 0755))

	path := filepath.Join(testDir, zkEvmFileName)

	t.Logf("Storing ZkEVM object to: %s", path)

	// 2. Store to Disk
	// We use 'false' for compression to test the raw mmap capability
	err := serde.StoreToDisk(path, getTestZkEVM(), false)
	require.NoError(t, err, "StoreToDisk failed for ZkEVM")

	// 3. Verify file creation
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0), "ZkEVM binary file created but is empty")

	t.Logf("Successfully stored ZkEVM. Size: %d bytes", info.Size())
}

// PHASE 2: LOAD
// This test loads the artifact created by TestStoreZkEVM and performs a Deep Compare.
func TestZkEVM_Load(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	path := filepath.Join(testDir, zkEvmFileName)

	// 1. Dependency Check
	// Ensure the artifact from Phase 1 exists
	_, err := os.Stat(path)
	require.NoError(t, err, "ZkEVM artifact missing. Did you run TestStoreZkEVM first?")

	t.Logf("Loading ZkEVM object from: %s", path)

	// 2. Prepare Destination
	// We use reflection to ensure we create the exact pointer type expected by LoadFromDisk
	// z is likely a struct or a pointer to a struct.
	// reflect.New creates a pointer to the type of z.
	destPtr := reflect.New(reflect.TypeOf(getTestZkEVM()))

	// 3. Load from Disk
	closer, err := serde.LoadFromDisk(path, destPtr.Interface(), false)
	require.NoError(t, err, "LoadFromDisk failed for ZkEVM")

	// CRITICAL: Ensure we close the mmap and clean up the file at the end
	defer func() {
		require.NoError(t, closer.Close(), "Failed to close mmap")
		require.NoError(t, os.RemoveAll(testDir), "Failed to cleanup directory")
	}()

	// 4. Verification (DeepCmp)
	// destPtr is a pointer to the type of z. We need to dereference it once (Elem())
	// to compare it against the original 'z'.
	loadedVal := destPtr.Elem().Interface()

	t.Log("Running DeepCmp on ZkEVM... this might take a moment.")

	// FailFast = true to stop on the first error
	isMatch := serde.DeepCmp(getTestZkEVM(), loadedVal, true)

	if !isMatch {
		t.Fatal("DeepCmp failed: Loaded ZkEVM object does not match the original")
	}

	t.Log("Success: ZkEVM serialization/deserialization verified.")
}

const (
	// Directory to store the IOP binary artifacts
	iopArtifactsDir = "files"
)

// PHASE 1: STORE
// Iterates through all scenarios defined in 'serdeScenarios' and persists them to disk.
func TestIOP_Store(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// 1. Setup Environment
	// Clean up previous runs to ensure valid file creation
	_ = os.RemoveAll(iopArtifactsDir)
	require.NoError(t, os.MkdirAll(iopArtifactsDir, 0755))

	for _, scenario := range serdeScenarios {

		// Skip scenarios marked as not for testing
		if !scenario.test {
			continue
		}

		t.Run(fmt.Sprintf("Store/%s", scenario.name), func(subT *testing.T) {
			// 2. Build the Object
			// We use the scenario builder to generate the complex CompiledIOP
			comp := getScenarioComp(&scenario)
			require.NotNil(subT, comp, "Failed to build scenario %s", scenario.name)

			// 3. Define File Path
			path := filepath.Join(iopArtifactsDir, fmt.Sprintf("%s.bin", scenario.name))
			subT.Logf("Storing %s to %s", scenario.name, path)

			// 4. Store to Disk
			// We use 'false' for compression to test raw mapping
			err := serde.StoreToDisk(path, comp, false)
			require.NoError(subT, err, "StoreToDisk failed for %s", scenario.name)

			// 5. Verify File Existence
			info, err := os.Stat(path)
			require.NoError(subT, err)
			require.Greater(subT, info.Size(), int64(0), "File created but empty")
		})
	}
}

// PHASE 2: LOAD
// Loads the artifacts created in Phase 1 and compares them against a freshly built original.
func TestIOP_Load(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// Ensure the artifacts directory exists
	_, err := os.Stat(iopArtifactsDir)
	require.NoError(t, err, "Artifacts directory missing. Did you run TestIOP_Store?")

	// Cleanup files after all load tests complete
	defer func() {
		_ = os.RemoveAll(iopArtifactsDir)
	}()

	for _, scenario := range serdeScenarios {

		if !scenario.test {
			continue
		}

		t.Run(fmt.Sprintf("Load/%s", scenario.name), func(subT *testing.T) {
			path := filepath.Join(iopArtifactsDir, fmt.Sprintf("%s.bin", scenario.name))

			// 1. Rebuild Expected Object (The "Gold Standard")
			// We rebuild it so we have a distinct memory object to compare against
			expected := getScenarioComp(&scenario)

			// 2. Prepare Destination
			// CompiledIOP is a struct, and getScenarioComp returns *wizard.CompiledIOP.
			// LoadFromDisk expects a pointer to the type we want to load.
			// So we pass **wizard.CompiledIOP (which is &loaded where loaded is *wizard.CompiledIOP)
			var loaded *wizard.CompiledIOP

			// 3. Load from Disk
			subT.Logf("Loading %s...", path)
			closer, err := serde.LoadFromDisk(path, &loaded, false)
			require.NoError(subT, err, "LoadFromDisk failed for %s", scenario.name)
			defer closer.Close() // Release mmap

			// 4. Verification (Deep Compare)
			// We use failFast=true to stop immediately on mismatch
			subT.Logf("Verifying %s via DeepCmp...", scenario.name)
			match := serde.DeepCmp(expected, loaded, true)

			if !match {
				subT.Fatalf("DeepCmp Failed: Loaded object for scenario '%s' differs from original build.", scenario.name)
			}
			subT.Logf("Success: %s verified.", scenario.name)
		})
	}
}

// -----------------------------------------------------------------------------
// Test Helpers & Data Generation (Deterministic for I/O consistency)
// -----------------------------------------------------------------------------

// Wrapper struct to test SmartVector inside a larger object graph
type SmartVecContainer struct {
	Label   string
	Vector  *smartvectors.Regular
	Version uint64
}

func TestStoreSmartVector(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// 1. Setup Data
	// vector.ForTest usually creates a []field.Element from integers
	originalData := vector.ForTest(1, 2, 3, 4, 5)
	mySmartVec := smartvectors.NewRegular(originalData)

	container := &SmartVecContainer{
		Label:   "RegularVectorTest",
		Vector:  mySmartVec,
		Version: 2025,
	}

	// 2. Serialize
	t.Log("Serializing SmartVecContainer...")
	b, err := serde.Serialize(container)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	// 3. Save to Disk
	if err := os.MkdirAll("files", 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("files", "smart_vector.bin")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	t.Logf("Wrote %d bytes to %s", len(b), path)
}

func TestLoadSmartVector(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	path := filepath.Join("files", "smart_vector.bin")

	// 1. Read from Disk
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// 2. Deserialize
	var loaded SmartVecContainer
	if err := serde.Deserialize(b, &loaded); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// 3. Verification
	t.Logf("Loaded Label: %s, Version: %d", loaded.Label, loaded.Version)

	if loaded.Label != "RegularVectorTest" {
		t.Errorf("Label mismatch: got %s", loaded.Label)
	}

	if loaded.Vector == nil {
		t.Fatal("Loaded vector is nil")
	}

	// 4. Content Verification
	expectedData := vector.ForTest(1, 2, 3, 4, 5)
	if loaded.Vector.Len() != len(expectedData) {
		t.Fatalf("Length mismatch: got %d, want %d", loaded.Vector.Len(), len(expectedData))
	}

	for i := 0; i < loaded.Vector.Len(); i++ {
		got, _ := loaded.Vector.GetBase(i)
		want := expectedData[i]
		if got != want {
			t.Errorf("Value mismatch at index %d:\nGot:  %v\nWant: %v", i, got, want)
		}
	}

	// 5. DeepCmp Sanity
	// Note: We reconstruct a fresh container for comparison to ensure DeepCmp works on these types
	expectedContainer := &SmartVecContainer{
		Label:   "RegularVectorTest",
		Vector:  smartvectors.NewRegular(expectedData),
		Version: 2025,
	}

	if !serde.DeepCmp(expectedContainer, &loaded, false) {
		t.Error("Global DeepCmp failed for SmartVecContainer")
	} else {
		t.Log("Global DeepCmp passed")
	}
}

// MatrixContainer mimics the structure of vortex.EncodedMatrix/Ctx
type MatrixContainer struct {
	Name string
	// This mimics EncodedMatrix (slice of interfaces)
	Matrix []smartvectors.SmartVector
	// This mimics a direct slice of concrete types
	DirectSlice []*smartvectors.Regular
}

func TestStoreSliceOfSmartVectors(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// 1. Setup Data
	// Create a jagged matrix pattern to test variable lengths
	// Row 0: [1, 2, 3]
	// Row 1: [10, 20]
	// Row 2: [100, 200, 300, 400]

	row0 := smartvectors.NewRegular(vector.ForTest(1, 2, 3))
	row1 := smartvectors.NewRegular(vector.ForTest(10, 20))
	row2 := smartvectors.NewRegular(vector.ForTest(100, 200, 300, 400))

	// Construct the container
	container := &MatrixContainer{
		Name: "VortexMatrixTest",
		// Simulate the interface slice (EncodedMatrix)
		Matrix: []smartvectors.SmartVector{row0, row1, row2},
		// Simulate concrete slice for comparison
		DirectSlice: []*smartvectors.Regular{row0, row1, row2},
	}

	// 2. Serialize
	t.Log("Serializing MatrixContainer...")
	b, err := serde.Serialize(container)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}

	// 3. Save to Disk
	if err := os.MkdirAll("files", 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("files", "matrix_vector.bin")
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	t.Logf("Wrote %d bytes to %s", len(b), path)
}

func TestLoadSliceOfSmartVectors(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	path := filepath.Join("files", "matrix_vector.bin")

	// 1. Read
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// 2. Deserialize
	var loaded MatrixContainer
	if err := serde.Deserialize(b, &loaded); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// 3. Verify Basic Fields
	if loaded.Name != "VortexMatrixTest" {
		t.Errorf("Name mismatch: got %s", loaded.Name)
	}

	// 4. Verify Matrix (Slice of Interfaces)
	t.Log("Verifying Matrix (Slice of Interfaces)...")
	if len(loaded.Matrix) != 3 {
		t.Fatalf("Matrix length mismatch: got %d, want 3", len(loaded.Matrix))
	}

	// 5. Verify DirectSlice (Slice of Concrete Pointers)
	t.Log("Verifying DirectSlice...")
	if len(loaded.DirectSlice) != 3 {
		t.Fatalf("DirectSlice length mismatch: got %d, want 3", len(loaded.DirectSlice))
	}

	// 6. Check Referencing/Dedup
	if loaded.Matrix[0] != loaded.DirectSlice[0] {
		t.Log("Warning: Deduplication might be failing (Matrix[0] != DirectSlice[0])")
	}

	// -------------------------------------------------------------------------
	// 7. Global DeepCmp Sanity
	// -------------------------------------------------------------------------
	// We verify that the entire object graph matches bit-for-bit.
	// We reconstruct the expected object exactly as it was in TestStore.

	row0 := smartvectors.NewRegular(vector.ForTest(1, 2, 3))
	row1 := smartvectors.NewRegular(vector.ForTest(10, 20))
	row2 := smartvectors.NewRegular(vector.ForTest(100, 200, 300, 400))

	expectedContainer := &MatrixContainer{
		Name:        "VortexMatrixTest",
		Matrix:      []smartvectors.SmartVector{row0, row1, row2},
		DirectSlice: []*smartvectors.Regular{row0, row1, row2},
	}

	// Note: failFast=false allows us to see all diffs if it fails
	if !serde.DeepCmp(expectedContainer, &loaded, false) {
		t.Error("Global DeepCmp failed: Deserialized object does not match Original")
	} else {
		t.Log("Global DeepCmp passed")
	}
}
