package serde

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/sirupsen/logrus"
)

// DeepCmp checks if two values are equal, ignoring unexported fields, including
// in nested structs. It logs mismatched fields with their paths and values. If
// failFast is true, it returns after the first mismatch. It can be seen as a
// more powerful version of reflect.DeepEqual that also return all the
// mismatches.
//
// The function returns a boolean that indicates if the values are deeply-equal
// and a list of errors and warnings.
//
// It should be noted that the function does not behave like reflect.DeepEqual
// in every aspect. For instance, it disregards unexported fields and adds a
// warning when one is found. Second of all, it adds a warning when a and b are
// pointers to the same memory location as this would be fishy when comparing
// values that are serialized-deserialized. It also will return a warninf when
// uncountering non-comparable values like functions pointers but will still
// give a warning when that happens. Finally, the function fully disregards
// values that are tagged with the `serde:"omit` tag.
func DeepCmp(a, b interface{}, failFast bool) bool {
	cachedPtrs := make(map[uintptr]struct{})
	return compareExportedFieldsWithPath(cachedPtrs, reflect.ValueOf(a), reflect.ValueOf(b), "", failFast)
}

func compareExportedFieldsWithPath(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	// Handle invalid values
	if !a.IsValid() || !b.IsValid() {
		// Treat nil and zero values as equivalent
		if !a.IsValid() && !b.IsValid() {
			return true
		}
		if !a.IsValid() {
			a = reflect.Zero(b.Type())
		}
		if !b.IsValid() {
			b = reflect.Zero(a.Type())
		}
	}

	// Type check after normalization of invalid values
	if a.Type() != b.Type() {
		logrus.Printf("Mismatch at %s: types differ (v1: %v, v2: %v, types: %v, %v)\n", path, a.Interface(), b.Interface(), a.Type(), b.Type())
		return false
	}

	// Specialized handlers
	switch a.Type() {
	case reflect.TypeFor[*symbolic.Expression]():
		return compareSymbolicExpressions(a, b, path)
	case reflect.TypeFor[frontend.Variable]():
		return true
	}

	switch a.Kind() {
	case reflect.Func:
		// Ignore Func
		return true
	case reflect.Interface:
		return compareExportedFieldsWithPath(cachedPtrs, a.Elem(), b.Elem(), path+".(interface)", failFast)
	case reflect.Map:
		return compareMaps(cachedPtrs, a, b, path, failFast)
	case reflect.Ptr:
		return comparePointers(cachedPtrs, a, b, path, failFast)
	case reflect.Struct:
		return compareStructs(cachedPtrs, a, b, path, failFast)
	case reflect.Slice, reflect.Array:
		return compareSlices(cachedPtrs, a, b, path, failFast)
	default:
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			logrus.Printf("Mismatch at %s: values differ (v1: %v, v2: %v, type_v1: %v type_v2: %v)\n", path, a.Interface(), b.Interface(), a.Type(), b.Type())
			return false
		}
		return true
	}
}

func compareSymbolicExpressions(a, b reflect.Value, path string) bool {
	ae := a.Interface().(*symbolic.Expression)
	be := b.Interface().(*symbolic.Expression)

	// 1. Basic Nil Checks
	if ae == nil && be == nil {
		return true
	}
	if (ae == nil) != (be == nil) {
		logrus.Errorf("Mismatch at %s: one value is nil, the other is not\n", path)
		return false
	}

	// 2. Safe Validation Wrapper (Catches SIGSEGV/Panics)
	var errA, errB error

	// Validate A (Truth)
	func() {
		defer func() {
			if r := recover(); r != nil {
				errA = fmt.Errorf("PANIC during validation: %v", r)
			}
		}()
		errA = ae.Validate()
	}()

	// Validate B (Deserialized)
	func() {
		defer func() {
			if r := recover(); r != nil {
				errB = fmt.Errorf("PANIC during validation: %v", r)
			}
		}()
		errB = be.Validate()
	}()

	// 3. Report Validation Failures without Crashing
	if errA != nil || errB != nil {
		logrus.Warnf("Validation failed at %s. This implies the object was not fully restored.\n\tTruth Err: %v\n\tProver Err: %v", path, errA, errB)
		// We return false to fail the test, but we don't crash the runner.
		return false
	}

	// 4. Compare Hash
	if ae.ESHash != be.ESHash {
		logrus.Errorf("Mismatch at %s: hashes differ (v1: %v, v2: %v)\n", path, ae.ESHash.String(), be.ESHash.String())
		return false
	}

	return true
}

func comparePointers(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	if a.IsNil() && b.IsNil() {
		return true
	}

	if a.IsNil() != b.IsNil() {
		logrus.Printf("Mismatch at %s: nil status differs (v1: %v, v2: %v, type: %v)\n", path, a, b, a.Type())
		return false
	}

	if _, seen := cachedPtrs[a.Pointer()]; seen {
		return true
	}

	cachedPtrs[a.Pointer()] = struct{}{}
	return compareExportedFieldsWithPath(cachedPtrs, a.Elem(), b.Elem(), path, failFast)
}

func compareMaps(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	if a.Len() != b.Len() {
		logrus.Printf("Mismatch at %s: map lengths differ (v1: %v, v2: %v, type: %v)\n", path, a.Len(), b.Len(), a.Type())
		return false
	}

	// The module discoverer uses map[ifaces.Column] and map[column.Natural]
	// These use pointers
	switch a.Type().Key() {
	case reflect.TypeOf(column.Natural{}), reflect.TypeFor[ifaces.Column]():
		return true
	}

	for _, key := range a.MapKeys() {
		valA := a.MapIndex(key)
		valB := b.MapIndex(key)
		if !valB.IsValid() {
			logrus.Printf("Mismatch at %s: key %v is missing in second map\n", path, key)
			return false
		}
		keyPath := fmt.Sprintf("%s[%v]", path, key)
		if !compareExportedFieldsWithPath(cachedPtrs, valA, valB, keyPath, failFast) {
			if failFast {
				return false
			}
		}
	}
	return true
}

func compareStructs(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	// Log progress for top-level struct fields (path has no dots = top-level)
	isTopLevel := !strings.Contains(path, ".")
	parentHasCustomMarshaller := hasCustomMarshaller(a.Type())
	equal := true
	for i := 0; i < a.NumField(); i++ {
		structField := a.Type().Field(i)

		if tag, hasTag := structField.Tag.Lookup(serdeStructTag); hasTag {
			// test_omit fields are always skipped in comparison
			if strings.Contains(tag, SerdeStructTagTestOmit) {
				continue
			}
			if strings.Contains(tag, serdeStructTagOmit) {
				// If the parent struct has a custom marshaller, the omitted
				// field may be reconstructed during deserialization, so we
				// should still compare it — except for sync.Mutex types
				// which cannot be meaningfully compared.
				if !parentHasCustomMarshaller || isMutexType(structField.Type) {
					continue
				}
			}
		}

		if !structField.IsExported() {
			continue
		}

		fieldPath := structField.Name
		if path != "" {
			fieldPath = path + "." + structField.Name
		}

		if isTopLevel {
			logrus.Debugf("DeepCmp: comparing field %d/%d: %s", i+1, a.NumField(), fieldPath)
		}

		if !compareExportedFieldsWithPath(cachedPtrs, a.Field(i), b.Field(i), fieldPath, failFast) {
			equal = false
			if failFast {
				return false
			}
		}
	}
	return equal
}

// isPrimitiveKind returns true if the kind is a simple comparable type that
// can be bulk-compared with reflect.DeepEqual without recursive traversal.
func isPrimitiveKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128,
		reflect.String:
		return true
	}
	return false
}

// isDeepEqualSafe returns true if the type can be compared in bulk with
// reflect.DeepEqual (no pointers, interfaces, funcs, or channels inside).
func isDeepEqualSafe(t reflect.Type) bool {
	return isDeepEqualSafeVisited(t, make(map[reflect.Type]bool))
}

func isDeepEqualSafeVisited(t reflect.Type, visited map[reflect.Type]bool) bool {
	if result, ok := visited[t]; ok {
		return result
	}
	// Optimistically mark as safe to break cycles; will be corrected below.
	visited[t] = true

	switch t.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Func, reflect.Chan, reflect.Map:
		visited[t] = false
		return false
	case reflect.Slice, reflect.Array:
		result := isDeepEqualSafeVisited(t.Elem(), visited)
		visited[t] = result
		return result
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if !isDeepEqualSafeVisited(t.Field(i).Type, visited) {
				visited[t] = false
				return false
			}
		}
		return true
	default:
		return isPrimitiveKind(t.Kind())
	}
}

func compareSlices(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	if a.Len() != b.Len() {
		logrus.Printf("Mismatch at %s: slice lengths differ (v1: %v, v2: %v, type: %v)\n", path, a.Len(), b.Len(), a.Type())
		return false
	}

	if a.Len() == 0 {
		return true
	}

	// Fast path: for slices/arrays of types that contain no pointers,
	// interfaces, or funcs, use reflect.DeepEqual on the whole slice.
	if isDeepEqualSafe(a.Type().Elem()) {
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			logrus.Printf("Mismatch at %s: slice values differ (type: %v, len: %d)\n", path, a.Type(), a.Len())
			return false
		}
		return true
	}

	equal := true
	for i := 0; i < a.Len(); i++ {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		if !compareExportedFieldsWithPath(cachedPtrs, a.Index(i), b.Index(i), elemPath, failFast) {
			equal = false
			if failFast {
				return false
			}
		}
	}
	return equal
}

// hasCustomMarshaller returns true if the given type has a custom serde
// marshaller registered in the customRegistry. Types with custom marshallers
// may reconstruct serde:"omit" fields during deserialization.
func hasCustomMarshaller(t reflect.Type) bool {
	_, ok := customRegistry[t]
	return ok
}

var (
	mutexType    = reflect.TypeOf(sync.Mutex{})
	mutexPtrType = reflect.TypeOf(&sync.Mutex{})
)

// isMutexType returns true if t is sync.Mutex or *sync.Mutex.
func isMutexType(t reflect.Type) bool {
	return t == mutexType || t == mutexPtrType
}
