package circuits

// CircuitID is a type to represent the different circuits.
// It is used to identify the circuit to be used in the prover.
type CircuitID string

const (
	ExecutionCircuitID          CircuitID = "execution"
	ExecutionLargeCircuitID     CircuitID = "execution-large"
	ExecutionLimitlessCircuitID CircuitID = "execution-limitless"
	ExecutionDummyCircuitID     CircuitID = "execution-dummy"

	BlobDecompressionV0CircuitID    CircuitID = "blob-decompression-v0"
	BlobDecompressionV1CircuitID    CircuitID = "blob-decompression-v1"
	BlobDecompressionDummyCircuitID CircuitID = "blob-decompression-dummy"

	AggregationCircuitID CircuitID = "aggregation"

	EmulationCircuitID      CircuitID = "emulation"
	EmulationDummyCircuitID CircuitID = "emulation-dummy"

	PublicInputInterconnectionCircuitID CircuitID = "public-input-interconnection"
)

// MockCircuitID is a type to represent the different mock circuits.
type MockCircuitID int

const (
	MockCircuitIDExecution     MockCircuitID = 0
	MockCircuitIDDecompression MockCircuitID = 6789
	MockCircuitIDEmulation     MockCircuitID = 1
)
