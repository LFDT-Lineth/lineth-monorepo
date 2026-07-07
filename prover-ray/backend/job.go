package backend

// ProofType identifies which guest program to use for a proof request.
// It corresponds to proof_type in the Prover Gateway job protocol.
type ProofType string

const (
	// ProofTypeL2Execution proves a range of L2 blocks using the
	// l2-execution guest ELF (riscv-guests/l2-execution/).
	ProofTypeL2Execution ProofType = "l2-execution"
)

// Job is a single unit of proving work, delivery-system-agnostic.
// The gateway worker or the filesystem controller both produce Jobs;
// [Core.Prove] consumes them.
type Job struct {
	// ID is an opaque identifier used to correlate the Result.
	ID string

	// Type selects the guest ELF and any proof-type-specific processing.
	Type ProofType

	StartBlock uint64
	EndBlock   uint64

	// Payload is the raw SSZ bytes for the block range (already framed or
	// not yet — see [EncodeStatelessInput]). For l2-execution, this is the
	// SSZ-encoded StatelessInput that the guest reads at _in_start.
	//
	// Multi-block conflation encoding is not yet decided (open question #1
	// in backend-overview.md).
	Payload []byte
}

// ResultStatus indicates whether a prove attempt succeeded.
type ResultStatus string

const (
	ResultStatusOK     ResultStatus = "ok"
	ResultStatusFailed ResultStatus = "failed"
)

// PublicInputs carries the public output fields from the guest's
// 105-byte SszStatelessValidationResult.
//
// Extraction from wiop.Proof is not yet implemented (backend-overview.md §7).
// Field names follow the coordinator response schema
// (rollup_spec/src/rollup_spec/prover_io/getZkL2ExecutionProofV1.response.json).
type PublicInputs struct {
	ParentBlockHash      [32]byte
	EndBlockHash         [32]byte
	L2L1MessagesHash     [32]byte
	ParentFtxRollingHash [32]byte
	// TODO(open): remaining 11 fields from SszStatelessValidationResult once
	// the column-to-field mapping is established (backend-overview.md §7).
}

// Result is the backend's response for a completed [Job].
type Result struct {
	JobID  string
	Status ResultStatus

	// ProofBytes is the serialized wiop.Proof. Wire format not yet decided
	// (backend-overview.md §6); nil when Status is ResultStatusFailed.
	ProofBytes []byte

	PublicInputs PublicInputs

	// Err is non-nil when Status is ResultStatusFailed.
	Err error
}
