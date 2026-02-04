export const LazyOracleErrorsABI = [
  {
    inputs: [],
    name: "AccessControlBadConfirmation",
    type: "error",
  },
  {
    inputs: [
      { internalType: "address", name: "account", type: "address" },
      { internalType: "bytes32", name: "neededRole", type: "bytes32" },
    ],
    name: "AccessControlUnauthorizedAccount",
    type: "error",
  },
  {
    inputs: [],
    name: "AdminCannotBeZero",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "feeIncrease", type: "uint256" },
      { internalType: "uint256", name: "maxFeeIncrease", type: "uint256" },
    ],
    name: "CumulativeLidoFeesTooLarge",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "reportingFees", type: "uint256" },
      { internalType: "uint256", name: "previousFees", type: "uint256" },
    ],
    name: "CumulativeLidoFeesTooLow",
    type: "error",
  },
  {
    inputs: [],
    name: "InOutDeltaCacheIsOverwritten",
    type: "error",
  },
  {
    inputs: [],
    name: "InvalidInitialization",
    type: "error",
  },
  {
    inputs: [],
    name: "InvalidMaxLiabilityShares",
    type: "error",
  },
  {
    inputs: [],
    name: "InvalidProof",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "feeRate", type: "uint256" },
      { internalType: "uint256", name: "maxFeeRate", type: "uint256" },
    ],
    name: "MaxLidoFeeRatePerSecondTooLarge",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "rewardRatio", type: "uint256" },
      { internalType: "uint256", name: "maxRewardRatio", type: "uint256" },
    ],
    name: "MaxRewardRatioTooLarge",
    type: "error",
  },
  {
    inputs: [],
    name: "NotAuthorized",
    type: "error",
  },
  {
    inputs: [],
    name: "NotInitializing",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "quarantinePeriod", type: "uint256" },
      { internalType: "uint256", name: "maxQuarantinePeriod", type: "uint256" },
    ],
    name: "QuarantinePeriodTooLarge",
    type: "error",
  },
  {
    inputs: [],
    name: "TotalValueTooLarge",
    type: "error",
  },
  {
    inputs: [],
    name: "UnderflowInTotalValueCalculation",
    type: "error",
  },
  {
    inputs: [],
    name: "VaultReportIsFreshEnough",
    type: "error",
  },
] as const;
