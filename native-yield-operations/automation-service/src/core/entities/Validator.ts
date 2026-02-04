// N.B. ALL AMOUNTS HERE IN GWEI
export interface ValidatorBalance {
  balance: bigint;
  effectiveBalance: bigint;
  publicKey: string;
  validatorIndex: bigint;
  activationEpoch: number;
}

export interface ExitingValidator {
  balance: bigint;
  effectiveBalance: bigint;
  publicKey: string;
  validatorIndex: bigint;
  exitEpoch: number;
  exitDate: Date;
  slashed: boolean;
}

export interface ExitedValidator {
  balance: bigint; // in gwei
  publicKey: string;
  validatorIndex: bigint;
  slashed: boolean;
  withdrawableEpoch: number;
}

export interface ValidatorBalanceWithPendingWithdrawal extends ValidatorBalance {
  pendingWithdrawalAmount: bigint;
  withdrawableAmount: bigint;
}

/**
 * Aggregated pending withdrawal result with validator public key.
 */
export interface AggregatedPendingWithdrawal {
  validator_index: number;
  withdrawable_epoch: number;
  amount: bigint;
  pubkey: string;
}
