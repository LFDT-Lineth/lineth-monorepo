export const VaultHubABI = [
  {
    anonymous: false,
    inputs: [
      { indexed: true, internalType: "address", name: "vault", type: "address" },
      { indexed: false, internalType: "uint256", name: "transferred", type: "uint256" },
      { indexed: false, internalType: "uint256", name: "cumulativeLidoFees", type: "uint256" },
      { indexed: false, internalType: "uint256", name: "settledLidoFees", type: "uint256" },
    ],
    name: "LidoFeesSettled",
    type: "event",
  },
  {
    anonymous: false,
    inputs: [
      { indexed: true, internalType: "address", name: "vault", type: "address" },
      { indexed: false, internalType: "uint256", name: "sharesBurned", type: "uint256" },
      { indexed: false, internalType: "uint256", name: "etherWithdrawn", type: "uint256" },
    ],
    name: "VaultRebalanced",
    type: "event",
  },
] as const;
