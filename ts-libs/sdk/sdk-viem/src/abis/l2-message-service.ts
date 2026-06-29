/**
 * ABI fragments specific to the L2 `L2MessageService` (Linea side).
 *
 * The `as const` assertions are required for viem to infer argument and return types from the ABI.
 */

// L2-only: L1 claiming goes through `claimMessageWithProof` on `LineaRollup`.
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
