import { Address } from "viem";

export interface WithdrawalEvent {
  reserveIncrementAmount: bigint;
  yieldProvider: Address;
}
