package config

const (
	// VerifyingKeyFileName is the plonk verifying key for a gnark outer circuit.
	VerifyingKeyFileName = "verifying_key.bin"
	// CircuitFileName is the compiled gnark outer circuit (constraint system).
	CircuitFileName = "circuit.bin"
	// VerifierContractFileName is the Solidity verifier contract.
	VerifierContractFileName = "Verifier.sol"
	// ManifestFileName stores checksums and metadata for a circuit's setup assets.
	ManifestFileName = "manifest.json"
	// DefaultDictionaryFileName is the default blob decompression dictionary.
	DefaultDictionaryFileName = "compressor_dict.bin"

	// ExecutionCircuitBinFileName is the serialized inner circuit (wizard IOP),
	// produced by protocol/serde. Stored alongside the outer circuit assets.
	ExecutionCircuitBinFileName = "execution-circuit.bin"

	RequestsFromSubDir = "requests"
	RequestsToSubDir   = "responses"
	RequestsDoneSubDir = "requests-done"

	InProgressSuffix = "inprogress"
	FailSuffix       = "code"
	SuccessSuffix    = "success"

	// LargeSuffix is the extension to add in order to defer the job to the large prover.
	LargeSuffix = "large"
)
