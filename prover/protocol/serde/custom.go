package serde

import (
	"bytes"
	"fmt"
	"hash"
	"math/big"
	"math/bits"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	cs "github.com/consensys/gnark/constraint/koalabear"
	"github.com/consensys/gnark/frontend"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/sirupsen/logrus"
)

type customCodex struct {
	marshall   func(enc *encoder, v reflect.Value) (Ref, error)
	unmarshall func(dec *decoder, v reflect.Value, offset int64) error
}

var customRegistry = map[reflect.Type]customCodex{}

func registerCustomType(t reflect.Type, c customCodex) {
	customRegistry[t] = c
}

func init() {

	idCodex := customCodex{
		marshall:   marshallID,
		unmarshall: unmarshallID,
	}

	registerCustomType(reflect.TypeOf(ifaces.ColID("")), idCodex)
	registerCustomType(reflect.TypeOf(ifaces.QueryID("")), idCodex)
	registerCustomType(reflect.TypeOf(coin.Name("")), idCodex)

	// Value Types
	registerCustomType(reflect.TypeOf(big.Int{}), customCodex{
		marshall:   marshallBigInt,
		unmarshall: unmarshallBigInt})

	registerCustomType(reflect.TypeOf(&big.Int{}), customCodex{
		marshall:   marshallBigIntPtr,
		unmarshall: unmarshallBigInt})

	registerCustomType(reflect.TypeOf((*frontend.Variable)(nil)).Elem(), customCodex{
		marshall:   marshallFrontendVariable,
		unmarshall: unmarshallFrontendVariable,
	})

	// Gnark R1CS
	registerCustomType(reflect.TypeOf(cs.SparseR1CS{}), customCodex{
		marshall:   marshallPlonkCkt,
		unmarshall: unmarshallPlonkCkt})

	// Arithmetization
	registerCustomType(reflect.TypeOf(arithmetization.Arithmetization{}), customCodex{
		marshall:   marshallArithmetization,
		unmarshall: unmarshallArithmetization,
	})

	// RingSIS Key
	registerCustomType(reflect.TypeOf(ringsis.Key{}), customCodex{
		marshall:   marshallRingSisKey,
		unmarshall: unmarshallRingSisKey,
	})

	// FFT Domain
	registerCustomType(reflect.TypeOf(fft.Domain{}), customCodex{
		marshall:   marshallGnarkFFTDomain,
		unmarshall: unmarshallGnarkFFTDomain,
	})

	// Column Store
	registerCustomType(reflect.TypeOf(column.Store{}), customCodex{
		marshall:   marshallColumnStore,
		unmarshall: unmarshallColumnStore,
	})

	// Column Natural / Coin Info
	registerCustomType(reflect.TypeOf(column.Natural{}), customCodex{
		marshall:   marshallColumnNatural,
		unmarshall: unmarshallColumnNatural,
	})
	registerCustomType(reflect.TypeOf(coin.Info{}), customCodex{
		marshall:   marshallCoinInfo,
		unmarshall: unmarshallCoinInfo,
	})

	registerCustomType(reflect.TypeOf(func() hash.Hash { return nil }), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallHashGenerator,
	})

	registerCustomType(reflect.TypeOf(sync.Mutex{}), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallAsZero,
	})
	registerCustomType(reflect.TypeOf(&sync.Mutex{}), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallAsNewPtr,
	})
}

// ---------------- IMPLEMENTATIONS ----------------

var TypeOfQuery = reflect.TypeOf((*ifaces.Query)(nil)).Elem()

func marshallQuery(enc *encoder, v reflect.Value) (Ref, error) {
	q, ok := v.Interface().(ifaces.Query)
	if !ok {
		return 0, fmt.Errorf("value of type %s does not implement ifaces.Query", v.Type())
	}
	id := q.UUID().String()
	if ref, ok := enc.uuidMap[id]; ok {
		traceLog("Query UUID Dedup Hit! %s -> Ref %d", id, ref)
		return ref, nil
	}

	// Linearize the underlying value (struct or pointer to struct) using existing helpers.
	var ref Ref
	var err error
	if v.Kind() == reflect.Ptr {
		// Dereference pointers until we reach the concrete
		elem := v
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		ref, err = linearizeStruct(enc, elem)
	} else if v.Kind() == reflect.Struct {
		ref, err = linearizeStruct(enc, v)
	} else {
		return 0, fmt.Errorf("query implementation must be a struct or pointer to struct, got %v", v.Kind())
	}
	if err != nil {
		return 0, err
	}
	enc.uuidMap[id] = ref
	return ref, nil
}

// marshallID handles string deduplication.
// 1. Checks if string content exists in stringMap.
// 2. If hit: returns existing Ref.
// 3. If miss: writes string data, writes Header, updates map, returns new Ref.
func marshallID(enc *encoder, v reflect.Value) (Ref, error) {
	s := v.String()

	// 1. Check Cache (Deduplication)
	if ref, ok := enc.idMap[s]; ok {
		traceLog("ID cache Hit! '%s' -> Ref %d", s, ref)
		return ref, nil
	}

	// 2. Write Data (Payload)
	// We write the raw bytes to the heap
	startOffset := enc.offset
	enc.buf.WriteString(s)
	enc.offset += int64(len(s))

	// 3. Write Header
	// We create a FileSlice pointing to the payload
	fs := FileSlice{
		Offset: Ref(startOffset),
		Len:    int64(len(s)),
		Cap:    int64(len(s)),
	}

	// Write the header to get the Reference ID
	headerOff := enc.write(fs)
	ref := Ref(headerOff)

	// 4. Update Cache
	enc.idMap[s] = ref
	traceLog("String Intern New: '%s' -> Ref %d", s, ref)

	return ref, nil
}

// unmarshallID reuses the standard string decoding logic.
// The decoder doesn't need a special cache map because the Ref in the file
// already points to the specific header for this string.
func unmarshallID(dec *decoder, v reflect.Value, offset int64) error {
	return dec.decodeString(v, offset)
}

// --- Column Store (STRUCT) ---
func marshallColumnStore(enc *encoder, v reflect.Value) (Ref, error) {
	p, err := ptrFromStruct(v)
	if err != nil {
		return 0, err
	}
	if p == nil {
		return 0, nil
	}
	store := p.(*column.Store)
	packed := store.Pack()
	return encode(enc, reflect.ValueOf(packed))
}

func unmarshallColumnStore(dec *decoder, v reflect.Value, offset int64) error {
	var packed column.PackedStore
	packedVal := reflect.ValueOf(&packed).Elem()
	if err := dec.decode(packedVal, offset); err != nil {
		return err
	}
	newStorePtr := packed.Unpack()
	v.Set(reflect.ValueOf(newStorePtr).Elem())
	return nil
}

// --- Column Natural ---
func marshallColumnNatural(enc *encoder, v reflect.Value) (Ref, error) {
	nat := v.Interface().(column.Natural)
	// 1. Logical Deduplication by UUID
	id := nat.UUID().String()
	if ref, ok := enc.uuidMap[id]; ok {
		traceLog("UUID Dedup Hit! %s -> Ref %d", id, ref)
		return ref, nil
	}

	packed := nat.Pack()

	// Encode the packed struct (this will linearize it to the buffer)
	ref, err := encode(enc, reflect.ValueOf(packed))
	if err != nil {
		return 0, err
	}

	// 2. Register the new reference
	enc.uuidMap[id] = ref

	return ref, nil
}
func unmarshallColumnNatural(dec *decoder, v reflect.Value, offset int64) error {
	var packed column.PackedNatural
	packedVal := reflect.ValueOf(&packed).Elem()
	if err := dec.decode(packedVal, offset); err != nil {
		return err
	}
	nat := packed.Unpack()
	v.Set(reflect.ValueOf(nat))
	return nil
}

// --- Coin Info ---
type PackedCoin struct {
	Type       int8   `serde:"t"`
	Size       int    `serde:"s"`
	UpperBound int32  `serde:"u"`
	Name       string `serde:"n"`
	Round      int    `serde:"r"`
}

func asPackedCoin(c coin.Info) PackedCoin {
	return PackedCoin{Type: int8(c.Type), Size: c.Size, UpperBound: int32(c.UpperBound), Name: string(c.Name), Round: c.Round}
}

func marshallCoinInfo(enc *encoder, v reflect.Value) (Ref, error) {
	c := v.Interface().(coin.Info)
	id := c.UUID().String()
	if ref, ok := enc.uuidMap[id]; ok {
		return ref, nil
	}

	packed := asPackedCoin(c)
	ref, err := encode(enc, reflect.ValueOf(packed))
	if err != nil {
		return 0, err
	}
	enc.uuidMap[id] = ref
	return ref, nil
}

func unmarshallCoinInfo(dec *decoder, v reflect.Value, offset int64) error {
	var packed PackedCoin
	if err := dec.decode(reflect.ValueOf(&packed).Elem(), offset); err != nil {
		return err
	}
	// Handle zero-value coin.Info (uninitialized fields with Type=0)
	if packed.Type == 0 {
		v.Set(reflect.ValueOf(coin.Info{}))
		return nil
	}
	sizes := []int{}
	if packed.Size > 0 {
		sizes = append(sizes, packed.Size)
	}
	if packed.UpperBound > 0 {
		sizes = append(sizes, int(packed.UpperBound))
	}
	unpacked := coin.NewInfo(coin.Name(packed.Name), coin.Type(packed.Type), packed.Round, sizes...)
	v.Set(reflect.ValueOf(unpacked))
	return nil
}

// --- RingSIS Key (STRUCT) ---
// Serialized as 3 consecutive int64s: {LogTwoDegree, LogTwoBound, MaxNumFieldToHash}
func marshallRingSisKey(enc *encoder, v reflect.Value) (Ref, error) {
	p, err := ptrFromStruct(v)
	if err != nil {
		return 0, err
	}
	key := p.(*ringsis.Key)
	logTwoDegree := bits.TrailingZeros(uint(key.SisGnarkCrypto.Degree))
	ref := enc.write(logTwoDegree)            // int → 8 bytes
	enc.write(key.SisGnarkCrypto.LogTwoBound) // int → 8 bytes
	enc.write(key.MaxNumFieldToHash)          // int → 8 bytes
	return Ref(ref), nil
}
func unmarshallRingSisKey(dec *decoder, v reflect.Value, offset int64) error {
	if offset < 0 || int(offset)+24 > len(dec.data) {
		return fmt.Errorf("ringsis key data out of bounds")
	}
	logTwoDegree := int(*(*int64)(unsafe.Pointer(&dec.data[offset])))
	logTwoBound := int(*(*int64)(unsafe.Pointer(&dec.data[offset+8])))
	maxNum := int(*(*int64)(unsafe.Pointer(&dec.data[offset+16])))
	key := ringsis.GenerateKey(logTwoDegree, logTwoBound, maxNum)
	v.Set(reflect.ValueOf(key).Elem())
	return nil
}

// --- Gnark FFT Domain (STRUCT) ---
func marshallGnarkFFTDomain(enc *encoder, v reflect.Value) (Ref, error) {
	p, err := ptrFromStruct(v)
	if err != nil {
		return 0, err
	}
	domain := p.(*fft.Domain)
	startOffset := enc.offset

	// Zero allocation direct write (with no tmp. intermediate buffer) - we can write directly to the
	// encoder's underlying buffer since gnark lib allows it. If not, we'd have to allocate memory to create a
	// tmp. buffer and write to it.
	n, err := domain.WriteTo(enc.buf)
	if err != nil {
		return 0, fmt.Errorf("fft domain write failed: %w", err)
	}

	// Update the encoder cursor manually as the value 'n' tells us exactly how many bytes were written.
	enc.offset += n

	// Write the Header - the data is now explicitly in the file (i.e. encoder buffer), so we just point to it.
	fs := FileSlice{Offset: Ref(startOffset), Len: n, Cap: n}

	// Write the header and return its location
	headerOffset := enc.write(fs)
	return Ref(headerOffset), nil
}
func unmarshallGnarkFFTDomain(dec *decoder, v reflect.Value, offset int64) error {
	// 1. Header Validation
	if offset < 0 || int(offset)+int(SizeOf[FileSlice]()) > len(dec.data) {
		return fmt.Errorf("fft domain header out of bounds")
	}
	fs := (*FileSlice)(unsafe.Pointer(&dec.data[offset]))
	if fs.Offset.IsNull() {
		return nil
	}

	// 2. Data Slicing (Zero-Copy View)
	start := int64(fs.Offset)
	end := start + int64(fs.Len) // Fixed: Cast fs.Len to int64
	if start < 0 || end > int64(len(dec.data)) {
		return fmt.Errorf("fft domain data out of bounds")
	}

	// 3. Reconstruction (Using Upstream Logic)
	// We create a Reader on the mapped bytes. This is very cheap.
	// d.ReadFrom will copy these bytes into the struct fields.
	// This copy is negligible for a struct of this size (approx 200 bytes).
	d := &fft.Domain{}
	if _, err := d.ReadFrom(bytes.NewReader(dec.data[start:end])); err != nil {
		return err
	}

	v.Set(reflect.ValueOf(d).Elem())
	return nil
}

// --- Gnark R1CS (STRUCT) ---
func marshallPlonkCkt(enc *encoder, v reflect.Value) (Ref, error) {
	p, err := ptrFromStruct(v)
	if err != nil {
		return 0, err
	}

	// Type assertion - We assume v holds *cs.SparseR1CS (or cs.SparseR1CS)
	csPtr, ok := p.(*cs.SparseR1CS)
	if !ok {
		return 0, fmt.Errorf("serializeR1CS: expected *cs.SparseR1CS, got %T", p)
	}

	startOffset := enc.offset

	// DIRECT WRITE (Optimization)
	// Just like fft.Domain, R1CS implements io.WriterTo so that we pass the encoder's underlying buffer directly.
	n, err := csPtr.WriteTo(enc.buf)
	if err != nil {
		return 0, fmt.Errorf("R1CS write failed: %w", err)
	}

	// Update the encoder cursor manually
	enc.offset += n

	// Write the Header - the data is now in the file and we just create the pointer to it.
	fs := FileSlice{Offset: Ref(startOffset), Len: n, Cap: n}
	off := enc.write(fs)
	return Ref(off), nil
}

func unmarshallPlonkCkt(dec *decoder, v reflect.Value, offset int64) error {
	if offset < 0 || int(offset)+int(SizeOf[FileSlice]()) > len(dec.data) {
		return fmt.Errorf("R1CS header out of bounds")
	}
	fs := (*FileSlice)(unsafe.Pointer(&dec.data[offset]))
	if fs.Offset.IsNull() {
		return nil
	}

	start := int64(fs.Offset)
	end := start + fs.Len
	if start < 0 || end > int64(len(dec.data)) {
		return fmt.Errorf("R1CS data out of bounds")
	}

	// ReadFrom (Library Safe) - we create a Reader on the mapped bytes.
	csPtr := &cs.SparseR1CS{}
	if _, err := csPtr.ReadFrom(bytes.NewReader(dec.data[start:end])); err != nil {
		return fmt.Errorf("failed to read R1CS: %w", err)
	}

	// Set Result - handle both pointer and value targets using similar logic to your helper
	if v.Kind() == reflect.Ptr {
		v.Set(reflect.ValueOf(csPtr))
	} else {
		v.Set(reflect.ValueOf(csPtr).Elem())
	}
	return nil
}

// --- Arithmetization ---
func marshallArithmetization(enc *encoder, v reflect.Value) (Ref, error) {
	var bodyBuf bytes.Buffer
	if err := linearizeStructBodyMap(enc, v, &bodyBuf); err != nil {
		return 0, err
	}
	off := enc.writeBytes(bodyBuf.Bytes())
	return Ref(off), nil
}

func unmarshallArithmetization(dec *decoder, v reflect.Value, offset int64) error {
	if err := dec.decodeStruct(v, offset); err != nil {
		return err
	}
	arith := v.Addr().Interface().(*arithmetization.Arithmetization)
	var err error
	arith.BinaryFile, arith.Metadata, err = arithmetization.UnmarshalZkEVMBin(arith.ZkEVMBin)
	if err != nil {
		return err
	}
	arith.AirSchema, arith.LimbMapping = arithmetization.CompileZkevmBin(arith.BinaryFile, arith.Settings.OptimisationLevel)
	return nil
}

// --- Big Int, Frontend Variable, SmartVectors, Helpers ---
func marshallBigInt(enc *encoder, v reflect.Value) (Ref, error) {
	if v.Kind() == reflect.Ptr {
		return encodeBigInt(enc, v.Interface().(*big.Int))
	}
	if v.CanAddr() {
		return encodeBigInt(enc, v.Addr().Interface().(*big.Int))
	}
	bi := v.Interface().(big.Int)
	return encodeBigInt(enc, &bi)
}

func marshallBigIntPtr(enc *encoder, v reflect.Value) (Ref, error) {
	if v.IsNil() {
		return 0, nil
	}
	return encodeBigInt(enc, v.Interface().(*big.Int))
}

func unmarshallBigInt(dec *decoder, v reflect.Value, offset int64) error {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return decodeBigInt(dec.data, v.Elem(), offset)
	}
	return decodeBigInt(dec.data, v, offset)
}

func marshallFrontendVariable(enc *encoder, v reflect.Value) (Ref, error) {
	bi := toBigInt(v)
	if bi == nil {
		return 0, nil
	}
	return encodeBigInt(enc, bi)
}
func unmarshallFrontendVariable(dec *decoder, v reflect.Value, offset int64) error {
	bi := new(big.Int)
	if err := decodeBigInt(dec.data, reflect.ValueOf(bi).Elem(), offset); err != nil {
		return err
	}
	v.Set(reflect.ValueOf(bi))
	return nil
}

func marshallAsEmpty(enc *encoder, v reflect.Value) (Ref, error) {
	// FIX: Write 1 byte so that the object has a unique offset/identity in the stream.
	return Ref(enc.write(byte(0))), nil
}

func unmarshallHashGenerator(dec *decoder, v reflect.Value, offset int64) error {
	f := func() hash.Hash { return poseidon2.NewMDHasher() }
	v.Set(reflect.ValueOf(f))
	return nil
}

//	func unmarshallHashTypeHasher(dec *decoder, v reflect.Value, offset int64) error {
//		v.Set(reflect.ValueOf(poseidon2.MDHasher{}))
//		return nil
//	}
func unmarshallAsZero(dec *decoder, v reflect.Value, offset int64) error {
	v.Set(reflect.Zero(v.Type()))
	return nil
}

func unmarshallAsNewPtr(dec *decoder, v reflect.Value, offset int64) error {
	v.Set(reflect.New(v.Type().Elem()))
	return nil
}

func encodeBigInt(enc *encoder, b *big.Int) (Ref, error) {
	if b == nil {
		return 0, nil
	}
	bytes := b.Bytes()
	sign := int64(0)
	if b.Sign() < 0 {
		sign = 1
	}
	startOffset := enc.offset

	// Note: We use enc.buf.Write (raw std. library write) to dump the "payload" (the sign byte + magnitude bytes)
	// directly onto the heap without any metadata. We then manually construct one FileSlice that points to that raw
	// dump. We use enc.write(fs) to write that header properly into the file structure.  This is done essentially to
	// avoid the "double headers" problem (i.e. to get the actual payload, the decoder would have to read a header to
	// find a header to find the bytes - thereby wasting 24 bytes of space and adding CPU overhead unnecessarily).
	enc.buf.WriteByte(byte(sign))
	enc.buf.Write(bytes)
	enc.offset += int64(1 + len(bytes)) // plus 1 for sign byte
	fs := FileSlice{Offset: Ref(startOffset), Len: int64(len(bytes)), Cap: sign}
	off := enc.write(fs)
	return Ref(off), nil
}

func decodeBigInt(data []byte, target reflect.Value, offset int64) error {
	if offset < 0 || int(offset)+int(SizeOf[FileSlice]()) > len(data) {
		return fmt.Errorf("bigint header out of bounds")
	}
	fs := (*FileSlice)(unsafe.Pointer(&data[offset]))
	if fs.Offset.IsNull() {
		return nil
	}
	dataStart := int64(fs.Offset) + 1
	dataLen := int64(fs.Len)
	if dataStart+dataLen > int64(len(data)) {
		return fmt.Errorf("bigint data out of bounds")
	}
	bytes := data[dataStart : dataStart+dataLen]
	bi := new(big.Int).SetBytes(bytes)

	// Note: While encoding we write ths "sign" byte twice once in the encoder buffer and then in the FileSlice.Cap.
	// While it looks redundant, we favour this method for semantic Completeness of the Payload ([Sign Byte] + [Magnitude Bytes])
	// which is a complete, standalone binary representation of a Signed Integer. While decoding, we fetch the sign byte
	// from `FileSlice.Cap` because the decoder has already loaded the struct into the CPU cache while reading `Offset` and `Len`.
	// The `Cap` field is effectively already in a register. This avoids *peeking* into the data blob:
	// reading the sign byte would require following the pointer into the memory-mapped region (`dec.data[offset]`), performing an extra memory load,
	// and then creating a slice for the remaining data.
	if fs.Cap == 1 {
		bi.Neg(bi)
	}
	target.Set(reflect.ValueOf(*bi))
	return nil
}

// toBigInt normalizes any valid frontend.Variable type into a *big.Int.
// It returns nil if the value should be ignored (e.g. nil or expr.Term).
func toBigInt(v reflect.Value) *big.Int {
	// 1. Safety check: Ensure we have a valid value before calling Interface()
	if !v.IsValid() || (v.Kind() == reflect.Interface && v.IsNil()) {
		return nil
	}

	// 2. Type Switch covering ALL supported frontend.Variable types
	switch val := v.Interface().(type) {
	// Signed Integers
	case int:
		return big.NewInt(int64(val))
	case int8:
		return big.NewInt(int64(val))
	case int16:
		return big.NewInt(int64(val))
	case int32:
		return big.NewInt(int64(val))
	case int64:
		return big.NewInt(val)

	// Unsigned Integers
	case uint:
		return new(big.Int).SetUint64(uint64(val))
	case uint8:
		return new(big.Int).SetUint64(uint64(val))
	case uint16:
		return new(big.Int).SetUint64(uint64(val))
	case uint32:
		return new(big.Int).SetUint64(uint64(val))
	case uint64:
		return new(big.Int).SetUint64(val)

	// GNARK Types
	case field.Element:
		bi := new(big.Int)
		val.BigInt(bi)
		return bi
	case big.Int:
		// Copy to avoid mutating the original if the caller modifies the result
		return new(big.Int).Set(&val)
	case *big.Int:
		// Copy to avoid mutating the original
		return new(big.Int).Set(val)

	// String Support (Base-10)
	case string:
		if bi, ok := new(big.Int).SetString(val, 10); ok {
			return bi
		}
		return nil

	default:
		// 3. The Safety Fallback for expr.Term
		// We check the type string to avoid importing internal packages.
		// If it's an internal expression term, we skip it (return nil).
		// The check cannot be done on val.Type as it would return
		// [frontend.Variable]. The check is somewhat fragile as it rely on
		// the type name and the package name. We return nil in that case, be
		// -cause it signifies that the variable belongs to a circuit that has
		// been compiled by gnark. That information is not relevant to
		// serialize.
		if strings.Contains(v.Type().String(), "expr.Term") {
			logrus.Warn("******* Skipping expr.Term *******")
			return nil
		}

		// Unrecognized type
		return nil
	}
}

// ptrFromStruct: a small helper to get pointer safely from struct value
func ptrFromStruct(v reflect.Value) (interface{}, error) {
	// 1. If it's already a pointer, return it directly
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		return v.Interface(), nil
	}

	// 2. If it's a value, try to get its address
	if !v.CanAddr() {
		return nil, fmt.Errorf("cannot address value of type %v", v.Type())
	}
	return v.Addr().Interface(), nil
}
