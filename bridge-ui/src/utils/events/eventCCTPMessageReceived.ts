const eventCCTPMessageReceived = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "caller", type: "address" },
    { indexed: false, internalType: "uint32", name: "sourceDomain", type: "uint32" },
    { indexed: true, internalType: "bytes32", name: "nonce", type: "bytes32" },
    { indexed: false, internalType: "bytes32", name: "sender", type: "bytes32" },
    { indexed: true, internalType: "uint32", name: "finalityThresholdExecuted", type: "uint32" },
    { indexed: false, internalType: "bytes", name: "messageBody", type: "bytes" },
  ],
  name: "MessageReceived",
  type: "event",
} as const;

export default eventCCTPMessageReceived;
