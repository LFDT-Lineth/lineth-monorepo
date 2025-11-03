package serialization_test

import (
	"path"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/serialization"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
	"github.com/consensys/linea-monorepo/prover/zkevm"
	"github.com/sirupsen/logrus"
)

var (
	z = zkevm.GetTestZkEVM()
)

// Helper function for serialization and deserialization tests
func runSerdeTest(t *testing.T, input any, name string, isSanityCheck, failFast bool) {

	// In case the test panics, log the error but do not let the panic
	// interrupt the test.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic during serialization/deserialization of %s: %v", name, r)
			debug.PrintStack()
		}
	}()

	if input == nil {
		t.Error("test input is nil")
		return
	}

	var output = reflect.New(reflect.TypeOf(input)).Interface()
	var b []byte
	var err error

	// Measure serialization time
	serTime := profiling.TimeIt(func() {
		logrus.Printf("Starting to serialize:%s \n", name)
		b, err = serialization.Serialize(input)
		if err != nil {
			t.Fatalf("Error during serialization of %s: %v", name, err)
		}
	})

	// Measure deserialization time
	desTime := profiling.TimeIt(func() {
		logrus.Printf("Starting to deserialize:%s\n", name)
		err = serialization.Deserialize(b, output)
		if err != nil {
			t.Fatalf("Error during deserialization of %s: %v", name, err)
		}
	})

	// Log results
	t.Logf("%s serialization=%v deserialization=%v buffer-size=%v \n", name, serTime, desTime, len(b))

	if isSanityCheck {
		// Sanity check: Compare exported fields
		t.Logf("Running sanity checks on deserialized object: Comparing if the values matched before and after serialization")
		outputDeref := reflect.ValueOf(output).Elem().Interface()
		if !serialization.DeepCmp(input, outputDeref, failFast) {
			t.Errorf("Mismatch in exported fields of %s during serde", name)
		} else {
			t.Logf("Sanity checks passed for %s", name)
		}
	}
}

func runSerdeTestPerf(t *testing.T, input any, name string) *profiling.PerformanceLog {

	// In case the test panics, log the error but do not let the panic
	// interrupt the test.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic during serialization/deserialization of %s: %v", name, r)
			debug.PrintStack()
		}
	}()

	if input == nil {
		t.Fatal("test input is nil")
	}

	var output = reflect.New(reflect.TypeOf(input)).Interface()
	var b []byte
	var err error

	monitor, err := profiling.StartPerformanceMonitor(name, 100*time.Millisecond, path.Join("perf", name))
	if err != nil {
		t.Fatalf("Error setting up performance monitor: %v", err)
	}

	func() {
		logrus.Printf("Starting to serialize:%s \n", name)
		b, err = serialization.Serialize(input)
		if err != nil {
			t.Fatalf("Error during serialization of %s: %v", name, err)
		}
	}()

	func() {
		logrus.Printf("Starting to deserialize:%s\n", name)
		err = serialization.Deserialize(b, output)
		if err != nil {
			t.Fatalf("Error during deserialization of %s: %v", name, err)
		}
	}()

	perfLog, err := monitor.Stop()
	if err != nil {
		t.Fatalf("Error stopping performance monitor: %v", err)
	}

	return perfLog
}

func justserde(t *testing.B, input any, name string) {
	// In case the test panics, log the error but do not let the panic
	// interrupt the test.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Panic during serialization/deserialization of %s: %v", name, r)
			debug.PrintStack()
		}
	}()

	if input == nil {
		t.Error("test input is nil")
		return
	}

	var output = reflect.New(reflect.TypeOf(input)).Interface()
	var b []byte
	var err error

	b, err = serialization.Serialize(input)
	if err != nil {
		t.Fatalf("Error during serialization of %s: %v", name, err)
	}

	err = serialization.Deserialize(b, output)
	if err != nil {
		t.Fatalf("Error during deserialization of %s: %v", name, err)
	}
}

func TestSerdeZkEVM(t *testing.T) {
	runSerdeTest(t, z, "ZkEVM", true, false)
}

func TestSerdeZKEVMFull(t *testing.T) {

	cfg, err := config.NewConfigFromFileUnchecked("../../config/config-mainnet-limitless.toml")
	if err != nil {
		t.Fatalf("failed to read config file: %s", err)
	}

	var (
		traceLimits = cfg.TracesLimits
		zkEVM       = zkevm.FullZKEVMWithSuite(&traceLimits, zkevm.CompilationSuite{}, cfg)
	)

	runSerdeTest(t, zkEVM, "ZkEVM", true, false)
}
