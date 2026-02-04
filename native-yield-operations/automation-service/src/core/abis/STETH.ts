export const STETHABI = [
  {
    constant: true,
    inputs: [{ name: "_sharesAmount", type: "uint256" }],
    name: "getPooledEthBySharesRoundUp",
    outputs: [{ name: "", type: "uint256" }],
    payable: false,
    stateMutability: "view",
    type: "function",
  },
] as const;
