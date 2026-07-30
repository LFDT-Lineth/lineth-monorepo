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

	// Payload is the framed StatelessInput for the block: the 0x0001 schema id
	// followed by the SSZ StatelessInput, exactly the output of
	// utils/ssz.EncodeStatelessInput. [Core.Prove] passes these bytes through
	// [decodePayload], and [sszBlobs] prepends the [u64 LE len] prefix the guest
	// reads at _in_start. Callers supply the framed bytes only and must not add
	// the length prefix themselves.
	//
	// Multi-block conflation encoding is not yet decided (open question #1
	// in wiki backend-overview.md); [Core.Prove] rejects jobs spanning more
	// than one block.
	Payload []byte
}

// ResultStatus indicates whether a prove attempt succeeded.
type ResultStatus string

const (
	ResultStatusOK     ResultStatus = "ok"
	ResultStatusFailed ResultStatus = "failed"
)

// PublicInputs carries the 16 public output fields of the coordinator
// response. Some are read from the guest's 105-byte
// SszStatelessValidationResult; that result is too small to hold all 16, so
// the rest are computed by the Lineth wrapper (run_l2_execution_guest).
//
// The wiop mechanism is in place (wiop.RegisterPublicInputs, commit e48fd92f):
// col.At(pos).Open(ctx) exposes a column position as a cell, RegisterPublicInputs
// registers it, and sys.Prove returns its value in wiop.PublicInput. What
// remains is establishing which columns/positions in RISCV-ZKC.bin carry each
// field, and which fields come from the wrapper instead (open question #5).
//
// Count and field names follow the coordinator response schema
// (rollup_spec/src/rollup_spec/prover_io/getZkL2ExecutionProofV1.response.json).
type PublicInputs struct {
	ParentBlockHash      [32]byte
	EndBlockHash         [32]byte
	L2L1MessagesHash     [32]byte
	ParentFtxRollingHash [32]byte
	// Remaining 12 fields: pending column-to-field mapping.
}

// Result is the backend's response for a completed [Job].
type Result struct {
	JobID  string
	Status ResultStatus

	// ProofBytes is the serialized wiop.Proof. Wire format not yet decided
	// (wiki backend-overview.md §6); nil when Status is ResultStatusFailed.
	ProofBytes []byte

	PublicInputs PublicInputs

	// Err is non-nil when Status is ResultStatusFailed.
	Err error
}
