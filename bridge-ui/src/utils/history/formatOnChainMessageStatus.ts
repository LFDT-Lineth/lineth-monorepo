import { OnChainMessageStatus } from "@consensys/linea-sdk-viem";

import { TransactionStatus } from "@/types";

export function formatOnChainMessageStatus(status: OnChainMessageStatus): TransactionStatus {
  switch (status) {
    case OnChainMessageStatus.UNKNOWN:
      return TransactionStatus.PENDING;
    case OnChainMessageStatus.CLAIMABLE:
      return TransactionStatus.READY_TO_CLAIM;
    case OnChainMessageStatus.CLAIMED:
      return TransactionStatus.COMPLETED;
    default:
      return TransactionStatus.PENDING;
  }
}
