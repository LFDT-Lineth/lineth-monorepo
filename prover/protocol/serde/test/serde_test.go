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
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/recursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/selfrecursion"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerdeValue(t *testing.T) {
	testCases := []struct {
		V    any
		Name string
	}{
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
		{
			Name: "boxed-slice",
			V: func() any {
				return struct {
					A any
				}{A: []int{1, 2, 3}}
			}(),
		},
		{
			Name: "boxed-array",
			V: func() any {
				return struct {
					A any
				}{A: [8]int{1, 2, 3}}
			}(),
		},
		{
			Name: "boxed-map",
			V: func() any {
				return struct {
					A any
				}{A: map[string]int{"a": 1, "b": 2, "c": 3}}
			}(),
		},
		{
			Name: "boxed-set",
			V: func() any {
				return struct {
					A any
				}{A: map[dummy.DoneOperation]struct{}{
					dummy.DoneOperation{Type: 1, Round: 0, PosInRound: 0}: {},
					dummy.DoneOperation{Type: 2, Round: 0, PosInRound: 0}: {},
					dummy.DoneOperation{Type: 3, Round: 0, PosInRound: 0}: {},
				}}
			}(),
		},
		{
			Name: "boxed-slice-of-ptr",
			V: func() any {
				a, b, c := 1, 2, 3
				return struct {
					A any
				}{
					A: []*int{&a, &b, &c},
				}
			}(),
		},
	}

	for i := 0; i < len(testCases); i++ {

		t.Run(fmt.Sprintf("test-case-%v/%v", i, testCases[i].Name), func(t *testing.T) {

			msg, err := serde.Serialize(testCases[i].V)
			require.NoError(t, err)

			// t.Logf("testcase=%v, msg=%v\n", i, string(msg))

			unmarshaled := reflect.New(reflect.TypeOf(testCases[i].V)).Interface()
			err = serde.Deserialize(msg, unmarshaled)
			require.NoError(t, err)

			unmarshalledDereferenced := reflect.ValueOf(unmarshaled).Elem().Interface()
			if !serde.DeepCmp(testCases[i].V, unmarshalledDereferenced, false) {
				t.Errorf("Mismatch in exported fields after full serde value")
			}

		})
	}
}

func TestSerdeSliceOfSmartVectors(t *testing.T) {
	// 1. Setup Data
	row0 := smartvectors.NewRegular(vector.ForTest(1, 2, 3))
	row1 := smartvectors.NewRegular(vector.ForTest(10, 20))
	row2 := smartvectors.NewRegular(vector.ForTest(100, 200, 300, 400))

	// Construct the container
	container := &MatrixContainer{
		Name: "VortexMatrixInMemory",
		// Simulate the interface slice (EncodedMatrix)
		Matrix: []smartvectors.SmartVector{row0, row1, row2},
		// Simulate concrete slice for comparison
		DirectSlice: []*smartvectors.Regular{row0, row1, row2},
	}

	// 2. In-Memory Round Trip
	t.Log("Serializing MatrixContainer (In-Memory)...")
	b, err := serde.Serialize(container)
	if err != nil {
		t.Fatalf("Serialization failed: %v", err)
	}
	t.Logf("Serialized size: %d bytes", len(b))

	var loaded MatrixContainer
	if err := serde.Deserialize(b, &loaded); err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// 3. Verification

	// A. Basic Fields
	if loaded.Name != "VortexMatrixInMemory" {
		t.Errorf("Name mismatch: got %s", loaded.Name)
	}

	// B. Verify Matrix (Slice of Interfaces)
	t.Log("Verifying Matrix (Slice of Interfaces)...")
	if len(loaded.Matrix) != 3 {
		t.Fatalf("Matrix length mismatch: got %d, want 3", len(loaded.Matrix))
	}

	// C. Verify DirectSlice
	t.Log("Verifying DirectSlice...")
	if len(loaded.DirectSlice) != 3 {
		t.Fatalf("DirectSlice length mismatch: got %d, want 3", len(loaded.DirectSlice))
	}

	// D. Check Dedup/Referencing
	// In the loaded object, Matrix[0] (interface wrapper) and DirectSlice[0] (concrete pointer)
	// should point to the exact same underlying struct in memory.
	if loaded.Matrix[0] != loaded.DirectSlice[0] {
		t.Errorf("Dedup failed: Matrix[0] and DirectSlice[0] do not point to the same object")
	}

	// E. DeepCmp Sanity
	if !serde.DeepCmp(container, &loaded, false) {
		t.Error("Global DeepCmp failed")
	} else {
		t.Log("Global DeepCmp passed")
	}
}

func TestStoreAndColumnIntegrity(t *testing.T) {
	// todo @gusiri: serde pointer deduplication does not preserve heap identity
	// for Natural handles and Coin pointers after deserialize. The logical data
	// is correct (DeepCmp passes) but pointer equality checks fail.
	t.Skip("serde pointer deduplication does not preserve heap identity after deserialize")
	// 1. Setup Original State
	originalStore := column.NewStore()

	// Add a column - this creates the internal maps and slices
	// We cast to column.Natural to access the unexported 'store' pointer later
	natValue := originalStore.AddToRound(0, "column_a", 4, column.Committed, true).(column.Natural)

	// Create a shared Coin instance
	sharedCoin := coin.NewInfo("beta", coin.FieldExt, 1)

	// Build a container where multiple pointers point to the EXACT same RAM addresses
	type Container struct {
		Store *column.Store
		Col1  *column.Natural
		Col2  *column.Natural
		Coin1 *coin.Info
		Coin2 *coin.Info
	}

	input := &Container{
		Store: originalStore,
		Col1:  &natValue,
		Col2:  &natValue, // Point to the same handle
		Coin1: &sharedCoin,
		Coin2: &sharedCoin, // Point to the same coin
	}

	// 2. Serialize
	data, err := serde.Serialize(input)
	require.NoError(t, err, "Serialization failed")

	// 3. Deserialize
	var output Container
	err = serde.Deserialize(data, &output)
	require.NoError(t, err, "Deserialization failed")

	// 4. THE VALIDATION

	// A. Logical Field Validation
	// Proves data was reconstructed correctly despite the Pack/Unpack cycle.
	require.True(t, serde.DeepCmp(input, &output, false), "Logical data mismatch")

	// B. Store Deduplication (Handle -> Root)
	// Both handles must point to the same Store instance found at the root.
	ptrRootStore := reflect.ValueOf(output.Store).Pointer()
	ptrCol1InternalStore := reflect.ValueOf(*output.Col1).FieldByName("store").Pointer()
	ptrCol2InternalStore := reflect.ValueOf(*output.Col2).FieldByName("store").Pointer()

	assert.Equal(t, ptrRootStore, ptrCol1InternalStore, "Col1 handle should share the root Store instance")
	assert.Equal(t, ptrCol1InternalStore, ptrCol2InternalStore, "Col1 and Col2 handles must share the same Store instance")

	// C. Handle Deduplication (Handle -> Handle)
	// This proves the library didn't create two Natural handles for the same offset.
	assert.Equal(t, reflect.ValueOf(output.Col1).Pointer(), reflect.ValueOf(output.Col2).Pointer(),
		"Natural handles themselves were not deduplicated in the ptrMap")

	// D. Coin Deduplication (Structural Type)
	// Proves that standard pointers (no custom handlers) are also correctly deduplicated.
	assert.Equal(t, reflect.ValueOf(output.Coin1).Pointer(), reflect.ValueOf(output.Coin2).Pointer(),
		"Coin pointers should point to the exact same heap address")

	// E. Functional Verification
	// Final check that the private internal maps of the Store are actually working.
	assert.True(t, output.Store.Exists("column_a"), "Private indicesByNames map was not restored")
	assert.Equal(t, 4, output.Store.GetSize("column_a"), "Private byRounds slice was not restored")
}

func TestStoreMutationIntegrity(t *testing.T) {
	// This test ensures that because pointers are shared, a mutation in the Store
	// is visible through the Column handle, just like in real Go RAM.

	store := column.NewStore()
	col := store.AddToRound(0, "col", 8, column.Committed, true).(column.Natural)

	type Root struct {
		Store *column.Store
		Col   *column.Natural
	}

	data, _ := serde.Serialize(&Root{store, &col})

	var decoded Root
	serde.Deserialize(data, &decoded)

	// Mutate the state in the Store
	decoded.Store.MarkAsIgnored("col")

	// Check if the handle (which points to the same Store) sees the change
	assert.True(t, decoded.Col.Status() == column.Ignored,
		"Mutation in shared Store should be visible through the Natural handle")
}

func TestSerdePrecompIOP_Mem(t *testing.T) {
	// 1. Create a concrete vector (simulating the precomputed polynomial)
	// We make it large enough to potentially trigger SIMD optimizations in DeepCmp
	const size = 16
	vals := make([]field.Element, size)
	for i := 0; i < size; i++ {
		vals[i].SetUint64(1)
	}
	// Use the concrete type that implements SmartVector
	vec := smartvectors.Regular(vals)

	// 2. Define the protocol
	define := func(b *wizard.Builder) {
		// RegisterPrecomputed puts this vector into comp.Precomputed (Map[ColID]Interface)
		b.RegisterPrecomputed("P1", &vec)
	}

	// 3. Compile to get the object graph
	comp := wizard.Compile(define, dummy.Compile)

	// 4. Serialize (In-Memory)
	// This uses the encoder, but writes to a bytes.Buffer on the Heap.
	buf, err := serde.Serialize(comp)
	require.NoError(t, err)

	// 5. Deserialize
	var decoded *wizard.CompiledIOP
	err = serde.Deserialize(buf, &decoded)
	require.NoError(t, err)

	// 6. Compare
	// This usually PASSES because Heap allocations are aligned by the Go runtime.
	if !serde.DeepCmp(comp, decoded, true) {
		t.Fatal("Mismatch in In-Memory test")
	}
}

const (
	reproDir      = "files"
	reproFilename = "precomp_bug.bin"
)

// helper to build the exact same object for both Store (source) and Load (truth)
func buildPrecompIOP() *wizard.CompiledIOP {
	// Create a vector of 1s.
	// We use 16 elements to potentially trigger SIMD/AVX logic in underlying libs.
	const size = 16
	vals := make([]field.Element, size)
	for i := 0; i < size; i++ {
		vals[i].SetUint64(1)
	}
	vec := smartvectors.Regular(vals)

	define := func(b *wizard.Builder) {
		b.RegisterPrecomputed("P1", &vec)
		_ = b.RegisterRandomCoin("COIN", coin.FieldExt)
		_ = b.RegisterCommit("Q", 16)

	}
	return wizard.Compile(define,
		vortex.Compile(
			2, false,
			vortex.ForceNumOpenedColumns(16),
			vortex.WithSISParams(&ringsis.StdParams),
		),
		dummy.Compile)
}

func TestSerdePrecompIOP_Store(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	// 1. Setup Environment
	require.NoError(t, os.MkdirAll(reproDir, 0755))
	path := filepath.Join(reproDir, reproFilename)

	// 2. Build Object
	comp := buildPrecompIOP()

	// 3. Store to Disk
	// If the encoder lacks alignment logic, this will write the vector data
	// at an unaligned offset (e.g. 33 + headers).
	err := serde.StoreToDisk(path, comp, false)
	require.NoError(t, err)

	t.Logf("Stored artifact to %s", path)
}

func TestSerdePrecompIOP_Load(t *testing.T) {
	t.Skipf("the test is a development/debug/integration test. It is not needed for CI")
	path := filepath.Join(reproDir, reproFilename)
	_, err := os.Stat(path)
	require.NoError(t, err, "Artifact not found. Run TestSerdePrecompIOP_Store first.")

	// 1. Build Expected (Truth)
	expected := buildPrecompIOP()

	// 2. Load from Disk (Memory Map)
	// mmap will return a pointer to a page-aligned address.
	// If the internal offset is unaligned (e.g. 33), the effective address
	// for the slice data will be Base + 33 (Unaligned).
	var loaded *wizard.CompiledIOP
	closer, err := serde.LoadFromDisk(path, &loaded, false)
	require.NoError(t, err)
	defer closer.Close()

	// 3. Compare
	// This reads the uint64s from the loaded object.
	// On x86 with AVX (or via gnark-crypto), unaligned reads may result in garbage data.
	if !serde.DeepCmp(expected, loaded, true) {
		t.Fatal("Mismatch in Precomputed Vector (likely due to unaligned read)")
	} else {
		t.Logf("Check passed")
		_ = os.Remove(path)
	}
}
