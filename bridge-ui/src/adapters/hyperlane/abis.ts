export const HUB_PORTAL_ABI = [
  {
    type: "function",
    stateMutability: "payable",
    name: "transferMLikeToken",
    inputs: [
      { name: "_amount", type: "uint256", internalType: "uint256" },
      { name: "_token", type: "address", internalType: "address" },
      { name: "_destinationChainId", type: "uint256", internalType: "uint256" },
      { name: "_destinationToken", type: "address", internalType: "address" },
      { name: "_recipient", type: "address", internalType: "address" },
      { name: "_refundAddress", type: "address", internalType: "address" },
    ],
    outputs: [],
  },
  {
    type: "function",
    stateMutability: "view",
    name: "quoteTransfer",
    inputs: [
      { name: "_amount", type: "uint256", internalType: "uint256" },
      { name: "_destinationChainId", type: "uint256", internalType: "uint256" },
      { name: "_recipient", type: "address", internalType: "address" },
    ],
    outputs: [{ name: "", type: "uint256", internalType: "uint256" }],
  },
] as const;

export const MAILBOX_ABI = [
  {
    type: "function",
    stateMutability: "view",
    name: "delivered",
    inputs: [{ name: "messageId", type: "bytes32", internalType: "bytes32" }],
    outputs: [{ name: "", type: "bool", internalType: "bool" }],
  },
] as const;
