export const StakingVaultABI = [
  {
    inputs: [{ internalType: "uint256", name: "_numberOfKeys", type: "uint256" }],
    name: "calculateValidatorWithdrawalFee",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;
