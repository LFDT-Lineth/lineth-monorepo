// https://ethereum.github.io/beacon-APIs/

export interface IBeaconNodeAPIClient {
  getPendingPartialWithdrawals(): Promise<PendingPartialWithdrawal[] | undefined>;
}

export interface PendingPartialWithdrawalResponse {
  execution_optimistic: boolean;
  finalized: boolean;
  data: PendingPartialWithdrawal[];
}

export interface PendingPartialWithdrawal {
  validator_index: number;
  amount: bigint;
  withdrawable_epoch: number;
}
