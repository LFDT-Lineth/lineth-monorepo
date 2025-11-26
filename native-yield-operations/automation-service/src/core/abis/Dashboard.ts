export const DashboardABI = [
  {
    anonymous: false,
    inputs: [
      { indexed: true, internalType: "address", name: "sender", type: "address" },
      { indexed: false, internalType: "uint256", name: "fee", type: "uint256" },
    ],
    name: "FeeDisbursed",
    type: "event",
  },
  {
    inputs: [],
    name: "obligations",
    outputs: [
      { internalType: "uint256", name: "sharesToBurn", type: "uint256" },
      { internalType: "uint256", name: "feesToSettle", type: "uint256" },
    ],
    stateMutability: "view",
    type: "function",
  },
] as const;
