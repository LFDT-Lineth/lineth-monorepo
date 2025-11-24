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
] as const;
