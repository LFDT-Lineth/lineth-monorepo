package serialization

import (
	"bytes"
	"hash"
	"math/big"
	"reflect"
	"strings"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark-crypto/utils/unsafe"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/hashtypes"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/fxamacker/cbor/v2"
)

// Use a vendor-specific tag number for homogeneous field elements packed as bytes.
// RFC 8746 reserves ranges for typed arrays but doesn't define this 377-field explicitly.
// Pick a private-use tag in the high range to avoid collisions.
const cborTagFieldElementsPacked uint64 = 60001

// CustomCodex represents an optional behavior for a specific type
type CustomCodex struct {
	Type reflect.Type
	Ser  func(ser *Serializer, val reflect.Value) (any, *serdeError)
	Des  func(des *Deserializer, val any, t reflect.Type) (reflect.Value, *serdeError)
}

var CustomCodexes = map[reflect.Type]CustomCodex{}

func init() {

	CustomCodexes[TypeOfBigInt] = CustomCodex{
		Type: TypeOfBigInt,
		Ser:  marshalBigInt,
		Des:  unmarshalBigInt,
	}

	CustomCodexes[TypeOfFieldElement] = CustomCodex{
		Type: TypeOfFieldElement,
		Ser:  marshalFieldElement,
		Des:  unmarshalFieldElement,
	}

	CustomCodexes[TypeOfArrOfFieldElement] = CustomCodex{
		Type: TypeOfArrOfFieldElement,
		Ser:  marshalArrayOfFieldElement,
		Des:  unmarshalArrayOfFieldElement,
	}

	CustomCodexes[TypeOfFrontendVariable] = CustomCodex{
		Type: TypeOfFrontendVariable,
		Ser:  marshalFrontendVariable,
		Des:  unmarshalFrontendVariable,
	}

	CustomCodexes[TypeOfHashFuncGenerator] = CustomCodex{
		Type: TypeOfHashFuncGenerator,
		Ser:  marshalAsEmptyStruct,
		Des:  unmarshalHashGenerator,
	}

	CustomCodexes[TypeOfHashTypeHasher] = CustomCodex{
		Type: TypeOfHashTypeHasher,
		Ser:  marshalHashTypeHasher,
		Des:  unmarshalHashTypeHasher,
	}

	CustomCodexes[reflect.TypeOf(sync.Mutex{})] = CustomCodex{
		Type: reflect.TypeOf(sync.Mutex{}),
		Ser:  marshalAsNil,
		Des:  unmarshalAsZero,
	}

	CustomCodexes[TypeOfRingSisKeyPtr] = CustomCodex{
		Type: TypeOfRingSisKeyPtr,
		Ser:  marshalRingSisKey,
		Des:  unmarshalRingSisKey,
	}

	CustomCodexes[TypeOfGnarkFFTDomainPtr] = CustomCodex{
		Type: TypeOfGnarkFFTDomainPtr,
		Ser:  marshalGnarkFFTDomain,
		Des:  unmarshalGnarkFFtDomain,
	}

	CustomCodexes[reflect.TypeOf(smartvectors.Regular{})] = CustomCodex{
		Type: reflect.TypeOf(smartvectors.Regular{}),
		Ser:  marshalArrayOfFieldElement,
		Des:  unmarshalArrayOfFieldElement,
	}

	CustomCodexes[TypeOfMutexPtr] = CustomCodex{
		Type: TypeOfMutexPtr,
		Ser:  marshalAsEmptyStruct,
		Des:  makeNewObject,
	}

}

func marshalRingSisKey(ser *Serializer, val reflect.Value) (any, *serdeError) {
	key, ok := val.Interface().(*ringsis.Key)
	if !ok {
		return nil, newSerdeErrorf("illegal cast of val of type %T to %v", val, TypeOfRingSisKeyPtr)
	}
	return key.KeyGen.MaxNumFieldToHash, nil
}

func unmarshalRingSisKey(des *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {
	maxNumFieldToHash, ok := val.(uint64)
	if !ok {
		return reflect.Value{}, newSerdeErrorf("illegal cast of val of type %T to int", val)
	}
	ringSiskey := ringsis.GenerateKey(ringsis.StdParams, int(maxNumFieldToHash))
	return reflect.ValueOf(ringSiskey), nil
}

func marshalGnarkFFTDomain(ser *Serializer, val reflect.Value) (any, *serdeError) {
	domain := val.Interface().(*fft.Domain)

	if domain == nil {
		return nil, nil
	}

	var buf bytes.Buffer
	if _, err := domain.WriteTo(&buf); err != nil {
		return nil, newSerdeErrorf("could not marshal fft.Domain: %w", err)
	}
	return buf.Bytes(), nil
}

func unmarshalGnarkFFtDomain(des *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {

	// nil case
	if val == nil {
		return reflect.Zero(TypeOfGnarkFFTDomainPtr), nil
	}

	// Expect a []byte coming from CBOR decoding
	var b []byte
	switch v := val.(type) {
	case []byte:
		b = v
	default:
		// defensive: CBOR typically decodes bytes to []byte, but return a helpful error if not.
		return reflect.Value{}, newSerdeErrorf("expected []byte for fft.Domain deserialization, got %T", val)
	}

	d := &fft.Domain{}
	if _, err := d.ReadFrom(bytes.NewReader(b)); err != nil {
		return reflect.Value{}, newSerdeErrorf("could not unmarshal fft.Domain: %w", err)
	}

	return reflect.ValueOf(d), nil
}

func marshalFieldElement(_ *Serializer, val reflect.Value) (any, *serdeError) {
	f := val.Interface().(field.Element)
	bi := fieldToSmallBigInt(f)
	f.BigInt(bi)
	return marshalBigInt(nil, reflect.ValueOf(bi))
}

func unmarshalFieldElement(_ *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {

	f, err := unmarshalBigInt(nil, val, TypeOfBigInt)
	if err != nil {
		return reflect.Value{}, err
	}

	var fe field.Element
	fe.SetBigInt(f.Interface().(*big.Int))
	return reflect.ValueOf(fe), nil
}

func marshalBigInt(_ *Serializer, val reflect.Value) (any, *serdeError) {
	return val.Interface().(*big.Int), nil
}

func unmarshalBigInt(_ *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {
	switch v := val.(type) {
	case big.Int:
		return reflect.ValueOf(&v), nil
	case int64:
		return reflect.ValueOf(big.NewInt(v)), nil
	case uint64:
		return reflect.ValueOf(new(big.Int).SetUint64(v)), nil
	default:
		return reflect.Value{}, newSerdeErrorf("invalid type: %T, value: %++v", val, val)
	}
}

// marshalArrayOfFieldElement: add CBOR tag wrapper.
func marshalArrayOfFieldElement(_ *Serializer, val reflect.Value) (any, *serdeError) {
	var buf = &bytes.Buffer{}

	v, ok := val.Interface().([]field.Element)
	if !ok {
		v = []field.Element(val.Interface().(smartvectors.Regular))
	}
	if err := unsafe.WriteSlice(buf, v); err != nil {
		return nil, newSerdeErrorf("could not marshal array of field element: %w", err)
	}

	// Wrap in cbor.Tag so decoders know this is a homogeneous packed vector.
	// Packing field elements as a single tagged byte string avoids element-by-element CBOR encoding/decoding,
	// cutting per-element reflection, encoder work, intermediate allocations, and per-item headers;
	// The optimization replaces a CBOR array of N field.Element items with a single tagged byte string whose content is the N elements
	// serialized contiguously in native limb form, then wrapped once with a private tag (e.g., 60001) indicating “packed field elements.”
	// This changes the wire shape from O(N) separate CBOR items to one item containing O(N) bytes. Saves about 55GiB of runtime memory.
	return cbor.Tag{
		Number:  cborTagFieldElementsPacked,
		Content: buf.Bytes(),
	}, nil
}

// unmarshalArrayOfFieldElement: accept either tagged content or raw []byte for backward compatibility.
func unmarshalArrayOfFieldElement(_ *Deserializer, val any, t reflect.Type) (reflect.Value, *serdeError) {
	var raw []byte

	switch x := val.(type) {
	// The tagged byte string path simply extracts the []byte content and reconstructs []field.Element using unsafe.ReadSlice
	// in one pass, instead of driving the decoder through N element decodes and reflection-based assignments.
	// This is a single decode step on the CBOR side plus a single contiguous read on the application side.
	// It avoids per-element CBOR encode/decode and reflection, replacing N items with a single tag+byte-string and a
	// single-pass binary read.
	case cbor.Tag:
		// Accept our tag and extract the bytes content.
		if x.Number != cborTagFieldElementsPacked {
			return reflect.Value{}, newSerdeErrorf("unexpected CBOR tag for field elements: %d", x.Number)
		}
		b, ok := x.Content.([]byte)
		if !ok {
			return reflect.Value{}, newSerdeErrorf("tagged field elements not []byte content, got %T", x.Content)
		}
		raw = b
	default:
		return reflect.Value{}, newSerdeErrorf("invalid type for field elements: %T", val)
	}

	r := bytes.NewReader(raw)
	v, _, err := unsafe.ReadSlice[[]field.Element](r)
	if err != nil {
		return reflect.Value{}, newSerdeErrorf("could not unmarshal array of field element: %w", err)
	}

	if t == reflect.TypeOf(smartvectors.Regular{}) {
		return reflect.ValueOf(smartvectors.Regular(v)), nil
	}
	return reflect.ValueOf(v), nil
}

func marshalFrontendVariable(ser *Serializer, val reflect.Value) (any, *serdeError) {

	var (
		variable = val.Interface().(frontend.Variable)
		bi       = &big.Int{}
	)

	switch v := variable.(type) {
	case int:
		bi.SetInt64(int64(v))
	case uint:
		bi.SetUint64(uint64(v))
	case int64:
		bi.SetInt64(int64(v))
	case uint64:
		bi.SetUint64(v)
	case int32:
		bi.SetInt64(int64(v))
	case uint32:
		bi.SetUint64(uint64(v))
	case int16:
		bi.SetInt64(int64(v))
	case uint16:
		bi.SetUint64(uint64(v))
	case int8:
		bi.SetInt64(int64(v))
	case uint8:
		bi.SetUint64(uint64(v))
	case field.Element:
		bi = fieldToSmallBigInt(v)
	case big.Int:
		*bi = v
	case *big.Int:
		bi = v
	case string:
		bi.SetString(v, 0)
	default:
		if variable == nil {
			return nil, nil
		}

		// The check cannot be done on val.Type as it would return
		// [frontend.Variable]. The check is somewhat fragile as it rely on
		// the type name and the package name. We return nil in that case, be
		// -cause it signifies that the variable belongs to a circuit that has
		// been compiled by gnark. That information is not relevant to
		// serialize.
		if strings.Contains(reflect.TypeOf(variable).String(), "expr.Term") {
			return nil, nil
		}

		return nil, newSerdeErrorf("invalid type for a frontend variable: %T, value: %++v, type.string=%v", variable, variable, reflect.TypeOf(variable).String())
	}

	res, err := marshalBigInt(ser, reflect.ValueOf(bi))
	if err != nil {
		return nil, newSerdeErrorf("could not marshal frontend variable: %w", err)
	}

	return res, nil
}

func unmarshalFrontendVariable(des *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {

	bi, err := unmarshalBigInt(des, val, TypeOfBigInt)
	if err != nil {
		return reflect.Value{}, newSerdeErrorf("could not unmarshal frontend variable: %w", err)
	}

	v := reflect.New(TypeOfFrontendVariable).Elem()
	v.Set(reflect.ValueOf(bi))
	return v, nil
}

func unmarshalHashGenerator(des *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {
	f := func() hash.Hash {
		return mimc.NewMiMC()
	}
	return reflect.ValueOf(f), nil
}

func marshalHashTypeHasher(ser *Serializer, val reflect.Value) (any, *serdeError) {
	return nil, nil
}

func unmarshalHashTypeHasher(des *Deserializer, val any, _ reflect.Type) (reflect.Value, *serdeError) {
	return reflect.ValueOf(hashtypes.MiMC), nil
}

// marshalAsNil is a custom serialization function that marshals the given value
// to nil. It is used for types that are not meant to be serialized, such as
// functions.
func marshalAsNil(_ *Serializer, _ reflect.Value) (any, *serdeError) {
	return nil, nil
}

// unmarshalAsZero is a custom deserialization function that unmarshals the
// given value to zero. It is meant for the type that are not intended to be
// serialized.
func unmarshalAsZero(des *Deserializer, val any, t reflect.Type) (reflect.Value, *serdeError) {
	return reflect.Zero(t), nil
}

// This converts the field.Element to a smaller big.Int. This is done to
// reduce the size of the CBOR encoding. The backward conversion is automatically
// done [field.SetBigInt] as it handles negative values.
func fieldToSmallBigInt(v field.Element) *big.Int {
	neg := new(field.Element).Neg(&v)
	if neg.IsUint64() {
		n := neg.Uint64()
		return new(big.Int).SetInt64(-int64(n))
	}

	bi := &big.Int{}
	v.BigInt(bi)
	return bi
}

// marshalAsEmptyStruct is a custom serialization function that marshals the
// given value to an empty struct. It is used for types that are not meant to be
// serialized, such as functions.
func marshalAsEmptyStruct(_ *Serializer, _ reflect.Value) (any, *serdeError) {
	return struct{}{}, nil
}

// makeNewPtr creates an object using reflect.New and is indicated for pointer
// types as it creates a pointer to zero object rather than returning a nil
// pointer. The provided type must be a pointer type.
func makeNewObject(_ *Deserializer, _ any, t reflect.Type) (reflect.Value, *serdeError) {
	if t.Kind() != reflect.Ptr {
		return reflect.Value{}, newSerdeErrorf("type %v is not a pointer type", t.String())
	}
	return reflect.New(t.Elem()), nil
}
