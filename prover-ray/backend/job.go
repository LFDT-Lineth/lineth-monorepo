package backend

// ProofType identifies which guest program to use for a proof request.
type ProofType string

const (
	// ProofTypeL2Execution proves a range of L2 blocks using the
	// l2-execution guest ELF (riscv-guests/l2-execution/).
	ProofTypeL2Execution ProofType = "l2-execution"
)

// Job is the normalized input to [Core.Prove], after request delivery and
// protocol-specific translation have already happened.
type Job struct {
	// ID is an opaque identifier used to correlate the Result.
	ID string

	// Type selects the guest ELF and any proof-type-specific processing.
	Type ProofType

	// StartBlock and EndBlock identify the block range covered by Payload.
	// Single-block jobs set both to the same block number.
	StartBlock uint64
	EndBlock   uint64

	// Payload is the raw guest input carried into the RISC-V guest data section.
	// [Core.Prove] passes these bytes through [decodePayload], and the guest
	// data-section builder prepends the [u64 LE len] prefix the guest reads at
	// _in_start. Callers supply the guest bytes only and must not add the length
	// prefix themselves.
	//
	// For L2 execution jobs today, Payload is the framed StatelessInput: the
	// 0x0001 schema id followed by the SSZ StatelessInput, exactly the output of
	// utils/ssz.EncodeStatelessInput.
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
// (rollup_spec/src/rollup_spec/prover_io/schemas/getZkL2ExecutionProofV1.response.schema.json).
type PublicInputs struct {
	ParentBlockHash                          [32]byte
	EndBlockHash                             [32]byte
	EndBlockNumber                           uint64
	EndBlockTimestamp                        uint64
	L2L1MessagesHash                         [32]byte
	ParentL1L2BridgeRollingHash              [32]byte
	ParentL1L2BridgeRollingHashMessageNumber uint64
	EndL1L2BridgeRollingHash                 [32]byte
	EndL1L2BridgeRollingHashMessageNumber    uint64
	DynamicChainConfigHash                   [32]byte
	ParentFtxRollingHash                     [32]byte
	ParentProcessedFtxNumber                 uint64
	EndFtxRollingHash                        [32]byte
	EndProcessedFtxNumber                    uint64
	FilteredAddressesHash                    [32]byte
	TxFromsHash                              [32]byte
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
