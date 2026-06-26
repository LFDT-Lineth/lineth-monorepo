/**
 * Minimal, hand-maintained ABI fragments for the protocol contracts the SDK interacts with.
 *
 * Each constant is intentionally scoped to a single function or event (rather than a full contract
 * ABI) so call sites stay tree-shakeable and the encoded selectors are easy to audit. They are kept
 * internal to `sdk-viem` (not re-exported from `src/index.ts`) and are therefore not part of the
 * public package API.
 *
 * The `as const` assertions are required for viem to infer argument and return types from the ABI.
 */

// ---------------------------------------------------------------------------
// Message service (shared between L1 `LineaRollup` and L2 `L2MessageService`)
// ---------------------------------------------------------------------------

export const MESSAGE_SENT_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [
      { indexed: true, internalType: "address", name: "_from", type: "address" },
      { indexed: true, internalType: "address", name: "_to", type: "address" },
      { indexed: false, internalType: "uint256", name: "_fee", type: "uint256" },
      { indexed: false, internalType: "uint256", name: "_value", type: "uint256" },
      { indexed: false, internalType: "uint256", name: "_nonce", type: "uint256" },
      { indexed: false, internalType: "bytes", name: "_calldata", type: "bytes" },
      { indexed: true, internalType: "bytes32", name: "_messageHash", type: "bytes32" },
    ],
    name: "MessageSent",
    type: "event",
  },
] as const;

export const SEND_MESSAGE_ABI = [
  {
    inputs: [
      { internalType: "address", name: "_to", type: "address" },
      { internalType: "uint256", name: "_fee", type: "uint256" },
      { internalType: "bytes", name: "_calldata", type: "bytes" },
    ],
    name: "sendMessage",
    outputs: [],
    stateMutability: "payable",
    type: "function",
  },
] as const;

// ---------------------------------------------------------------------------
// L2 `L2MessageService` (Linea side)
// ---------------------------------------------------------------------------

export const CLAIM_MESSAGE_ABI = [
  {
    inputs: [
      { internalType: "address", name: "_from", type: "address" },
      { internalType: "address", name: "_to", type: "address" },
      { internalType: "uint256", name: "_fee", type: "uint256" },
      { internalType: "uint256", name: "_value", type: "uint256" },
      { internalType: "address payable", name: "_feeRecipient", type: "address" },
      { internalType: "bytes", name: "_calldata", type: "bytes" },
      { internalType: "uint256", name: "_nonce", type: "uint256" },
    ],
    name: "claimMessage",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

export const MINIMUM_FEE_IN_WEI_ABI = [
  {
    inputs: [],
    name: "minimumFeeInWei",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

export const INBOX_L1_L2_MESSAGE_STATUS_ABI = [
  {
    inputs: [{ internalType: "bytes32", name: "messageHash", type: "bytes32" }],
    name: "inboxL1L2MessageStatus",
    outputs: [{ internalType: "uint256", name: "messageStatus", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

// ---------------------------------------------------------------------------
// L1 `LineaRollup`
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Token bridge
// ---------------------------------------------------------------------------

export const BRIDGE_TOKEN_ABI = [
  {
    inputs: [
      { internalType: "address", name: "_token", type: "address" },
      { internalType: "uint256", name: "_amount", type: "uint256" },
      { internalType: "address", name: "_recipient", type: "address" },
    ],
    name: "bridgeToken",
    outputs: [],
    stateMutability: "payable",
    type: "function",
  },
] as const;

export const COMPLETE_BRIDGING_ABI = [
  {
    inputs: [
      { internalType: "address", name: "_nativeToken", type: "address" },
      { internalType: "uint256", name: "_amount", type: "uint256" },
      { internalType: "address", name: "_recipient", type: "address" },
      { internalType: "uint256", name: "_chainId", type: "uint256" },
      { internalType: "bytes", name: "_tokenMetadata", type: "bytes" },
    ],
    name: "completeBridging",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

export const BRIDGED_TO_NATIVE_TOKEN_ABI = [
  {
    inputs: [{ internalType: "address", name: "bridged", type: "address" }],
    name: "bridgedToNativeToken",
    outputs: [{ internalType: "address", name: "native", type: "address" }],
    stateMutability: "view",
    type: "function",
  },
] as const;
