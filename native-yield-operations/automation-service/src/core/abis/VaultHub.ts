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
  {
    inputs: [{ internalType: "address", name: "_vault", type: "address" }],
    name: "settleableLidoFeesValue",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
  {
    inputs: [{ internalType: "address", name: "_vault", type: "address" }],
    name: "latestReport",
    outputs: [
      {
        components: [
          { internalType: "uint104", name: "totalValue", type: "uint104" },
          { internalType: "int104", name: "inOutDelta", type: "int104" },
          { internalType: "uint48", name: "timestamp", type: "uint48" },
        ],
        internalType: "struct VaultHub.Report",
        name: "",
        type: "tuple",
      },
    ],
    stateMutability: "view",
    type: "function",
  },
  {
    inputs: [{ internalType: "address", name: "_vault", type: "address" }],
    name: "isReportFresh",
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "view",
    type: "function",
  },
  {
    inputs: [{ internalType: "address", name: "_vault", type: "address" }],
    name: "isVaultConnected",
    outputs: [{ internalType: "bool", name: "", type: "bool" }],
    stateMutability: "view",
    type: "function",
  },
] as const;
