// https://ethereum.github.io/beacon-APIs/

export interface IBeaconNodeAPIClient {
  getPendingPartialWithdrawals(): Promise<PendingPartialWithdrawal[] | undefined>;
  getPendingDeposits(): Promise<PendingDeposit[] | undefined>;
  getCurrentEpoch(): Promise<number | undefined>;
}

export interface BeaconApiResponse {
  execution_optimistic: boolean;
  finalized: boolean;
}

export interface PendingPartialWithdrawalResponse extends BeaconApiResponse {
  data: PendingPartialWithdrawal[];
}

export interface PendingPartialWithdrawal {
  validator_index: number;
  amount: bigint;
  withdrawable_epoch: number;
}

export interface PendingDepositResponse extends BeaconApiResponse {
  data: PendingDeposit[];
}

export interface PendingDeposit {
  pubkey: string;
  withdrawal_credentials: string;
  amount: number;
  signature: string;
  slot: number;
}

export interface RawPendingPartialWithdrawal {
  validator_index: string;
  amount: string;
  withdrawable_epoch: string;
}

export interface RawPendingDeposit {
  pubkey: string;
  withdrawal_credentials: string;
  amount: string;
  signature: string;
  slot: string;
}

export interface BeaconHeadMessage {
  slot: string;
  proposer_index: string;
  parent_root: string;
  state_root: string;
  body_root: string;
}

export interface BeaconHeadHeader {
  message: BeaconHeadMessage;
  signature: string;
}

export interface BeaconHeadData {
  root: string;
  canonical: boolean;
  header: BeaconHeadHeader;
}

export interface BeaconHeadResponse extends BeaconApiResponse {
  data: BeaconHeadData;
}
