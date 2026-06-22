export const LidoStVaultYieldProviderErrorsABI = [
  { inputs: [], name: "ArrayLengthsDoNotMatch", type: "error" },
  { inputs: [], name: "CallerNotProxyAdmin", type: "error" },
  { inputs: [], name: "ContextIsNotYieldManager", type: "error" },
  { inputs: [], name: "MintLSTDisabledDuringOssification", type: "error" },
  { inputs: [], name: "NoEthSent", type: "error" },
  { inputs: [], name: "NoValidatorExitForUnstakePermissionless", type: "error" },
  {
    inputs: [{ internalType: "enum IYieldProvider.OperationType", name: "operationType", type: "uint8" }],
    name: "OperationNotSupportedDuringOssification",
    type: "error",
  },
  {
    inputs: [{ internalType: "enum IYieldProvider.OperationType", name: "operationType", type: "uint8" }],
    name: "OperationNotSupportedDuringStakingPause",
    type: "error",
  },
  { inputs: [], name: "SingleValidatorOnlyForUnstakePermissionless", type: "error" },
  { inputs: [], name: "UnknownYieldProviderVendor", type: "error" },
  { inputs: [], name: "UnpauseStakingForbiddenWhenOssified", type: "error" },
  { inputs: [], name: "ZeroAddressNotAllowed", type: "error" },
  { inputs: [], name: "ZeroHashNotAllowed", type: "error" },
] as const;
