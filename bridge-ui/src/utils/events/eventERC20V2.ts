const eventERC20V2 = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "sender",
      type: "address",
    },
    {
      indexed: true,
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
      indexed: false,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
  ],
  name: "BridgingInitiatedV2",
  type: "event",
} as const;

export default eventERC20V2;
