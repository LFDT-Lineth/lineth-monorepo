package serialization_test

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/serialization"
)

func TestSerdeFE(t *testing.T) {
	newFieldElement := func(n int64) field.Element {
		var f field.Element
		f.SetBigInt(big.NewInt(n))
		return f
	}

	// Test cases for single field.Element
	singleTests := []struct {
		name string
		val  field.Element
	}{
		{"Zero", field.Element{0, 0, 0, 0}},
		{"Small", newFieldElement(42)},
		{"Large", newFieldElement(1 << 60)},
		{
			"ModulusMinusOne",
			func() field.Element {
				var f field.Element
				f.SetOne()
				f.Neg(&f) // modulus - 1
				return f
			}(),
		},
		{
			"Random",
			func() field.Element {
				var f field.Element
				b := make([]byte, 32)
				_, _ = rand.Read(b)
				f.SetBytes(b)
				return f
			}(),
		},
	}

	for _, tt := range singleTests {
		t.Run("Single_"+tt.name, func(t *testing.T) {
			bytes, err := serialization.Serialize(tt.val)
			if err != nil {
				t.Fatalf("serialize error: %v", err)
			}
			var result field.Element
			if err := serialization.Deserialize(bytes, &result); err != nil {
				t.Fatalf("deserialize error: %v", err)
			}
			if result != tt.val {
				t.Errorf("mismatch: got %v, want %v", result, tt.val)
			}
		})
	}

	// Test cases for []field.Element
	arrayTests := []struct {
		name string
		val  []field.Element
	}{
		{"Empty", []field.Element{}},
		{"Single", []field.Element{newFieldElement(42)}},
		{"Multiple", []field.Element{
			newFieldElement(0),
			newFieldElement(42),
			newFieldElement(1 << 60),
			func() field.Element {
				var f field.Element
				f.SetOne()
				f.Neg(&f)
				return f
			}(),
		}},
		{
			"Repeated",
			[]field.Element{
				newFieldElement(123),
				newFieldElement(123),
				newFieldElement(123),
			},
		},
		{
			"BoundaryNearModulus",
			[]field.Element{
				func() field.Element {
					var f field.Element
					f.SetOne()
					f.Neg(&f)
					return f
				}(),
				newFieldElement(1),
			},
		},
		{
			"LargeArray",
			func() []field.Element {
				arr := make([]field.Element, 1000)
				for i := 0; i < len(arr); i++ {
					arr[i] = newFieldElement(int64(i * i))
				}
				return arr
			}(),
		},
		{
			"RandomElements",
			func() []field.Element {
				arr := make([]field.Element, 100)
				for i := range arr {
					var f field.Element
					b := make([]byte, 32)
					_, _ = rand.Read(b)
					f.SetBytes(b)
					arr[i] = f
				}
				return arr
			}(),
		},
	}

	for _, tt := range arrayTests {
		t.Run("Array_"+tt.name, func(t *testing.T) {
			bytes, err := serialization.Serialize(tt.val)
			if err != nil {
				t.Fatalf("serialize error: %v", err)
			}
			var result []field.Element
			if err := serialization.Deserialize(bytes, &result); err != nil {
				t.Fatalf("deserialize error: %v", err)
			}
			if len(result) != len(tt.val) {
				t.Fatalf("length mismatch: got %d, want %d", len(result), len(tt.val))
			}
			for i := range tt.val {
				if result[i] != tt.val[i] {
					t.Errorf("element %d mismatch: got %v, want %v", i, result[i], tt.val[i])
				}
			}
		})
	}
}

func TestFieldElementLimbMismatch(t *testing.T) {
	// Construct a field.Element with known limb values
	original := [4]uint64{
		4432961018360255618, // limb 0
		1234567890123456789, // limb 1
		9876543210987654321, // limb 2
		1111111111111111111, // limb 3
	}

	// Serialize it
	bytes, err := serialization.Serialize(original)
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Deserialize it
	var result [4]uint64
	if err := serialization.Deserialize(bytes, &result); err != nil {
		t.Fatalf("failed to deserialize: %v", err)
	}

	// Compare limbs manually and print diff
	for i := 0; i < 4; i++ {
		if result[i] != original[i] {
			t.Errorf("field.Element limb mismatch at [%d]: got %d, want %d", i, result[i], original[i])
		}
	}
}
