package serialization

import (
	"bytes"
	"fmt"
	"hash"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	cs "github.com/consensys/gnark/constraint/bls12-377"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/google/uuid"
)

var (
	SerdeStructTag = "serde"

	// Do not serialize fields with this tag
	SerdeStructTagOmit = "omit"

	// Serialize this but don't include it in test comparisions to prevent OOM
	SerdeStructTagTestOmit = "test_omit"
)

// Global type constants for reflection-based type checking.
// These define the reflect.Type of key protocol-specific types, used to identify
// special types during serialization and deserialization.
var (
	TypeOfColumnNatural      = reflect.TypeOf(column.Natural{})
	TypeOfColumnID           = reflect.TypeOf(ifaces.ColID(""))
	TypeOfCoin               = reflect.TypeOf(coin.Info{})
	TypeOfCoinID             = reflect.TypeOf(coin.Name(""))
	TypeOfQuery              = reflect.TypeOf((*ifaces.Query)(nil)).Elem()
	TypeOfQueryID            = reflect.TypeOf(ifaces.QueryID(""))
	TypeOfCompiledIOPPointer = reflect.TypeOf(&wizard.CompiledIOP{})
	TypeOfCompiledIOP        = reflect.TypeOf(wizard.CompiledIOP{})
	TypeOfStorePtr           = reflect.TypeOf(&column.Store{})
	TypeOfPackedColumn       = reflect.TypeOf(column.PackedNatural{})
	TypeOfPackedStore        = reflect.TypeOf(column.PackedStore{})
	TypeOfVariableMetadata   = reflect.TypeOf((*symbolic.Metadata)(nil)).Elem()
	TypeOfArrayOfExpr        = reflect.TypeOf([]*symbolic.Expression{})
	TypeOfExpressionPtr      = reflect.TypeOf(&symbolic.Expression{})
	TypeOfExpression         = reflect.TypeOf(symbolic.Expression{})
	TypeOfArrayOfInt         = reflect.TypeOf([]int{})
	TypeOfFieldElement       = reflect.TypeOf(field.Element{})
	TypeOfBigInt             = reflect.TypeOf(&big.Int{})
	TypeOfArrOfFieldElement  = reflect.TypeOf([]field.Element{})
	TypeOfPlonkCirc          = reflect.TypeOf(&cs.SparseR1CS{})
	TypeOfArithmetization    = reflect.TypeOf(arithmetization.Arithmetization{})
	TypeOfFrontendVariable   = reflect.TypeOf((*frontend.Variable)(nil)).Elem()
	TypeOfHashFuncGenerator  = reflect.TypeOf(func() hash.Hash { return nil })
	TypeOfHashTypeHasher     = reflect.TypeOf(func() hashtypes.Hasher { return hashtypes.Hasher{} })
	TypeOfRingSisKeyPtr      = reflect.TypeOf(&ringsis.Key{})
	TypeOfGnarkFFTDomainPtr  = reflect.TypeOf(&fft.Domain{})
	TypeOfRingSisKeyGenParam = reflect.TypeOf(ringsis.KeyGen{})
	TypeOfMutexPtr           = reflect.TypeOf(&sync.Mutex{})
)

// BackReference represents an integer index into PackedObject arrays (e.g., Columns, Coins).
// It enables efficient reuse of shared objects, avoiding redundant serialization.
type BackReference int

// Serializer manages the serialization process, packing objects into a PackedObject.
// It tracks references to objects (e.g., columns, coins) and collects warnings for non-fatal issues.
type Serializer struct {
	PackedObject     *PackedObject                // The output structure containing serialized data.
	typeMap          map[string]int               // Maps type names to indices in PackedObject.Types.
	pointerMap       map[uintptr]int              // Maps pointer values to indices in PackedObject.Pointers.
	coinMap          map[uuid.UUID]int            // Maps coin UUIDs to indices in PackedObject.Coins.
	coinIdMap        map[string]int               // Maps coin IDs to indices in PackedObject.CoinIDs.
	columnMap        map[uuid.UUID]int            // Maps column UUIDs to indices in PackedObject.Columns.
	columnIdMap      map[string]int               // Maps column IDs to indices in PackedObject.ColumnIDs.
	queryMap         map[uuid.UUID]int            // Maps query UUIDs to indices in PackedObject.Queries.
	queryIDMap       map[string]int               // Maps query IDs to indices in PackedObject.QueryIDs.
	compiledIOPsFast map[*wizard.CompiledIOP]int  // Maps CompiledIOP pointers to indices in PackedObject.CompiledIOP.
	stores           map[*column.Store]int        // Maps Store pointers to indices in PackedObject.Store.
	circuitMap       map[*cs.SparseR1CS]int       // Maps circuit pointers to indices in PackedObject.Circuits.
	exprMap          map[*symbolic.Expression]int // Maps expression pointers to indices in PackedObject.Expressions
	warnings         []string                     // Collects warnings (e.g., unexported fields) for debugging.

	// compiledIOPs map[*wizard.CompiledIOP]int // Maps CompiledIOP pointers to indices in PackedObject.CompiledIOP.
}

// Deserializer manages the deserialization process, reconstructing objects from a PackedObject.
// It caches reconstructed objects to resolve back-references and collects warnings.
type Deserializer struct {
	PackedObject     *PackedObject          // The input structure to deserialize.
	pointedValues    []reflect.Value        // Maps pointer values to indices in PackedObject.Pointers.
	columns          []*column.Natural      // Cache of deserialized columns, indexed by BackReference.
	coins            []*coin.Info           // Cache of deserialized coins.
	queries          []*ifaces.Query        // Cache of deserialized queries.
	compiledIOPsFast []*wizard.CompiledIOP  // Cache of deserialized CompiledIOPs.
	stores           []*column.Store        // Cache of deserialized stores.
	circuits         []*cs.SparseR1CS       // Cache of deserialized circuits.
	expressions      []*symbolic.Expression // Cache of deserialized expressions
	warnings         []string               // Collects warnings for debugging.

	muPtr sync.RWMutex

	// CompiledIOPs  []*wizard.CompiledIOP  // Cache of deserialized CompiledIOPs.
}

// PackedObject is the serialized representation of data, designed for CBOR encoding.
// It stores type metadata, objects, and a payload for the root serialized value.
type PackedObject struct {
	Types           []string             `cbor:"a"` // Type names for interfaces.
	PointedValues   []any                `cbor:"c"` // Serialized pointers (as PackedIFace).
	ColumnIDs       []string             `cbor:"d"` // String IDs for columns.
	Columns         []PackedStructObject `cbor:"e"` // Serialized columns (as PackedStructObject).
	CoinIDs         []string             `cbor:"f"` // String IDs for coins.
	Coins           []PackedCoin         `cbor:"g"` // Serialized coins.
	QueryIDs        []string             `cbor:"h"` // String IDs for queries.
	Queries         []PackedStructObject `cbor:"i"` // Serialized queries.
	Store           [][]any              `cbor:"j"` // Serialized stores (as arrays).
	CompiledIOPFast []PackedCompiledIOP  `cbor:"k"` // Serialized CompiledIOPs.
	Circuits        [][]byte             `cbor:"l"` // Serialized circuits.
	Expressions     []PackedStructObject `cbor:"m"` // Serialized expressions
	Payload         any                  `cbor:"n"` // CBOR-encoded root value.

	// CompiledIOP   []PackedStructObject `cbor:"k"` // Serialized CompiledIOPs.
}

// PackedIFace serializes an interface value, storing its type index and concrete value.
type PackedIFace struct {
	Type     int `cbor:"t"` // Index into PackedObject.Types.
	Concrete any `cbor:"c"` // Serialized concrete value.
}

// PackedCoin is a compact representation of coin.Info, optimized for CBOR encoding.
type PackedCoin struct {
	Type       int8   `cbor:"t"`           // Coin type (e.g., Random, Fixed).
	Size       int    `cbor:"s,omitempty"` // Coin size (optional).
	UpperBound int32  `cbor:"u,omitempty"` // Upper bound for coin (optional).
	Name       string `cbor:"n"`           // Coin name.
	Round      int    `cbor:"r"`           // Round number.
}

// PackedStructObject is a slice of serialized field values for a struct.
type PackedStructObject []any

func NewSerializer() *Serializer {
	return &Serializer{
		PackedObject:     &PackedObject{},
		typeMap:          map[string]int{},
		pointerMap:       map[uintptr]int{},
		coinMap:          map[uuid.UUID]int{},
		coinIdMap:        map[string]int{},
		columnMap:        map[uuid.UUID]int{},
		columnIdMap:      map[string]int{},
		queryMap:         map[uuid.UUID]int{},
		queryIDMap:       map[string]int{},
		compiledIOPsFast: map[*wizard.CompiledIOP]int{},
		stores:           map[*column.Store]int{},
		circuitMap:       map[*cs.SparseR1CS]int{},
		exprMap:          map[*symbolic.Expression]int{},
	}
}

// Serialize is the entry point for serializing any value into CBOR-encoded bytes.
// It packs the value into a PackedObject.Payload, encodes the PackedObject, and returns the result.
// Warnings are printed for debugging.
func Serialize(v any) (bytesOfV []byte, err error) {
	// Initialize a new Serializer with empty maps and a PackedObject.
	ser := NewSerializer()

	// Pack the input value.
	payload, errV := ser.PackValue(reflect.ValueOf(v))
	if errV != nil {
		return nil, newSerdeErrorf("could not pack the value: %w", errV)
	}

	// Print any warnings (e.g., unexported fields).
	for i := range ser.warnings {
		fmt.Println(ser.warnings[i])
	}

	// Store the packed (already serialized) payload directly
	packedObject := ser.PackedObject
	packedObject.Payload = payload

	// Single CBOR encode of the whole PackedObject
	bytesOfV, err = encodeWithCBOR(packedObject)
	if err != nil {
		return nil, fmt.Errorf("could not encode the packedObject with CBOR: %v", err)
	}
	return bytesOfV, nil

}

func NewDeserializer(packedObject *PackedObject) *Deserializer {
	return &Deserializer{
		pointedValues:    make([]reflect.Value, len(packedObject.PointedValues)),
		columns:          make([]*column.Natural, len(packedObject.Columns)),
		coins:            make([]*coin.Info, len(packedObject.Coins)),
		queries:          make([]*ifaces.Query, len(packedObject.Queries)),
		compiledIOPsFast: make([]*wizard.CompiledIOP, len(packedObject.CompiledIOPFast)),
		stores:           make([]*column.Store, len(packedObject.Store)),
		circuits:         make([]*cs.SparseR1CS, len(packedObject.Circuits)),
		expressions:      make([]*symbolic.Expression, len(packedObject.Expressions)),
		PackedObject:     packedObject,
	}
}

// Deserialize is the entry point for deserializing CBOR-encoded bytes into a pointer.
// It decodes the bytes into a PackedObject, unpacks the Payload, and sets the result into v.
// Warnings are printed for debugging.
func Deserialize(b []byte, v any) error {
	packedObject := &PackedObject{}

	if reflect.TypeOf(v).Kind() != reflect.Ptr {
		return fmt.Errorf("invalid type: %v, expected a pointer", reflect.TypeOf(v).String())
	}
	// Decode the CBOR-encoded PackedObject once
	if err := decodeWithCBOR(b, packedObject); err != nil {
		return fmt.Errorf("the serialized object does not have the [PackedObject] format, err=%w", err)
	}

	deser := NewDeserializer(packedObject)
	payloadType := reflect.TypeOf(v).Elem()

	// Directly unpack the already-decoded payload (no CBOR decode here)
	res, err := deser.UnpackValue(packedObject.Payload, payloadType)
	if err != nil {
		return fmt.Errorf("could not deserialize the payload, err=%w", err)
	}
	for i := range deser.warnings {
		fmt.Println(deser.warnings[i])
	}

	reflect.ValueOf(v).Elem().Set(res)
	return nil
}

// PackValue recursively serializes a reflect.Value into a serializable form.
// It handles protocol-specific types (e.g., columns, coins) and generic types (e.g., structs, slices).
// Returns the serialized value or an error.
func (ser *Serializer) PackValue(v reflect.Value) (any, *serdeError) {
	// This captures the case where the value is nil to begin with
	if !v.IsValid() || v.Interface() == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, nil
	}

	typeOfV := v.Type()
	// Identify custom codexes
	if codex, ok := CustomCodexes[typeOfV]; ok {
		return codex.Ser(ser, v)
	}

	// Handle protocol-specific types.
	switch {
	case typeOfV == TypeOfColumnNatural:
		return ser.PackColumn(v.Interface().(column.Natural))
	case typeOfV == TypeOfColumnID:
		return ser.PackColumnID(v.Interface().(ifaces.ColID))
	case typeOfV == TypeOfCoin:
		return ser.PackCoin(v.Interface().(coin.Info))
	case typeOfV == TypeOfCoinID:
		return ser.PackCoinID(v.Interface().(coin.Name))
	case typeOfV.Implements(TypeOfQuery) && typeOfV.Kind() != reflect.Interface && !(typeOfV.Kind() == reflect.Ptr && typeOfV.Elem().Implements(TypeOfQuery)):
		return ser.PackQuery(v.Interface().(ifaces.Query))
	case typeOfV == TypeOfQueryID:
		return ser.PackQueryID(v.Interface().(ifaces.QueryID))
	case typeOfV == TypeOfCompiledIOPPointer:
		unpacked := v.Interface().(*wizard.CompiledIOP)
		if unpacked == nil {
			return nil, nil
		}
		return ser.PackCompiledIOPFast(unpacked)
	case typeOfV == TypeOfStorePtr:
		unpacked := v.Interface().(*column.Store)
		if unpacked == nil {
			return nil, nil
		}
		return ser.PackStore(unpacked)
	case typeOfV == TypeOfPlonkCirc:
		unpacked := v.Interface().(*cs.SparseR1CS)
		if unpacked == nil {
			return nil, nil
		}
		return ser.PackPlonkCircuit(unpacked)
	case typeOfV == TypeOfExpressionPtr:
		unpacked := v.Interface().(*symbolic.Expression)
		if unpacked == nil {
			return nil, nil
		}
		return ser.PackExpression(unpacked)
	}

	// Handle generic Go types.
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.String:
		return v.Interface(), nil
	case reflect.Array, reflect.Slice:
		return ser.PackArrayOrSlice(v)
	case reflect.Interface:
		return ser.PackInterface(v)
	case reflect.Map:
		return ser.PackMap(v)
	case reflect.Pointer:
		return ser.PackPointer(v)
	case reflect.Struct:
		return ser.PackStructObject(v)
	default:
		// Panic for unsupported types
		return nil, newSerdeErrorf("unsupported type kind: %v", v.Kind())
	}
}

// UnpackValue recursively deserializes a value into a target reflect.Type.
// It resolves back-references for protocol-specific types and handles generic types.
// Returns the deserialized reflect.Value or an error.
func (de *Deserializer) UnpackValue(v any, t reflect.Type) (r reflect.Value, e *serdeError) {

	if v == nil {
		return reflect.Zero(t), nil
	}

	// Identify custom codexes
	if codex, ok := CustomCodexes[t]; ok {
		return codex.Des(de, v, t)
	}

	// Handle protocol-specific types.
	switch {
	case t == TypeOfColumnNatural:
		return de.UnpackColumn(backReferenceFromCBORInt(v))
	case t == TypeOfColumnID:
		return de.UnpackColumnID(backReferenceFromCBORInt(v))
	case t == TypeOfCoin:
		return de.UnpackCoin(backReferenceFromCBORInt(v))
	case t == TypeOfCoinID:
		return de.UnpackCoinID(backReferenceFromCBORInt(v))
	case t.Implements(TypeOfQuery) && t.Kind() != reflect.Interface && !(t.Kind() == reflect.Ptr && t.Elem().Implements(TypeOfQuery)):
		return de.UnpackQuery(backReferenceFromCBORInt(v), t)
	case t == TypeOfQueryID:
		return de.UnpackQueryID(backReferenceFromCBORInt(v))
	case t == TypeOfCompiledIOPPointer:
		return de.UnpackCompiledIOPFast(backReferenceFromCBORInt(v))
	case t == TypeOfStorePtr:
		return de.UnpackStore(backReferenceFromCBORInt(v))
	case t == TypeOfPlonkCirc:
		return de.UnpackPlonkCircuit(backReferenceFromCBORInt(v))
	case t == TypeOfExpressionPtr:
		return de.UnpackExpression(backReferenceFromCBORInt(v))
	}

	// Handle generic Go types.
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.String:
		return de.UnpackPrimitive(v, t), nil
	case reflect.Array, reflect.Slice:
		return de.UnpackArrayOrSlice(v.([]any), t)
	case reflect.Map:
		v := v.(map[any]any)
		return de.UnpackMap(v, t)
	case reflect.Interface:
		v_, ok := v.(map[interface{}]any)
		if !ok {
			return reflect.Value{}, newSerdeErrorf("expected %v to be of type map[interface{}]any, was=%T", v, v)
		}
		return de.UnpackInterface(v_, t)
	case reflect.Pointer:
		return de.UnpackPointer(v, t)
	case reflect.Struct:
		if v_, ok := v.(PackedStructObject); ok {
			v = []any(v_)
		}
		return de.UnpackStructObject(v.([]any), t)
	default:
		return reflect.Value{}, newSerdeErrorf("unsupported type kind: %v", t.Kind())
	}
}

// PackColumn serializes a column.Natural, returning a BackReference to its index in PackedObject.Columns.
// It ensures columns are serialized once, using UUIDs for deduplication.
func (ser *Serializer) PackColumn(c column.Natural) (BackReference, *serdeError) {

	// The error must be reported before we access the UUID otherwise, it will panic.
	if c.ID == "" {
		return 0, newSerdeErrorf("empty column ID")
	}

	cid := c.UUID()
	if _, ok := ser.columnMap[cid]; !ok {
		packed := c.Pack()
		marshaled, err := ser.PackStructObject(reflect.ValueOf(packed))
		if err != nil {
			return 0, err.wrapPath("(column)")
		}
		ser.PackedObject.Columns = append(ser.PackedObject.Columns, marshaled)
		ser.columnMap[cid] = len(ser.PackedObject.Columns) - 1
	}

	return BackReference(
		ser.columnMap[cid],
	), nil
}

// UnpackColumn deserializes a column.Natural from a BackReference.
// It caches the result to maintain object identity.
func (de *Deserializer) UnpackColumn(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.Columns) {
		return reflect.Value{}, newSerdeErrorf("invalid column backreference: %v", v)
	}

	// It's the first time that 'd' sees the column: it unpacks it from the
	// pre-unmarshalled object
	if de.columns[v] == nil {
		packedStruct := de.PackedObject.Columns[v]
		packedNatVal, err := de.UnpackStructObject(packedStruct, TypeOfPackedColumn)
		if err != nil {
			return reflect.Value{}, err.wrapPath("(column)")
		}

		packedNat := packedNatVal.Interface().(column.PackedNatural)
		nat := packedNat.Unpack()
		de.columns[v] = &nat
	}

	return reflect.ValueOf(*de.columns[v]), nil
}

// PackColumnID serializes an ifaces.ColID (string), returning a BackReference to its index in PackedObject.ColumnIDs.
func (ser *Serializer) PackColumnID(c ifaces.ColID) (BackReference, *serdeError) {
	if _, ok := ser.columnIdMap[string(c)]; !ok {
		ser.PackedObject.ColumnIDs = append(ser.PackedObject.ColumnIDs, string(c))
		ser.columnIdMap[string(c)] = len(ser.PackedObject.ColumnIDs) - 1
	}

	return BackReference(ser.columnIdMap[string(c)]), nil
}

// UnpackColumnID deserializes an ifaces.ColID from a BackReference.
func (de *Deserializer) UnpackColumnID(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.ColumnIDs) {
		return reflect.Value{}, newSerdeErrorf("invalid column-ID backreference: %v", v)
	}

	res := ifaces.ColID(de.PackedObject.ColumnIDs[v])
	return reflect.ValueOf(res), nil
}

// PackCoin serializes a coin.Info, returning a BackReference to its index in PackedObject.Coins.
func (ser *Serializer) PackCoin(c coin.Info) (BackReference, *serdeError) {
	if _, ok := ser.coinMap[c.UUID()]; !ok {
		ser.PackedObject.Coins = append(ser.PackedObject.Coins, ser.AsPackedCoin(c))
		ser.coinMap[c.UUID()] = len(ser.PackedObject.Coins) - 1
	}

	return BackReference(ser.coinMap[c.UUID()]), nil
}

// UnpackCoin deserializes a coin.Info from a BackReference, caching the result.
func (de *Deserializer) UnpackCoin(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.Coins) {
		return reflect.Value{}, newSerdeErrorf("invalid coin back-reference=%v", v)
	}

	if de.coins[v] == nil {
		packedCoin := de.PackedObject.Coins[v]

		sizes := []int{}
		if packedCoin.Size > 0 {
			sizes = append(sizes, packedCoin.Size)
		}
		if packedCoin.UpperBound > 0 {
			sizes = append(sizes, int(packedCoin.UpperBound))
		}

		unpacked := coin.NewInfo(
			coin.Name(packedCoin.Name),
			coin.Type(packedCoin.Type),
			packedCoin.Round,
			sizes...,
		)

		de.coins[v] = &unpacked
	}

	return reflect.ValueOf(*de.coins[v]), nil
}

// AsPackedCoin converts a coin.Info to a PackedCoin for serialization.
func (ser *Serializer) AsPackedCoin(c coin.Info) PackedCoin {
	return PackedCoin{
		Type:       int8(c.Type),
		Size:       c.Size,
		UpperBound: int32(c.UpperBound),
		Name:       string(c.Name),
		Round:      c.Round,
	}
}

// PackCoinID serializes a coin.Name (string), returning a BackReference to its index in PackedObject.CoinIDs.
func (ser *Serializer) PackCoinID(c coin.Name) (BackReference, *serdeError) {
	if _, ok := ser.coinIdMap[string(c)]; !ok {
		ser.PackedObject.CoinIDs = append(ser.PackedObject.CoinIDs, string(c))
		ser.coinIdMap[string(c)] = len(ser.PackedObject.CoinIDs) - 1
	}

	return BackReference(ser.coinIdMap[string(c)]), nil
}

// UnpackCoinID deserializes a coin.Name from a BackReference.
func (de *Deserializer) UnpackCoinID(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.CoinIDs) {
		return reflect.Value{}, newSerdeErrorf("invalid coin ID back reference: %v", v)
	}

	res := coin.Name(de.PackedObject.CoinIDs[v])
	return reflect.ValueOf(res), nil
}

// PackQuery serializes an ifaces.Query, returning a BackReference to its index in PackedObject.Queries.
func (ser *Serializer) PackQuery(q ifaces.Query) (BackReference, *serdeError) {
	if _, ok := ser.queryMap[q.UUID()]; !ok {
		valueOfQ := reflect.ValueOf(q)
		if valueOfQ.Type().Kind() == reflect.Ptr {
			valueOfQ = valueOfQ.Elem()
		}

		obj, err := ser.PackStructObject(valueOfQ)
		if err != nil {
			return 0, err.wrapPath("(query)")
		}
		ser.PackedObject.Queries = append(ser.PackedObject.Queries, obj)
		ser.queryMap[q.UUID()] = len(ser.PackedObject.Queries) - 1
	}

	return BackReference(ser.queryMap[q.UUID()]), nil
}

// UnpackQuery deserializes an ifaces.Query from a BackReference, ensuring it matches the target type.
func (de *Deserializer) UnpackQuery(v BackReference, t reflect.Type) (reflect.Value, *serdeError) {
	typeConcrete := t
	if t.Kind() == reflect.Ptr {
		typeConcrete = t.Elem()
	}

	if !t.Implements(TypeOfQuery) || typeConcrete.Kind() != reflect.Struct {
		return reflect.Value{}, newSerdeErrorf("invalid query type: %v", t.String())
	}

	if v < 0 || int(v) >= len(de.PackedObject.Queries) {
		return reflect.Value{}, newSerdeErrorf("invalid query backreference: %v", v)
	}

	if de.queries[v] == nil {
		packedQuery := de.PackedObject.Queries[v]
		query, err := de.UnpackStructObject(packedQuery, typeConcrete)
		if err != nil {
			return reflect.Value{}, err.wrapPath("(query)")
		}

		if t.Kind() == reflect.Ptr {
			query = query.Addr()
		}

		q := query.Interface().(ifaces.Query)
		de.queries[v] = &q
	}

	var (
		qIfaces = *de.queries[v]
		qValue  = reflect.ValueOf(qIfaces)
	)

	if qValue.Type() != t {
		return reflect.Value{}, newSerdeErrorf("the deserialized query does not have the expected type, %v != %v", t.String(), qValue.Type().String())
	}

	return qValue, nil
}

// PackQueryID serializes an ifaces.QueryID (string), returning a BackReference to its index in PackedObject.QueryIDs.
func (ser *Serializer) PackQueryID(q ifaces.QueryID) (BackReference, *serdeError) {
	if _, ok := ser.queryIDMap[string(q)]; !ok {
		ser.PackedObject.QueryIDs = append(ser.PackedObject.QueryIDs, string(q))
		ser.queryIDMap[string(q)] = len(ser.PackedObject.QueryIDs) - 1
	}

	return BackReference(ser.queryIDMap[string(q)]), nil
}

// UnpackQueryID deserializes an ifaces.QueryID from a BackReference.
func (de *Deserializer) UnpackQueryID(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.QueryIDs) {
		return reflect.Value{}, newSerdeErrorf("invalid query-ID backreference: %v", v)
	}

	res := ifaces.QueryID(de.PackedObject.QueryIDs[v])
	return reflect.ValueOf(res), nil
}

// PackStore serializes a column.Store, returning a BackReference to its index in PackedObject.Store.
func (ser *Serializer) PackStore(s *column.Store) (BackReference, *serdeError) {
	if _, ok := ser.stores[s]; !ok {
		packedStore := s.Pack()
		obj, err := ser.PackArrayOrSlice(reflect.ValueOf(packedStore))
		if err != nil {
			return 0, err.wrapPath("(store)")
		}
		ser.PackedObject.Store = append(ser.PackedObject.Store, obj)
		ser.stores[s] = len(ser.PackedObject.Store) - 1
	}

	return BackReference(ser.stores[s]), nil
}

// UnpackStore deserializes a column.Store from a BackReference, caching the result.
func (de *Deserializer) UnpackStore(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.Store) {
		return reflect.Value{}, newSerdeErrorf("invalid store backreference: %v", v)
	}

	if de.stores[v] == nil {
		preStore := de.PackedObject.Store[v]
		storeArr, err := de.UnpackArrayOrSlice(preStore, TypeOfPackedStore)
		if err != nil {
			return reflect.Value{}, err.wrapPath("(store)")
		}

		pStore := storeArr.Interface().(column.PackedStore)
		de.stores[v] = pStore.Unpack()
	}

	return reflect.ValueOf(de.stores[v]), nil
}

// PackPlonkCircuit serializes a plonk circuit using gnark's optimized serialization
// algoritm. The serialized object is stored in the table [PackedObject.Circuit]
// table and the
func (ser *Serializer) PackPlonkCircuit(circuit *cs.SparseR1CS) (BackReference, *serdeError) {

	if _, ok := ser.circuitMap[circuit]; !ok {
		buf := &bytes.Buffer{}
		circuit.WriteTo(buf)
		ser.PackedObject.Circuits = append(ser.PackedObject.Circuits, buf.Bytes())
		ser.circuitMap[circuit] = len(ser.PackedObject.Circuits) - 1
	}

	return BackReference(ser.circuitMap[circuit]), nil
}

// UnpackPlonkCircuit deserializes a circuit from a BackReference, caching the result.
func (de *Deserializer) UnpackPlonkCircuit(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.Circuits) {
		return reflect.Value{}, newSerdeErrorf("invalid circuit backreference: %v", v)
	}

	if de.circuits[v] == nil {
		circ := &cs.SparseR1CS{}
		packedCircuit := de.PackedObject.Circuits[v]
		r := bytes.NewReader(packedCircuit)
		if _, err := circ.ReadFrom(r); err != nil {
			return reflect.Value{}, newSerdeErrorf("could not scan the unpacked circuit, err=%w", err)
		}

		de.circuits[v] = circ
	}

	return reflect.ValueOf(de.circuits[v]), nil
}

// PackExpression packs a symbolic expression by caching the packed expression
// in the table [PackedObject.Expressions] table and returning a BackReference
// to it. The expression is cached using its ESHash.
func (ser *Serializer) PackExpression(e *symbolic.Expression) (BackReference, *serdeError) {
	if _, ok := ser.exprMap[e]; !ok {

		n := len(ser.PackedObject.Expressions)
		ser.exprMap[e] = n
		ser.PackedObject.Expressions = append(ser.PackedObject.Expressions, nil)

		packed, err := ser.PackStructObject(reflect.ValueOf(*e))
		if err != nil {
			return 0, err
		}

		ser.PackedObject.Expressions[n] = packed
	}

	return BackReference(ser.exprMap[e]), nil
}

// UnpackExpression unpacks an expression from a BackReference, using the cached
// result if possible or unpacking the underlying expression and then caching it.
func (de *Deserializer) UnpackExpression(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.Expressions) {
		return reflect.Value{}, newSerdeErrorf("invalid expression backreference: %v", v)
	}

	if de.expressions[v] == nil {
		preExpr := de.PackedObject.Expressions[v]
		expr, err := de.UnpackStructObject(preExpr, TypeOfExpression)
		if err != nil {
			return reflect.Value{}, err
		}

		unpacked := expr.Interface().(symbolic.Expression)
		de.expressions[v] = &unpacked
	}

	return reflect.ValueOf(de.expressions[v]), nil
}

// PackArrayOrSlice serializes arrays or slices by recursively serializing each element.
// It collects errors for all elements and returns a combined error if any fail.
func (ser *Serializer) PackArrayOrSlice(v reflect.Value) ([]any, *serdeError) {
	res := make([]any, v.Len())
	globalErr := &serdeError{}

	for i := 0; i < v.Len(); i++ {
		ri, err := ser.PackValue(v.Index(i))
		if err != nil {
			globalErr.appendError(err.wrapPath("[" + strconv.Itoa(i) + "]"))
		}
		res[i] = ri
	}

	if !globalErr.isEmpty() {
		return nil, globalErr
	}

	return res, nil
}

// UnpackArrayOrSlice deserializes arrays or slices, reconstructing elements into the target type.
// It collects errors for all elements and returns a combined error if any fail.
func (de *Deserializer) UnpackArrayOrSlice(v []any, t reflect.Type) (reflect.Value, *serdeError) {
	var res reflect.Value

	switch t.Kind() {
	case reflect.Array:
		res = reflect.New(t).Elem()
		if t.Len() != len(v) {
			return reflect.Value{}, newSerdeErrorf("failed to deserialize to %q, size mismatch: %d != %d", t.String(), len(v), t.Len())
		}
	case reflect.Slice:
		res = reflect.MakeSlice(t, len(v), len(v))
	default:
		return reflect.Value{}, newSerdeErrorf("failed to deserialize to %q, expected array or slice", t.String())
	}

	globalErr := &serdeError{}

	subType := t.Elem()
	for i := 0; i < len(v); i++ {
		subV, err := de.UnpackValue(v[i], subType)
		if err != nil {
			globalErr.appendError(err.wrapPath("[" + strconv.Itoa(i) + "]"))
			continue
		}
		res.Index(i).Set(subV)
	}

	if !globalErr.isEmpty() {
		return reflect.Value{}, globalErr
	}

	return res, nil
}

// PackInterface serializes an interface value, storing its type index and concrete value in a PackedIFace.
// It ensures the concrete type is registered and returns an error if not.
func (ser *Serializer) PackInterface(v reflect.Value) (any, *serdeError) {
	var (
		concrete          = v.Elem()
		cleanConcreteType = getPkgPathAndTypeNameIndirect(concrete.Interface())
	)

	if _, err := findRegisteredImplementation(cleanConcreteType); err != nil {
		return nil, newSerdeErrorf("attempted to serialize unregistered type repr=%q type=%v: %v", cleanConcreteType, concrete.Type().String(), err)
	}

	if _, ok := ser.typeMap[cleanConcreteType]; !ok {
		ser.PackedObject.Types = append(ser.PackedObject.Types, cleanConcreteType)
		ser.typeMap[cleanConcreteType] = len(ser.PackedObject.Types) - 1
	}

	packedConcrete, err := ser.PackValue(concrete)
	if err != nil {
		return nil, err.wrapPath("(" + concrete.Type().String() + ")")
	}

	return PackedIFace{
		Type:     ser.typeMap[cleanConcreteType],
		Concrete: packedConcrete,
	}, nil
}

// UnpackInterface deserializes an interface value from a map, resolving the concrete type and value.
func (de *Deserializer) UnpackInterface(pi map[interface{}]interface{}, t reflect.Type) (reflect.Value, *serdeError) {
	var (
		ctype, ok = pi["t"].(uint64)
		concrete  = pi["c"]
	)

	if !ok || int(ctype) >= len(de.PackedObject.Types) {
		return reflect.Value{}, newSerdeErrorf("invalid packed interface, it does not have a valid type integer: %v", ctype)
	}

	cleanConcreteType := de.PackedObject.Types[ctype]
	refType, err := findRegisteredImplementation(cleanConcreteType)
	if err != nil {
		return reflect.Value{}, newSerdeErrorf("unregistered type %q for interface: %w", cleanConcreteType, err)
	}

	if !refType.Implements(t) {
		return reflect.Value{}, newSerdeErrorf("the resolved type does not implement the target interface, %v ~ %v", refType.String(), t.String())
	}

	cres, errV := de.UnpackValue(concrete, refType)
	if errV != nil {
		return reflect.Value{}, errV.wrapPath("(" + refType.String() + ")")
	}

	// Create a new reflect.Value for the interface type
	// Reminder; here the important thing is to ensure that the returned
	// Value actually bears the requested interface type and not the
	// concrete type.
	ifaceValue := reflect.New(t).Elem()
	ifaceValue.Set(cres)

	return ifaceValue, nil
}

// PackStructObject serializes a struct as a PackedStructObject (slice of field values).
// It skips unexported fields, logs warnings, and registers the schema.
func (ser *Serializer) PackStructObject(obj reflect.Value) (PackedStructObject, *serdeError) {
	if obj.Kind() != reflect.Struct {
		return nil, newSerdeErrorf("obj.Kind() != reflect.Struct, type=%v", obj.Type().String())
	}

	values := make([]any, obj.NumField())
	globalErr := &serdeError{}

	// Note, since we don't want to register the schema before going through
	// all the components, we have to rely on the fact that schema and this loop
	// declare the fields in the same order.
	for i := 0; i < obj.NumField(); i++ {
		field := obj.Field(i)

		// Definitely not something we can accept
		if obj.Type().Field(i).Type == reflect.TypeOf(reflect.Value{}) {
			return nil, newSerdeErrorf("field type is reflect.Value, %v.%v", obj.Type().String(), obj.Type().Field(i).Name)
		}

		// When the field is has the omitted tag, we skip it there without any
		// warning.
		if tag, hasTag := obj.Type().Field(i).Tag.Lookup(SerdeStructTag); hasTag {
			if strings.Contains(tag, SerdeStructTagOmit) {
				// implicitly, we leave values[i] as nil
				continue
			}
		}

		// By caution, we emit a warning when finding an unexported field. To
		// help the caller understand that we might omit something that he would
		// not want to.
		if !obj.Type().Field(i).IsExported() {
			ser.warnf(fmt.Sprintf("field %v.%v is not exported", obj.Type().String(), obj.Type().Field(i).Name))
			// implicitly, we leave values[i] as nil
			continue
		}

		vi, err := ser.PackValue(field)
		if err != nil {
			prefix := "." + obj.Type().Field(i).Name
			globalErr.appendError(err.wrapPath(prefix))
		}
		values[i] = vi
	}

	if !globalErr.isEmpty() {
		return PackedStructObject{}, globalErr
	}

	// Importantly, we want to be sure that all the component have been
	// converted before we convert the current type. That way, we can ensure
	// that all the information of the dependency is added prior to the
	// information on the dependant struct. This is necessary for seria-
	// lization.
	return values, nil
}

// UnpackStructObject deserializes a PackedStructObject into a struct, using the schema to map fields.
// It skips unexported fields, logs warnings, and collects errors.
func (de *Deserializer) UnpackStructObject(v PackedStructObject, t reflect.Type) (reflect.Value, *serdeError) {
	if t.Kind() != reflect.Struct {
		return reflect.Value{}, newSerdeErrorf("invalid type: %v", t.String())
	}

	var (
		res = reflect.New(t).Elem()
	)

	if len(v) != t.NumField() {
		return reflect.Value{}, newSerdeErrorf("invalid number of fields: %v, expected %v, type=%v", len(v), t.NumField(), t.String())
	}

	// To ease debugging, all the errors for all the fields are joined and
	// wrapped in a single error.
	globalErr := &serdeError{}

	for i := 0; i < t.NumField(); i++ {

		structField := t.Field(i)

		// When the field is has the omitted tag, we skip it there without any
		// warning.
		if tag, hasTag := structField.Tag.Lookup(SerdeStructTag); hasTag {
			if strings.Contains(tag, SerdeStructTagOmit) {
				continue
			}
		}

		if !structField.IsExported() {
			de.warnf(fmt.Sprintf("field %v.%v is not exported", t.String(), structField.Name))
			continue
		}

		field := res.Field(i)
		value, err := de.UnpackValue(v[i], field.Type())
		if err != nil {
			prefix := "." + structField.Name
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		if field.Type() != value.Type() {
			err := newSerdeErrorf("field type mismatch: %v != %v", field.Type().String(), value.Type().String())
			prefix := "." + structField.Name
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		field.Set(value)
	}

	if !globalErr.isEmpty() {
		return reflect.Value{}, globalErr
	}

	return res, nil
}

// PackMap serializes a map with string keys, returning a map[any]any.
// It sorts keys for deterministic encoding and collects errors. The map is
// packed as an array of tuples.
func (ser *Serializer) PackMap(obj reflect.Value) (map[any]any, *serdeError) {
	if obj.Kind() != reflect.Map {
		return nil, newSerdeErrorf("obj.Kind() != reflect.Map, type=%v", obj.Type().String())
	}

	var (
		keys         = obj.MapKeys()
		packedKeys   = make([]any, len(keys))
		packedValues = make([]any, len(keys))
		globalErr    = &serdeError{}
	)

	for i, key := range keys {

		packedKey, err := ser.PackValue(key)
		if err != nil {
			prefix := fmt.Sprintf(".keys[%d]", i)
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		val := obj.MapIndex(key)
		packedValue, err := ser.PackValue(val)
		if err != nil {
			prefix := fmt.Sprintf("[%v]", key.Interface())
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		packedKeys[i] = packedKey
		packedValues[i] = packedValue
	}

	if !globalErr.isEmpty() {
		return nil, globalErr
	}

	return map[any]any{
		"k": packedKeys,
		"v": packedValues,
	}, nil
}

// UnpackMap deserializes a map[any]any into a map of the target type.
// It collects errors for keys and values.
func (de *Deserializer) UnpackMap(packedMap map[any]any, t reflect.Type) (reflect.Value, *serdeError) {
	if t.Kind() != reflect.Map {
		return reflect.Value{}, newSerdeErrorf("invalid map type: %v", t.String())
	}

	var (
		typeOfKey    = t.Key()
		typeOfValue  = t.Elem()
		packedKeys   = packedMap["k"].([]any)
		packedValues = packedMap["v"].([]any)
		res          = reflect.MakeMap(t)
		globalErr    = &serdeError{}
	)

	for i := range packedKeys {

		key := packedKeys[i]
		val := packedValues[i]

		k, err := de.UnpackValue(key, typeOfKey)
		if err != nil {
			prefix := fmt.Sprintf(".keys[%d]", i)
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		v, err := de.UnpackValue(val, typeOfValue)
		if err != nil {
			prefix := fmt.Sprintf("[%v]", k.Interface())
			globalErr.appendError(err.wrapPath(prefix))
			continue
		}

		res.SetMapIndex(k, v)
	}

	if !globalErr.isEmpty() {
		return reflect.Value{}, globalErr
	}

	return res, nil
}

// PackPointer serializes a pointer value.
func (ser *Serializer) PackPointer(v reflect.Value) (any, *serdeError) {

	if v.Kind() != reflect.Ptr {
		return reflect.Value{}, newSerdeErrorf("invalid type: %v, expected a pointer", v.Type().String())
	}

	if v.IsNil() {
		return nil, nil
	}

	if _, ok := ser.pointerMap[v.Pointer()]; !ok {

		// nil is appended just so that the recursive calls do not also use the
		// same backref for different objects. Also, we preemptively assign the
		// backref in the map to prevent infinite-loop with recursive data
		// structure (e.g. circular pointer references).
		backRef := len(ser.PackedObject.PointedValues)
		ser.pointerMap[v.Pointer()] = backRef
		ser.PackedObject.PointedValues = append(ser.PackedObject.PointedValues, nil)

		packedElem, err := ser.PackValue(v.Elem())
		if err != nil {
			return nil, err.wrapPath("(pointer)")
		}

		ser.PackedObject.PointedValues[backRef] = packedElem
	}

	return BackReference(ser.pointerMap[v.Pointer()]), nil
}

func (de *Deserializer) UnpackPointer(v any, t reflect.Type) (reflect.Value, *serdeError) {

	if t.Kind() != reflect.Ptr {
		return reflect.Value{}, newSerdeErrorf("invalid type: %v, expected a pointer", t.String())
	}

	if v == nil {
		// This returns a nil-pointer of the target type.
		return reflect.Zero(t), nil
	}

	backRefInt, ok := v.(uint64)
	if !ok {
		return reflect.Value{}, newSerdeErrorf("pointer type=%v is not a BackReference nor a nil value, got=%++v", t.String(), v)
	}
	backRef := BackReference(backRefInt)

	if backRef < 0 || int(backRef) >= len(de.PackedObject.PointedValues) {
		return reflect.Value{}, newSerdeErrorf("invalid pointer backreference: %v", v)
	}

	de.muPtr.RLock()
	val := de.pointedValues[backRef]
	de.muPtr.RUnlock()

	if (val == reflect.Value{}) {

		// To guards against infinite recursion, we preemptively assign a
		// pointer value that we will use for subsequent occurence of the same
		// backreference. This can happen when ser/de a structure with recursive
		// pointers.
		ptrValue := reflect.New(t.Elem())

		de.muPtr.Lock()
		de.pointedValues[backRef] = ptrValue
		de.muPtr.Unlock()

		packedElem := de.PackedObject.PointedValues[backRef]
		elem, err := de.UnpackValue(packedElem, t.Elem())
		if err != nil {
			return reflect.Value{}, err.wrapPath("(pointer)")
		}

		de.muPtr.Lock()
		ptrValue.Elem().Set(elem)
		de.muPtr.Unlock()
	}

	return de.pointedValues[backRef], nil
}

// UnpackPrimitive converts a primitive value to the target type using reflection.
func (de *Deserializer) UnpackPrimitive(v any, t reflect.Type) reflect.Value {
	return reflect.ValueOf(v).Convert(t)
}

// warnf logs a warning message to the Serializer’s Warnings slice.
func (ser *Serializer) warnf(warning string) {
	ser.warnings = append(ser.warnings, warning)
}

// warnf logs a warning message to the Deserializer’s Warnings slice.
func (de *Deserializer) warnf(warning string) {
	de.warnings = append(de.warnings, warning)
}

// backReferenceFromCBORInt converts a CBOR-decoded uint64 to a BackReference.
// It assumes the input is a valid index.
func backReferenceFromCBORInt(n any) BackReference {
	return BackReference(n.(uint64))
}
