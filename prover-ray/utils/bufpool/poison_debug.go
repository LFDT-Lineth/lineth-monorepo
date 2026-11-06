//go:build bufpooldebug

package bufpool

import (
	"reflect"
	"unsafe"
)

// poisonEnabled turns on the poisoning of the slices a Pool hands out and
// takes back; see [Pool].
const poisonEnabled = true

// poisonByte is the pattern written over every byte of a poisoned slice. As
// a 32-bit word, 0xa5a5a5a5 is above the KoalaBear modulus, so a poisoned
// field element is not even a valid one.
const poisonByte = 0xa5

// poison fills the memory of v with poisonByte. Slices of a type holding
// pointers are left alone: garbage pointers would crash the garbage
// collector.
func poison[T any](v []T) {
	if len(v) == 0 || hasPointers(reflect.TypeFor[T]()) {
		return
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(v))), uintptr(len(v))*unsafe.Sizeof(v[0]))
	b[0] = poisonByte
	for i := 1; i < len(b); i *= 2 {
		copy(b[i:], b[:i])
	}
}

// hasPointers reports whether a value of type t holds a pointer.
func hasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return false
	case reflect.Array:
		return t.Len() > 0 && hasPointers(t.Elem())
	case reflect.Struct:
		for i := range t.NumField() {
			if hasPointers(t.Field(i).Type) {
				return true
			}
		}
		return false
	default:
		return true
	}
}
