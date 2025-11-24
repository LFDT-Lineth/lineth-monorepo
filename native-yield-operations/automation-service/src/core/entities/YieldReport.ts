import { Address } from "viem";

export interface YieldReport {
  yieldAmount: bigint;
  outstandingNegativeYield: bigint;
  yieldProvider: Address;
}
