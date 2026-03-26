package serde

import (
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test structs for comparator tests

// structWithCustomMarshaller simulates a type like ringsis.Key that has
// a custom marshaller registered and contains serde:"omit" fields that
// are reconstructed during deserialization.
type structWithCustomMarshaller struct {
	Name          string `serde:"n"`
	Reconstructed int    `serde:"omit"`
}

// structWithoutCustomMarshaller has serde:"omit" fields but no custom
// marshaller, so omitted fields are truly absent after deserialization.
type structWithoutCustomMarshaller struct {
	Name    string `serde:"n"`
	Skipped int    `serde:"omit"`
}

// structWithTestOmit has a field tagged with test_omit which should
// always be skipped in DeepCmp regardless of custom marshallers.
type structWithTestOmit struct {
	Name        string `serde:"n"`
	TestOmitted int    `serde:"test_omit"`
}

// structWithMutex has a custom marshaller and a sync.Mutex field tagged
// serde:"omit" which should still be skipped in DeepCmp.
type structWithMutex struct {
	Name string     `serde:"n"`
	Mu   sync.Mutex `serde:"omit"`
}

// structWithMutexPtr has a custom marshaller and a *sync.Mutex field.
type structWithMutexPtr struct {
	Name string      `serde:"n"`
	Mu   *sync.Mutex `serde:"omit"`
}

func init() {
	// Register custom marshallers for test types that simulate the
	// ringsis.Key / arithmetization.Arithmetization pattern.
	registerCustomType(reflect.TypeOf(structWithCustomMarshaller{}), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallAsZero,
	})
	registerCustomType(reflect.TypeOf(structWithMutex{}), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallAsZero,
	})
	registerCustomType(reflect.TypeOf(structWithMutexPtr{}), customCodex{
		marshall:   marshallAsEmpty,
		unmarshall: unmarshallAsZero,
	})
}

func TestDeepCmp_OmittedFieldWithCustomMarshaller_IsCompared(t *testing.T) {
	// When a struct type has a custom marshaller, serde:"omit" fields should
	// be compared because the custom marshaller may reconstruct them.
	a := structWithCustomMarshaller{Name: "test", Reconstructed: 42}
	b := structWithCustomMarshaller{Name: "test", Reconstructed: 42}
	assert.True(t, DeepCmp(a, b, true), "identical values should match")

	// Mismatch in the omitted field should be detected
	c := structWithCustomMarshaller{Name: "test", Reconstructed: 99}
	assert.False(t, DeepCmp(a, c, true), "different Reconstructed values should mismatch")
}

func TestDeepCmp_OmittedFieldWithoutCustomMarshaller_IsSkipped(t *testing.T) {
	// When a struct type has NO custom marshaller, serde:"omit" fields should
	// be skipped (they are truly absent after deserialization).
	a := structWithoutCustomMarshaller{Name: "test", Skipped: 42}
	b := structWithoutCustomMarshaller{Name: "test", Skipped: 0}
	assert.True(t, DeepCmp(a, b, true), "omitted field difference should be ignored without custom marshaller")
}

func TestDeepCmp_TestOmitField_AlwaysSkipped(t *testing.T) {
	// Fields tagged serde:"test_omit" should always be skipped, even if
	// the struct has a custom marshaller.
	a := structWithTestOmit{Name: "test", TestOmitted: 42}
	b := structWithTestOmit{Name: "test", TestOmitted: 0}
	assert.True(t, DeepCmp(a, b, true), "test_omit field difference should always be ignored")
}

func TestDeepCmp_MutexField_SkippedEvenWithCustomMarshaller(t *testing.T) {
	// sync.Mutex fields should be skipped even if the parent struct has
	// a custom marshaller, since they can't be meaningfully compared.
	a := structWithMutex{Name: "test"}
	b := structWithMutex{Name: "test"}
	a.Mu.Lock() // Lock a's mutex to make them differ
	// Note: we can't unlock in defer because the test would deadlock
	// if DeepCmp tried to compare the mutex internals.
	assert.True(t, DeepCmp(&a, &b, true), "mutex field should be skipped")
}

func TestDeepCmp_MutexPtrField_SkippedEvenWithCustomMarshaller(t *testing.T) {
	a := structWithMutexPtr{Name: "test", Mu: &sync.Mutex{}}
	b := structWithMutexPtr{Name: "test", Mu: nil}
	assert.True(t, DeepCmp(a, b, true), "*sync.Mutex field should be skipped")
}

func TestHasCustomMarshaller(t *testing.T) {
	assert.True(t, hasCustomMarshaller(reflect.TypeOf(structWithCustomMarshaller{})))
	assert.False(t, hasCustomMarshaller(reflect.TypeOf(structWithoutCustomMarshaller{})))
}

func TestIsMutexType(t *testing.T) {
	assert.True(t, isMutexType(reflect.TypeOf(sync.Mutex{})))
	assert.True(t, isMutexType(reflect.TypeOf(&sync.Mutex{})))
	assert.False(t, isMutexType(reflect.TypeOf(42)))
	assert.False(t, isMutexType(reflect.TypeOf("hello")))
}

// deeplyNested exercises isDeepEqualSafe's recursion through struct/array nesting.
type innerStruct struct {
	Vals [4]uint64
}
type outerStruct struct {
	Arr [8]innerStruct
	X   int
}

func TestIsDeepEqualSafe_NestedStruct(t *testing.T) {
	assert.True(t, isDeepEqualSafe(reflect.TypeOf(outerStruct{})),
		"nested struct with only value types should be deep-equal safe")
}

// structWithPtr is NOT deep-equal safe because it contains a pointer.
type structWithPtr struct {
	X   int
	Ptr *int
}

func TestIsDeepEqualSafe_BasicTypes(t *testing.T) {
	assert.True(t, isDeepEqualSafe(reflect.TypeOf(42)))
	assert.True(t, isDeepEqualSafe(reflect.TypeOf("hello")))
	assert.True(t, isDeepEqualSafe(reflect.TypeOf([3]int{})))
	assert.True(t, isDeepEqualSafe(reflect.TypeOf([]int{})), "slice of primitives is safe")
	assert.False(t, isDeepEqualSafe(reflect.TypeOf((*int)(nil))), "pointer type")
	assert.False(t, isDeepEqualSafe(reflect.TypeOf(structWithPtr{})), "struct with pointer field")
	assert.False(t, isDeepEqualSafe(reflect.TypeOf([]*int{})), "slice of pointers")
}
