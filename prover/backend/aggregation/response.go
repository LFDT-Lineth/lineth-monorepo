package aggregation

// Response contains all the fields returned by the prover to run the
// aggregation. Reflects the data to be sent to the smart-contract for
// finalization
//
//	struct FinalizationData {
//	   bytes32 parentStateRootHash;
//	   bytes32[] dataHashes;
//	   bytes32 dataParentHash;
//	   uint256 finalBlockNumber;
//	   uint256 lastFinalizedTimestamp;
//	   uint256 finalTimestamp;
//	   bytes32 l1RollingHash;
//	   uint256 l1RollingHashMessageNumber;
//	   bytes32[] l2MerkleRoots;
//	   uint256 l2MerkleTreesDepth;
//	   uint256[] l2MessagingBlocksOffsets;
//	 }
type Response struct {

	// Shnarf to be used for the public input generation. Corresponds to the
	// last submitted blob that is part of the aggregation. Given in hexstring
	// format.
	FinalShnarf                  string `json:"finalShnarf"`
	ParentAggregationFinalShnarf string `json:"parentAggregationFinalShnarf"`

	// Aggregation proof in hexstring format
	AggregatedProof         string `json:"aggregatedProof"`
	AggregatedProverVersion string `json:"aggregatedProverVersion"`
	AggregatedVerifierIndex int    `json:"aggregatedVerifierIndex"`

	// Modulo reduced public input to be used to verify the proof.
	AggregatedProofPublicInput string `json:"aggregatedProofPublicInput"`

	// Parent data hash and the list of data hashes to be finalized
	DataHashes     []string `json:"dataHashes"`
	DataParentHash string   `json:"dataParentHash"`

	// ParentStateRootHash is the root hash of the last finalized state.
	// 0x-prefixed hexstring
	ParentStateRootHash string `json:"parentStateRootHash"`

	// The timestamp before and after what is finalized
	ParentAggregationLastBlockTimestamp uint `json:"parentAggregationLastBlockTimestamp"`
	LastFinalizedBlockNumber            uint `json:"lastFinalizedBlockNumber"`
	FinalTimestamp                      uint `json:"finalTimestamp"`
	FinalBlockNumber                    uint `json:"finalBlockNumber"`

	// Rolling hash of the L1 messages received on L2 so far for the state to be
	// currently finalized. In 0x prefixed Hexstring
	// L1RollingHash stores the last rolling hash found in a rolling hash event
	// during the execution.
	LastFinalizedL1RollingHash string `json:"lastFinalizedL1RollingHash"`
	L1RollingHash              string `json:"l1RollingHash"`

	// Number of L1 messages received on L2 so far for the state to be currently
	// finalized. Messaging Feedback loop messaging number - part of public input
	// L1RollingHashNumber stores the number of the last rolling hash event
	// occuring in the frame of the current aggregation.
	LastFinalizedL1RollingHashMessageNumber uint `json:"lastFinalizedL1RollingHashMessageNumber"`
	L1RollingHashMessageNumber              uint `json:"l1RollingHashMessageNumber"`

	// L2 messages related messages. L2MerkleRoots stores a sequences of Merkle
	// roots containing the hashes of the messages emitted on layer 2.
	L2MerkleRoots   []string `json:"l2MerkleRoots"` // 0x hexstring
	L2MsgTreesDepth uint     `json:"l2MerkleTreesDepth"`

	// Hexstring encoding a bitmap of the block containing “MessageSent” events.
	// events
	L2MessagingBlocksOffsets string `json:"l2MessagingBlocksOffsets"`
}
