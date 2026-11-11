package backend

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/conglomeration"

// ProofType identifies which guest program to use for a proof request.
type ProofType string

const (
	// ProofTypeL2Execution proves a range of L2 blocks using the
	// l2-execution guest ELF (riscv-guests/l2_execution/).
	ProofTypeL2Execution ProofType = "l2-execution"

	// ProofTypeRollup proves the compression / data-availability stage
	// (getZkRollupProofV1). "rollup" matches rollup_spec.
	ProofTypeRollup ProofType = "rollup"

	// ProofTypeRollupAggregation proves the aggregation stage
	// (getZkRollupAggregationProofV1).
	ProofTypeRollupAggregation ProofType = "rollup-aggregation"
)

// Job is the normalized input to [Core.Prove], after request delivery and
// protocol-specific translation have already happened.
type Job struct {
	// ID is an opaque identifier used to correlate the Result.
	ID string

	// Type selects any proof-type-specific processing.
	Type ProofType

	// GuestProgramID is the Program ID (keccak256 of the guest ELF) of the guest
	// to run, from the request's guestProgramId.
	GuestProgramID [32]byte

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
	// Multi-block conflation encoding is not yet decided; [Core.Prove] rejects
	// jobs spanning more than one block.
	Payload []byte

	// ShardIndexes lists the trace shards this prover proves, in any order;
	// empty proves them all. Without sharding ([Config.ShardSize] zero) only
	// shard 0 exists. The shard count depends on the guest input and is only
	// known after the job's fast-mode pass, so an index past the end fails the
	// job. A strict subset of the shards yields [ResultStatusNode], not a final
	// proof.
	ShardIndexes []uint
}

// ResultStatus indicates whether a prove attempt succeeded.
type ResultStatus string

const (
	ResultStatusOK     ResultStatus = "ok"
	ResultStatusFailed ResultStatus = "failed"
	// ResultStatusNode is a proof of a strict subset of the trace's shards: a
	// conglomeration node to be merged elsewhere, not a final proof.
	// ProofBytes and PublicInputs are empty; Node and NumShards are set.
	ResultStatusNode ResultStatus = "node"
)

// PublicInputs carries the 16 public output fields of the coordinator
// response. Some are read from the guest's 105-byte
// SszStatelessValidationResult; the rest are computed by the Lineth wrapper
// (run_l2_execution_guest). Which columns of RISCV-ZKC.bin carry each field,
// and which come from the wrapper, is not yet established; see
// zkcdriver/risc5.RegisterPublicInputs.
//
// Count and field names follow the getZkL2ExecutionProofV1 response schema.
type PublicInputs struct {
	ParentBlockHash                       [32]byte
	EndBlockHash                          [32]byte
	EndBlockNumber                        uint64
	EndBlockTimestamp                     uint64
	EndL1L2BridgeRollingHash              [32]byte
	EndL1L2BridgeRollingHashMessageNumber uint64
	DynamicChainConfigHash                [32]byte
	ParentFtxRollingHash                  [32]byte
	ParentFtxNumber                       uint64
	EndFtxRollingHash                     [32]byte
	EndProcessedFtxNumber                 uint64
	FilteredAddresses                     [][20]byte
	TxFromsHash                           [32]byte
	BlockCount                            uint64
	L2MessagingBlocksOffsets              []uint16
	L2L1Messages                          [][32]byte
}

// Result is the backend's response for a completed [Job].
type Result struct {
	JobID  string
	Status ResultStatus

	// ProofBytes is the proof output for the mode. For full it is the serialized
	// wiop.Proof (wire format not yet decided); for partial it is the image of
	// a devmode proof of the guest output hash; for dev-zkvm it is the guest's
	// 0x0003 output from the ZkC Execute run, which the runner cross-checks
	// against the native runner. nil when Status is not ResultStatusOK.
	ProofBytes []byte

	PublicInputs PublicInputs

	// GuestOutput is the guest's 0x0003 output (hash || frame) in partial, for
	// the runner's optional native runner check. nil in the other modes.
	GuestOutput []byte

	// Node is the non-root conglomeration node when Status is
	// ResultStatusNode: the shard proof itself for a single requested shard,
	// a merged node otherwise. NumShards is the trace's shard count, so that
	// the party merging the nodes can tell when their coverage is complete.
	Node      *conglomeration.Node
	NumShards uint

	// Err is non-nil when Status is ResultStatusFailed.
	Err error
}
