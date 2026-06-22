export const StakingVaultErrorsABI = [
  { inputs: [], name: "AlreadyOssified", type: "error" },
  { inputs: [], name: "BeaconChainDepositsAlreadyPaused", type: "error" },
  { inputs: [], name: "BeaconChainDepositsAlreadyResumed", type: "error" },
  { inputs: [], name: "BeaconChainDepositsOnPause", type: "error" },
  { inputs: [], name: "EthCollectionNotAllowed", type: "error" },
  {
    inputs: [
      { internalType: "uint256", name: "_balance", type: "uint256" },
      { internalType: "uint256", name: "_required", type: "uint256" },
    ],
    name: "InsufficientBalance",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "_staged", type: "uint256" },
      { internalType: "uint256", name: "_requested", type: "uint256" },
    ],
    name: "InsufficientStaged",
    type: "error",
  },
  {
    inputs: [
      { internalType: "uint256", name: "_passed", type: "uint256" },
      { internalType: "uint256", name: "_required", type: "uint256" },
    ],
    name: "InsufficientValidatorWithdrawalFee",
    type: "error",
  },
  { inputs: [], name: "InvalidInitialization", type: "error" },
  { inputs: [], name: "InvalidPubkeysLength", type: "error" },
  { inputs: [], name: "MalformedPubkeysArray", type: "error" },
  {
    inputs: [
      { internalType: "uint256", name: "keysCount", type: "uint256" },
      { internalType: "uint256", name: "amountsCount", type: "uint256" },
    ],
    name: "MismatchedArrayLengths",
    type: "error",
  },
  { inputs: [], name: "NewDepositorSameAsPrevious", type: "error" },
  { inputs: [], name: "NoWithdrawalRequests", type: "error" },
  { inputs: [], name: "NotInitializing", type: "error" },
  {
    inputs: [{ internalType: "address", name: "owner", type: "address" }],
    name: "OwnableInvalidOwner",
    type: "error",
  },
  {
    inputs: [{ internalType: "address", name: "account", type: "address" }],
    name: "OwnableUnauthorizedAccount",
    type: "error",
  },
  { inputs: [], name: "RenouncementNotAllowed", type: "error" },
  {
    inputs: [{ internalType: "address", name: "token", type: "address" }],
    name: "SafeERC20FailedOperation",
    type: "error",
  },
  { inputs: [], name: "SenderNotDepositor", type: "error" },
  { inputs: [], name: "SenderNotNodeOperator", type: "error" },
  {
    inputs: [
      { internalType: "address", name: "recipient", type: "address" },
      { internalType: "uint256", name: "amount", type: "uint256" },
    ],
    name: "TransferFailed",
    type: "error",
  },
  { inputs: [], name: "WithdrawalFeeInvalidData", type: "error" },
  { inputs: [], name: "WithdrawalFeeReadFailed", type: "error" },
  {
    inputs: [{ internalType: "bytes", name: "callData", type: "bytes" }],
    name: "WithdrawalRequestAdditionFailed",
    type: "error",
  },
  {
    inputs: [{ internalType: "string", name: "name", type: "string" }],
    name: "ZeroArgument",
    type: "error",
  },
] as const;
