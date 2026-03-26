package serde_test

import (
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/serde"
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
		bBytes, err = serde.Serialize(input)
		if err != nil {
			b.Fatalf("Error during initial serialization of %s: %v", name, err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if onlySerialize {
			// Benchmark only serialization
			_, err = serde.Serialize(input)
			if err != nil {
				b.Fatalf("Error during serialization of %s: %v", name, err)
			}
		} else {
			// Benchmark only deserialization (using pre-serialized bytes)
			err = serde.Deserialize(bBytes, output)
			if err != nil {
				b.Fatalf("Error during deserialization of %s: %v", name, err)
			}
		}
	}
}

// Benchmark functions
func BenchmarkSerIOP1(b *testing.B) {
	benchmarkScenario(b, "iop1", true) // true for serialization only
}

func BenchmarkDeserIOP1(b *testing.B) {
	benchmarkScenario(b, "iop1", false) // false for deserialization only
}

func BenchmarkSerIOP2(b *testing.B) {
	benchmarkScenario(b, "iop2", true)
}

func BenchmarkDeserIOP2(b *testing.B) {
	benchmarkScenario(b, "iop2", false)
}

func BenchmarkSerIOP3(b *testing.B) {
	benchmarkScenario(b, "iop3", true)
}

func BenchmarkDeserIOP3(b *testing.B) {
	benchmarkScenario(b, "iop3", false)
}

func BenchmarkSerIOP4(b *testing.B) {
	benchmarkScenario(b, "iop4", true)
}

func BenchmarkDeserIOP4(b *testing.B) {
	benchmarkScenario(b, "iop4", false)
}

func BenchmarkSerIOP5(b *testing.B) {
	benchmarkScenario(b, "iop5", true)
}

func BenchmarkDeserIOP5(b *testing.B) {
	benchmarkScenario(b, "iop5", false)
}

func BenchmarkSerIOP6(b *testing.B) {
	benchmarkScenario(b, "iop6", true)
}

func BenchmarkDeserIOP6(b *testing.B) {
	benchmarkScenario(b, "iop6", false)
}

// Helper function to run benchmark for a specific scenario
func benchmarkScenario(b *testing.B, scenarioName string, onlySerialize bool) {
	// Find the scenario
	var scenario *serdeScenario
	for _, s := range serdeScenarios {
		if s.name == scenarioName {
			scenario = &s
			break
		}
	}

	if scenario == nil {
		b.Fatalf("Scenario %s not found", scenarioName)
	}

	if !scenario.benchmark {
		b.Skipf("Scenario %s is not configured for benchmarking", scenarioName)
		return
	}

	comp := getScenarioComp(scenario)
	runSerdeBenchmark(b, comp, scenarioName, onlySerialize)
}
