/**
 * ABI fragments for the `TokenBridge` contract (deployed on both L1 and L2).
 *
 * The `as const` assertions are required for viem to infer argument and return types from the ABI.
 */

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
