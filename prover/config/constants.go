package config

const (
	VerifyingKeyFileName      = "verifying_key.bin"
	CircuitFileName           = "circuit.bin"
	VerifierContractFileName  = "Verifier.sol"
	ManifestFileName          = "manifest.json"
	DefaultDictionaryFileName = "compressor_dict.bin"

	RequestsFromSubDir = "requests"
	RequestsToSubDir   = "responses"
	RequestsDoneSubDir = "requests-done"

	InProgressSuffix = "inprogress"
	FailSuffix       = "code"
	SuccessSuffix    = "success"

	// Extension to add in order to defer the job to the large prover
	LargeSuffix = "large"
)
