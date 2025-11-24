// N.B. ALL AMOUNTS HERE IN GWEI
export interface ValidatorBalance {
  balance: bigint;
  effectiveBalance: bigint;
  publicKey: string;
  validatorIndex: bigint;
}

export interface ValidatorBalanceWithPendingWithdrawal extends ValidatorBalance {
  pendingWithdrawalAmount: bigint;
  withdrawableAmount: bigint;
}
