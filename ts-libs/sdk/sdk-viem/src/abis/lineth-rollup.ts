/**
 * ABI fragments specific to the L1 `LinethRollup` (settlement, anchoring and the L1 claim path).
 *
 * The `as const` assertions are required for viem to infer argument and return types from the ABI.
 */

export const CLAIM_MESSAGE_WITH_PROOF_ABI = [
  {
    inputs: [
      {
        components: [
          { internalType: "bytes32[]", name: "proof", type: "bytes32[]" },
          { internalType: "uint256", name: "messageNumber", type: "uint256" },
          { internalType: "uint32", name: "leafIndex", type: "uint32" },
          { internalType: "address", name: "from", type: "address" },
          { internalType: "address", name: "to", type: "address" },
          { internalType: "uint256", name: "fee", type: "uint256" },
          { internalType: "uint256", name: "value", type: "uint256" },
          { internalType: "address payable", name: "feeRecipient", type: "address" },
          { internalType: "bytes32", name: "merkleRoot", type: "bytes32" },
          { internalType: "bytes", name: "data", type: "bytes" },
        ],
        internalType: "struct IL1MessageService.ClaimMessageWithProofParams",
        name: "_params",
        type: "tuple",
      },
    ],
    name: "claimMessageWithProof",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

export const CURRENT_L2_BLOCK_NUMBER_ABI = [
  {
    inputs: [],
    name: "currentL2BlockNumber",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

export const IS_MESSAGE_CLAIMED_ABI = [
  {
    inputs: [{ internalType: "uint256", name: "_messageNumber", type: "uint256" }],
    name: "isMessageClaimed",
    outputs: [{ internalType: "bool", name: "isClaimed", type: "bool" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

export const NEXT_MESSAGE_NUMBER_ABI = [
  {
    inputs: [],
    name: "nextMessageNumber",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

export const L2_MESSAGING_BLOCK_ANCHORED_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [{ indexed: true, internalType: "uint256", name: "l2Block", type: "uint256" }],
    name: "L2MessagingBlockAnchored",
    type: "event",
  },
] as const;

export const L2_MERKLE_ROOT_ADDED_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [
      { indexed: true, internalType: "bytes32", name: "l2MerkleRoot", type: "bytes32" },
      { indexed: true, internalType: "uint256", name: "treeDepth", type: "uint256" },
    ],
    name: "L2MerkleRootAdded",
    type: "event",
  },
] as const;
