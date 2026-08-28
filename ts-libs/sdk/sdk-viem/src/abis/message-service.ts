/**
 * ABI fragments shared by the message-service base of both the L1 `LinethRollup` and the L2
 * `L2MessageService` (`MessageServiceBase`).
 *
 * The `as const` assertions are required for viem to infer argument and return types from the ABI.
 */

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
