import { Address, Hex } from "viem";

export interface WithdrawalRequests {
  pubkeys: Hex[];
  amountsGwei: bigint[];
}

export interface LidoStakingVaultWithdrawalParams extends WithdrawalRequests {
  refundRecipient: Address;
}
