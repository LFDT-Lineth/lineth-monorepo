package aggregation

import (
	"github.com/consensys/linea-monorepo/prover/backend/dataavailability"
	"github.com/consensys/linea-monorepo/prover/circuits/aggregation"
	pi_interconnection "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// Request collects all the fields used to perform an aggregation request.
type Request struct {

	// List of execution proofs prover responses containing the proofs to
	// aggregate.
	ExecutionProofs []string `json:"executionProofs"`

	// List of Invalidity proofs prover responses containing the proofs to
	// aggregate.
	InvalidityProofs []string `json:"invalidityProofs"`

	// List of the compression proofs prover responses containing the
	// compression proofs to aggregate.
	DecompressionProofs []string `json:"compressionProofs"`

	// Non-serialized fields. Theses are used for testing but must not be
	// used during the actual processing of the request. In particular, they
	// are used to help infer the L2 block range of the aggregation when the
	// filename is not available right away.
	Start_, End_ int

	// Last finalized timestamp. It cannot be infered from the other files. It is
	// used to compute the public inputs of the
	ParentAggregationLastBlockTimestamp uint64 `json:"parentAggregationLastBlockTimestamp"`

	// Last finalized rolling hash. If no L1 messages have been finalized, the
	// prover cannot infer what finalL1RollingHash to supply. That's why we need
	// this field.
	ParentAggregationLastL1RollingHash              string `json:"parentAggregationLastL1RollingHash"`
	ParentAggregationLastL1RollingHashMessageNumber int    `json:"parentAggregationLastL1RollingHashMessageNumber"`

	// Parent zk root hash of the state over which we want to finalize. In 0x
	// prefixed hexstring. If nothing (or zero) is set, we will pass the
	// initial state root hash of the first execution circuit.
	//
	// It can be either a KoalaOctuplet or a Bls12377Fr thus, we use
	// FullBytes32 to represent either. This is needed only for the koalabear
	// migration, afterward it will be simplifiable to only Koalabear.
	ParentAggregationStateRootHashContract types.FullBytes32 `json:"parentAggregationStateRootHashContract"`

	// last finalized stream hash
	ParentAggregationLastFtxRollingHash string `json:"parentAggregationLastFtxRollingHash"`
	// last finalized forced transaction number
	ParentAggregationLastFtxNumber int `json:"parentAggregationLastFtxNumber"`
}

// This struct contains a collection of fields that are to be extracted from the
// files passed in the requets. These fields are used to generate the response.
type CollectedFields struct {

	// Shnarf to be used for the public input generation. Corresponds to the
	// last submitted blob that is part of the aggregation. Given in hexstring
	// format.
	FinalShnarf                  string
	ParentAggregationFinalShnarf string

	// Parent data hash and the list of data hashes to be finalized
	DataHashes     []string
	DataParentHash string

	// Parent zk root hash of the state over which we want to finalize. In 0x
	// prefixed hexstring. If nothing (or zero) is set, we will pass the
	// initial state root hash of the first execution circuit.
	//
	// This field can be either a KoalaOctuplet or a Bls12377Fr thus, we use
	// FullBytes32 to represent either.
	ParentStateRootHashContract types.FullBytes32

	// Timestamp of the last already finalized L2 block
	ParentAggregationLastBlockTimestamp uint

	// Timestamp of the last L2 block to be finalized
	FinalTimestamp uint

	// Rolling hash of the L1 messages received on L2 so far for the state to be
	// currently finalized. In 0x prefixed Hexstring
	LastFinalizedL1RollingHash string
	L1RollingHash              string

	// Number of L1 messages received on L2 so far for the state to be currently
	// finalized. Messaging Feedback loop messaging number - part of public
	// input
	LastFinalizedL1RollingHashMessageNumber uint
	L1RollingHashMessageNumber              uint

	// L2 Merkle roots for L2 -> L1 messages - The prover will be calculating
	// Merkle roots for trees of a set depth
	HowManyL2Msgs uint

	// The root hashes of the L2 messages being sent on L2. These are the root
	// (possibly) several merkle trees of depth `self.L2MsgTreeDepth`. The root
	// hashes are hextring encoded.
	L2MsgRootHashes []string

	// The depth of the Merkle tree for the Merkle roots being anchored. This is
	// not used as part of the public inputs but is nonetheless helpful on the
	// contracts
	L2MsgTreeDepth uint

	// Bytes array indicating which L2 blocks have “MessageSent” events. The
	// index starting from 1 + currentL2BlockNumber. If the value contains 1,
	// it means that that block has events. The field is 0x prefixed encoded.
	L2MessagingBlocksOffsets string

	// Last block number being finalized and the last one already finalized.
	LastFinalizedBlockNumber uint
	FinalBlockNumber         uint

	// IsProoflessJob marks that the job is proofless and that the
	// response is to be written in a dedicated folder.
	IsProoflessJob bool

	// The proof claims for the execution prover
	ProofClaims []aggregation.ProofClaimAssignment

	// ProofClaimSources tracks the source file path for each entry in ProofClaims,
	// in the same order. Used for diagnostic logging.
	ProofClaimSources []string

	ExecutionPI       []public_input.Execution
	DecompressionPI   []dataavailability.Request
	InvalidityPI      []public_input.Invalidity
	InnerCircuitTypes []pi_interconnection.InnerCircuitType // a hint to the aggregation circuit detailing which public input correspond to which actual public input

	// last finalized (forced) transaction number
	LastFinalizedFtxNumber uint
	FinalFtxNumber         uint

	LastFinalizedFtxRollingHash string
	FinalFtxRollingHash         string
}
