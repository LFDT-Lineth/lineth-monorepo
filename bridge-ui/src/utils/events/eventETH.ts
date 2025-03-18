const eventETH = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "_from",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "_to",
      type: "address",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_fee",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_value",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_nonce",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "bytes",
      name: "_calldata",
      type: "bytes",
    },
    {
      indexed: true,
      internalType: "bytes32",
      name: "_messageHash",
      type: "bytes32",
    },
  ],
  name: "MessageSent",
  type: "event",
} as const;

export default eventETH;
