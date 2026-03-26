package serialization_test

import (
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/serialization"
)

// runSerdeBenchmark is updated to properly benchmark serialization and deserialization separately.
// For serialization benchmark, it serializes b.N times.
// For deserialization benchmark, it serializes once outside the loop, then deserializes b.N times.
func runSerdeBenchmark(b *testing.B, input any, name string, onlySerialize bool) {
	// In case the test panics, log the error but do not let the panic
	// interrupt the test.
	defer func() {
		if r := recover(); r != nil {
			b.Errorf("Panic during serialization/deserialization of %s: %v", name, r)
			debug.PrintStack()
		}
	}()

	if input == nil {
		b.Fatal("test input is nil")
	}

	var output = reflect.New(reflect.TypeOf(input)).Interface()
	var bBytes []byte
	var err error

	// Serialize once to get the bytes for deserialization benchmark
	if !onlySerialize {
		bBytes, err = serialization.Serialize(input)
		if err != nil {
			b.Fatalf("Error during initial serialization of %s: %v", name, err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if onlySerialize {
			// Benchmark only serialization
			_, err = serialization.Serialize(input)
			if err != nil {
				b.Fatalf("Error during serialization of %s: %v", name, err)
			}
		} else {
			// Benchmark only deserialization (using pre-serialized bytes)
			err = serialization.Deserialize(bBytes, output)
			if err != nil {
				b.Fatalf("Error during deserialization of %s: %v", name, err)
			}
		}
	}
}
