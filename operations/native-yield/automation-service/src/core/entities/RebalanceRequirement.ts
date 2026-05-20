export interface RebalanceRequirement {
  rebalanceDirection: RebalanceDirection;
  rebalanceAmount: bigint;
}

export enum RebalanceDirection {
  NONE = "NONE",
  STAKE = "STAKE", // Reserve excess
  UNSTAKE = "UNSTAKE", // Reserve deficit
}
