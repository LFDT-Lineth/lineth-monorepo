const eventERC20 = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "sender",
      type: "address",
    },
    {
      indexed: false,
      internalType: "address",
      name: "recipient",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "token",
      type: "address",
    },
    {
      indexed: true,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
  ],
  name: "BridgingInitiated",
  type: "event",
} as const;

export default eventERC20;
