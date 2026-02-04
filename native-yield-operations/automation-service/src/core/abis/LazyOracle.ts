export const LazyOracleABI = [
  {
    inputs: [
      {
        internalType: "address",
        name: "_vault",
        type: "address",
      },
      {
        internalType: "uint256",
        name: "_totalValue",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_cumulativeLidoFees",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_liabilityShares",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_maxLiabilityShares",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_slashingReserve",
        type: "uint256",
      },
      {
        internalType: "bytes32[]",
        name: "_proof",
        type: "bytes32[]",
      },
    ],
    name: "updateVaultData",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        internalType: "uint256",
        name: "timestamp",
        type: "uint256",
      },
      {
        indexed: true,
        internalType: "uint256",
        name: "refSlot",
        type: "uint256",
      },
      {
        indexed: true,
        internalType: "bytes32",
        name: "root",
        type: "bytes32",
      },
      {
        indexed: false,
        internalType: "string",
        name: "cid",
        type: "string",
      },
    ],
    name: "VaultsReportDataUpdated",
    type: "event",
  },
  {
    inputs: [],
    name: "latestReportData",
    outputs: [
      { internalType: "uint256", name: "timestamp", type: "uint256" },
      { internalType: "uint256", name: "refSlot", type: "uint256" },
      { internalType: "bytes32", name: "treeRoot", type: "bytes32" },
      { internalType: "string", name: "reportCid", type: "string" },
    ],
    stateMutability: "view",
    type: "function",
  },
] as const;
