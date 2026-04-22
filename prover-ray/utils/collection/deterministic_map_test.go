package collection

import (
	"testing"
)

func TestMakeDeterministicMap(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	if dm.Len() != 0 {
		t.Errorf("Expected length 0, got %d", dm.Len())
	}
	if cap(dm.Values) < 10 {
		t.Errorf("Expected capacity at least 10, got %d", cap(dm.Values))
	}
}

func TestInsertNewAndGet(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)

	if dm.Len() != 1 {
		t.Errorf("Expected length 1, got %d", dm.Len())
	}

	val, ok := dm.Get("key1")
	if !ok || val != 100 {
		t.Errorf("Expected value 100, got %v (ok=%v)", val, ok)
	}
}

func TestInsertNewDuplicate(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when inserting duplicate key")
		}
	}()

	dm.InsertNew("key1", 200)
}

func TestSetNew(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.Set("key1", 100)

	val, ok := dm.Get("key1")
	if !ok || val != 100 {
		t.Errorf("Expected value 100, got %v (ok=%v)", val, ok)
	}
}

func TestSetExisting(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)
	dm.Set("key1", 200)

	val, ok := dm.Get("key1")
	if !ok || val != 200 {
		t.Errorf("Expected value 200, got %v (ok=%v)", val, ok)
	}
	if dm.Len() != 1 {
		t.Errorf("Expected length 1, got %d", dm.Len())
	}
}

func TestGetNonexistent(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	val, ok := dm.Get("nonexistent")
	if ok {
		t.Errorf("Expected ok=false for nonexistent key")
	}
	if val != 0 {
		t.Errorf("Expected zero value, got %v", val)
	}
}

func TestGetPtr(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)

	ptr, ok := dm.GetPtr("key1")
	if !ok || ptr == nil {
		t.Errorf("Expected valid pointer, got %v (ok=%v)", ptr, ok)
	}
	if *ptr != 100 {
		t.Errorf("Expected value 100, got %d", *ptr)
	}

	// Modify through pointer
	*ptr = 200
	val, _ := dm.Get("key1")
	if val != 200 {
		t.Errorf("Expected value 200 after pointer modification, got %d", val)
	}
}

func TestGetPtrNonexistent(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	ptr, ok := dm.GetPtr("nonexistent")
	if ok || ptr != nil {
		t.Errorf("Expected ok=false and nil pointer for nonexistent key")
	}
}

func TestMustGet(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)

	val := dm.MustGet("key1")
	if val != 100 {
		t.Errorf("Expected value 100, got %d", val)
	}
}

func TestMustGetNonexistent(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting nonexistent key with MustGet")
		}
	}()

	dm.MustGet("nonexistent")
}

func TestHasKey(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)

	if !dm.HasKey("key1") {
		t.Error("Expected HasKey to return true for existing key")
	}
	if dm.HasKey("nonexistent") {
		t.Error("Expected HasKey to return false for nonexistent key")
	}
}

func TestIterKey(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)
	dm.InsertNew("key2", 200)
	dm.InsertNew("key3", 300)

	keys := make([]string, 0)
	for k := range dm.IterKey() {
		keys = append(keys, k)
	}

	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Check order is deterministic
	expected := []string{"key1", "key2", "key3"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("Expected key %s at position %d, got %s", expected[i], i, k)
		}
	}
}

func TestIterValues(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)
	dm.InsertNew("key2", 200)
	dm.InsertNew("key3", 300)

	values := make([]int, 0)
	for v := range dm.IterValues() {
		values = append(values, v)
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}

	expected := []int{100, 200, 300}
	for i, v := range values {
		if v != expected[i] {
			t.Errorf("Expected value %d at position %d, got %d", expected[i], i, v)
		}
	}
}

func TestIterKeyEarlyExit(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)
	dm.InsertNew("key2", 200)
	dm.InsertNew("key3", 300)

	count := 0
	for k := range dm.IterKey() {
		count++
		if count == 2 {
			break
		}
		_ = k
	}

	if count != 2 {
		t.Errorf("Expected to iterate 2 items, got %d", count)
	}
}

func TestValueSlice(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	dm.InsertNew("key1", 100)
	dm.InsertNew("key2", 200)

	slice := dm.ValueSlice()

	if len(slice) != 2 {
		t.Errorf("Expected slice length 2, got %d", len(slice))
	}
	if slice[0] != 100 || slice[1] != 200 {
		t.Errorf("Expected [100, 200], got %v", slice)
	}
}

func TestLen(t *testing.T) {
	dm := MakeDeterministicMap[string, int](10)

	if dm.Len() != 0 {
		t.Errorf("Expected length 0, got %d", dm.Len())
	}

	dm.InsertNew("key1", 100)
	if dm.Len() != 1 {
		t.Errorf("Expected length 1, got %d", dm.Len())
	}

	dm.Set("key1", 200)
	if dm.Len() != 1 {
		t.Errorf("Expected length 1, got %d", dm.Len())
	}
}

func TestComplexWorkflow(t *testing.T) {
	dm := MakeDeterministicMap[int, string](5)

	dm.InsertNew(1, "one")
	dm.InsertNew(2, "two")
	dm.InsertNew(3, "three")

	dm.Set(2, "TWO")

	if v, _ := dm.Get(2); v != "TWO" {
		t.Errorf("Expected 'TWO', got %s", v)
	}

	values := dm.ValueSlice()
	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}
}
