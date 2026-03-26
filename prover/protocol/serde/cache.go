package serde

import (
	"reflect"
	"strings"
	"sync"
)

type typeInfo struct {
	isPOD      bool
	isIndirect bool
	binSize    int64
}

var typeInfoCache sync.Map

func getTypeInfo(t reflect.Type) typeInfo {
	if val, ok := typeInfoCache.Load(t); ok {
		return val.(typeInfo)
	}

	info := typeInfo{
		isPOD:      isPOD(t),
		isIndirect: isIndirectType(t),
		binSize:    getBinarySize(t),
	}
	typeInfoCache.Store(t, info)
	return info
}

// getBinarySize: returns how many bytes the serialized representation of type `t` takes.
// NOTE: This is NOT the same as Go's in-memory size. The size returned here reflects
// how the value is represented on disk:
//
// - Fixed-size, pointer-free values are inlined.
// - Variable-size or heap-backed values are replaced by an 8-byte Ref.
//
// This function MUST remain perfectly consistent with the actual write logic;
// any mismatch will result in corrupted offsets or incorrect deserialization.
func getBinarySize(t reflect.Type) int64 {
	// Indirect types (custom registries, ptrs, slices, strings etc) are types that have variable sizes
	// not known at compile time and hence their inline representation is a Ref (8-byte offset).
	// These values are written elsewhere and referenced inline by offset.
	if isIndirectType(t) {
		return 8
	}
	k := t.Kind()
	// Explicit handling of scalar types int/uint are platform-dependent in Go,
	// so we normalize them to 8 bytes in the serialized representation.
	if k == reflect.Int || k == reflect.Uint {
		return 8
	}
	// Structs are serialized inline by concatenating the serialized
	// representation of each exported, non-omitted field.
	if k == reflect.Struct {
		var sum int64
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			if strings.Contains(f.Tag.Get(serdeStructTag), serdeStructTagOmit) {
				continue
			}
			sum += getBinarySize(f.Type)
		}
		return sum
	}
	// Arrays are fixed-size and serialized inline as repeated elements.
	if k == reflect.Array {
		elemSize := getBinarySize(t.Elem())
		return elemSize * int64(t.Len())
	}
	return int64(t.Size())
}

// isIndirectType returns true if the given type is an indirect type.
// Indirect types are types that have variable sizes not known at compile time.
// This includes pointers, slices, strings, interfaces, maps, and functions.
// Direct types are types that have fixed sizes known at compile time.
// This includes structs, arrays, and primitive types (bool,int/uint8, int/uint (normalized), etc).
// Types handled inside of the Custom Registry are also considered indirect.
func isIndirectType(t reflect.Type) bool {
	if _, ok := customRegistry[t]; ok {
		return true
	}
	k := t.Kind()
	return k == reflect.Ptr || k == reflect.Slice || k == reflect.String ||
		k == reflect.Interface || k == reflect.Map || k == reflect.Func
}

// isPOD returns true if the given type is a "Plain Old Data" type which are zero-copy safe.
// Plain Old Data types are types that have fixed sizes known at compile time.
func isPOD(t reflect.Type) bool {
	if isIndirectType(t) {
		return false
	}
	if t.Kind() == reflect.Struct {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			// IMPORTANT: The below two guard rails are important. Otherwise, this function checks ALL struct fields (including unexported ones)
			// to determine if bulk memory copy is safe, while getBinarySize only counts EXPORTED fields when calculating serialized size.
			// When a struct has unexported POD fields, isPOD returns true but getBinarySize returns a smaller value than Go's actual memory size.
			// The bulk copy operations in patchArray and decodeArray then use the wrong byte count (elemBinSize * count instead of
			// sizeof * count), which may cause data corruption for arrays of such structs.
			if !f.IsExported() {
				return false
			}
			// If we omit a field, the memory layout (RAM) and binary layout (File)
			// diverge. We cannot use bulk memory copy for this struct.
			if strings.Contains(f.Tag.Get(serdeStructTag), serdeStructTagOmit) {
				return false
			}
			if !isPOD(f.Type) {
				return false
			}
		}
		return true
	}
	if t.Kind() == reflect.Array {
		return isPOD(t.Elem())
	}

	return true
}

// normalizeIntegerSize converts platform-dependent types (int, uint) to fixed-size
// equivalents (int64, uint64) - 64 bit values. This ensures that the binary representation
// is consistent across different CPU architectures (32-bit vs 64-bit).
func normalizeIntegerSize(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Int:
		return int64(v.Int())
	case reflect.Uint:
		return uint64(v.Uint())
	default:
		return v.Interface()
	}
}
