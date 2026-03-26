package serialization

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
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

	// If both nil, they are equal
	if ae == nil && be == nil {
		return true
	}

	// If only one is nil, they differ
	if (ae == nil) != (be == nil) {
		logrus.Errorf("Mismatch at %s: one value is nil, the other is not\n", path)
		return false
	}

	// Both non-nil, validate and compare
	errA, errB := ae.Validate(), be.Validate()
	if errA != nil || errB != nil {
		logrus.Errorf("One of the expressions is invalid: path=%s errA=%v, errB=%v\n", path, errA, errB)
		return false
	}

	if ae.ESHash != be.ESHash {
		logrus.Errorf("Mismatch at %s: hashes differ (v1: %v, v2: %v)\n", path, ae.ESHash.Text(16), be.ESHash.Text(16))
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
	case TypeOfColumnNatural, reflect.TypeFor[ifaces.Column]():
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
	equal := true
	for i := 0; i < a.NumField(); i++ {
		structField := a.Type().Field(i)

		// When the field has the omitted tag, we skip it there without any warning.
		if tag, hasTag := structField.Tag.Lookup(SerdeStructTag); hasTag {
			if strings.Contains(tag, SerdeStructTagOmit) ||
				strings.Contains(tag, SerdeStructTagTestOmit) {
				continue
			}
		}

		// Skip unexported fields
		if !structField.IsExported() {
			continue
		}

		fieldA := a.Field(i)
		fieldB := b.Field(i)
		fieldName := structField.Name
		fieldPath := fieldName
		if path != "" {
			fieldPath = path + "." + fieldName
		}

		if !compareExportedFieldsWithPath(cachedPtrs, fieldA, fieldB, fieldPath, failFast) {
			equal = false
			if failFast {
				return false
			}
		}
	}
	return equal
}

func compareSlices(cachedPtrs map[uintptr]struct{}, a, b reflect.Value, path string, failFast bool) bool {
	if a.Len() != b.Len() {
		logrus.Printf("Mismatch at %s: slice lengths differ (v1: %v, v2: %v, type: %v)\n", path, a.Len(), b.Len(), a.Type())
		return false
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
