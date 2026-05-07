export interface IBeaconChainStakingClient {
  submitWithdrawalRequestsToFulfilAmount(amountWei: bigint): Promise<void>;
  submitMaxAvailableWithdrawalRequests(): Promise<void>;
}
