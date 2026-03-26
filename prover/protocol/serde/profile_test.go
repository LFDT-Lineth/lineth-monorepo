package serde_test

// Profile non-regression tests.
//
// The first run (or any run with -update) generates golden files under
// testdata/.  Inspect them manually to validate correctness, then commit
// them.  Subsequent runs compare the live output against those files.
//
// Update golden files:
//
//	go test ./protocol/serde/ -run TestProfile -update

import (
	"bytes"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/stretchr/testify/require"
)

// update rewrites the golden files instead of comparing against them.
var update = flag.Bool("update", false, "rewrite testdata golden files")

const testdataDir = "testdata"

// ---------------------------------------------------------------------------
// Local helper types (kept simple to make the output easy to inspect)
// ---------------------------------------------------------------------------

type flatScalars struct {
	A int
	B int32
	C bool
}

type withString struct {
	Name  string
	Score int
}

type inner struct {
	X int64
	Y int64
}

type nestedInline struct {
	Header inner
	Count  int
}

type withPointer struct {
	Val *inner
}

type withInterface struct {
	Data any
}

type withSliceOfInterfaces struct {
	Items []any
}

type withPODSlice struct {
	Elems []field.Element
}

type dedupPointers struct {
	A *inner
	B *inner // same pointer as A
}

type withMap struct {
	Labels map[string]int
}

type withMutex struct {
	Mu    *sync.Mutex
	Value int
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

func profileTestCases(t *testing.T) []struct {
	name string
	v    any
} {
	t.Helper()

	// Shared objects used to exercise deduplication paths.
	sharedInner := &inner{X: 42, Y: 99}

	col, sym, compiledIOP := buildWizardFixtures(t)

	return []struct {
		name string
		v    any
	}{
		// 1. Empty struct
		{
			name: "empty-struct",
			v:    struct{}{},
		},
		// 2. Flat scalar fields (int, int32, bool)
		{
			name: "flat-scalars",
			v:    flatScalars{A: 7, B: -3, C: true},
		},
		// 3. Struct with a string field
		{
			name: "string-field",
			v:    withString{Name: "hello", Score: 100},
		},
		// 4. Pointer to an int
		{
			name: "pointer-to-int",
			v: func() *int {
				n := 42
				return &n
			}(),
		},
		// 5. Nil pointer
		{
			name: "nil-pointer",
			v:    (*inner)(nil),
		},
		// 6. Slice of plain ints
		{
			name: "slice-of-ints",
			v:    []int{10, 20, 30, 40, 50},
		},
		// 7. Slice of field.Element (POD bulk-copy path)
		{
			name: "slice-of-field-elements",
			v:    withPODSlice{Elems: vector.ForTest(0, 1, 2, 3, 4, 5, 6, 7)},
		},
		// 8. Inline nested struct (no pointer indirection)
		{
			name: "nested-inline-struct",
			v:    nestedInline{Header: inner{X: 1, Y: 2}, Count: 5},
		},
		// 9. Struct with a pointer field
		{
			name: "pointer-field",
			v:    withPointer{Val: &inner{X: 10, Y: 20}},
		},
		// 10. Struct with an interface field holding a scalar
		{
			name: "interface-field-scalar",
			v:    withInterface{Data: int(7)},
		},
		// 11. Struct with a slice of interfaces (indirect slice path)
		{
			name: "slice-of-interfaces",
			v: withSliceOfInterfaces{Items: []any{
				int(1), int(2), int(3),
			}},
		},
		// 12. Map[string]int
		{
			name: "map-string-to-int",
			v:    withMap{Labels: map[string]int{"a": 1, "b": 2, "c": 3}},
		},
		// 13. Pointer deduplication: two fields share the same *inner
		{
			name: "dedup-pointers",
			v:    dedupPointers{A: sharedInner, B: sharedInner},
		},
		// 14. String interning: same ifaces.ColID value used in two fields
		{
			name: "string-interning",
			v: struct {
				ID1 ifaces.ColID
				ID2 ifaces.ColID
			}{
				ID1: ifaces.ColID("SHARED_ID"),
				ID2: ifaces.ColID("SHARED_ID"),
			},
		},
		// 15. big.Int (custom codec path)
		{
			name: "big-int",
			v: func() *big.Int {
				b, _ := new(big.Int).SetString("0xdeadbeefcafebabe", 0)
				return b
			}(),
		},
		// 16. sync.Mutex (marshalled-as-empty custom codec)
		{
			name: "mutex",
			v:    withMutex{Mu: &sync.Mutex{}, Value: 99},
		},
		// 17. A wizard column (column.Natural via custom codec + UUID dedup)
		{
			name: "wizard-column",
			v:    col,
		},
		// 18. A symbolic expression tree (pointer tree, deep nesting)
		{
			name: "symbolic-expression",
			v:    sym,
		},
		// 19. A small CompiledIOP (commit + univariate query + dummy compile)
		{
			name: "compiled-iop",
			v:    compiledIOP,
		},
		// 20. smartvectors.Regular – the most common large-data type in the system
		{
			name: "smartvector-regular",
			v:    smartvectors.NewRegular(vector.ForTest(rangeInts(256)...)),
		},
	}
}

// ---------------------------------------------------------------------------
// Test driver
// ---------------------------------------------------------------------------

func TestProfile(t *testing.T) {
	require.NoError(t, os.MkdirAll(testdataDir, 0o755))

	for _, tc := range profileTestCases(t) {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root, err := serde.Profile(tc.v)
			require.NoError(t, err, "Profile() failed")

			var buf bytes.Buffer
			require.NoError(t, serde.WriteProfileTo(root, &buf))

			goldenPath := filepath.Join(testdataDir, fmt.Sprintf("profile_%s.txt", tc.name))

			if *update || !fileExists(goldenPath) {
				require.NoError(t,
					os.WriteFile(goldenPath, buf.Bytes(), 0o600),
					"writing golden file",
				)
				t.Logf("wrote golden file: %s", goldenPath)
				return
			}

			golden, err := os.ReadFile(goldenPath)
			require.NoError(t, err, "reading golden file %s", goldenPath)
			require.Equal(t, string(golden), buf.String(),
				"profile output differs from golden file %s\nRun with -update to refresh", goldenPath)
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildWizardFixtures returns a column, a symbolic expression, and a compiled
// IOP — all built from a shared CompiledIOP so UUID deduplication fires.
func buildWizardFixtures(t *testing.T) (ifaces.Column, *symbolic.Expression, *wizard.CompiledIOP) {
	t.Helper()

	comp := wizard.NewCompiledIOP()
	a := comp.InsertColumn(0, "a", 16, column.Committed, true)
	b := comp.InsertColumn(0, "b", 16, column.Committed, true)
	comp.InsertUnivariate(0, "q", []ifaces.Column{a, b})

	// Build a symbolic expression: Add(Mul(a, b), Constant(1))
	expr := symbolic.Add(symbolic.Mul(a, b), symbolic.NewConstant(1))

	// Compile so we exercise the full CompiledIOP graph.
	compiled := wizard.Compile(
		func(bld *wizard.Builder) {
			c := bld.RegisterCommit("col", 16)
			bld.InsertUnivariate(0, "u", []ifaces.Column{c})
		},
		dummy.Compile,
	)
	_ = comp.InsertCoin(0, "coin", coin.FieldExt)

	return a, expr, compiled
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// rangeInts returns [0, 1, ..., n-1] for use with vector.ForTest.
func rangeInts(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}
