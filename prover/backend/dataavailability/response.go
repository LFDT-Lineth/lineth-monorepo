package dataavailability

// The decompression proof response contains all the fields of the requests
// plus some prover related fields. We keep all the fields from the request so
// that we can be sure that the prover will have all the relevant fields.
type Response struct {
	// All the fields passed by the request. All with the same JSON spec
	Request

	// Version ID for the prover
	ProverVersion string `json:"proverVersion"`

	// The shasum of the verifier key to use to verify the proof. This is used
	// by the aggregation circuit to identify the circuit ID to use in the proof.
	VerifyingKeyShaSum string `json:"verifyingKeyShaSum"`

	// The proof produced to assess the decompression. In hexstring. The proof
	// is defined over the field bls12-377.
	DecompressionProof string `json:"decompressionProof"`

	// Debug fields that are helpful for debugging and access intermediate
	// values corresponding to the generated proof.
	Debug struct {
		// Expected public input of the proof
		PublicInput string `json:"publicInput"`
	} `json:"debug"`
}
