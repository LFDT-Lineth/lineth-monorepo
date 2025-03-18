const eventUSDC = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "burnToken",
      type: "address",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
    {
      indexed: true,
      internalType: "address",
      name: "depositor",
      type: "address",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "mintRecipient",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "uint32",
      name: "destinationDomain",
      type: "uint32",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "destinationTokenMessenger",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "destinationCaller",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "maxFee",
      type: "uint256",
    },
    {
      indexed: true,
      internalType: "uint32",
      name: "minFinalityThreshold",
      type: "uint32",
    },
    {
      indexed: false,
      internalType: "bytes",
      name: "hookData",
      type: "bytes",
    },
  ],
  name: "DepositForBurn",
  type: "event",
} as const;

export default eventUSDC;
