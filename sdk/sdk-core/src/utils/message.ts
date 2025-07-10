import { OnChainMessageStatus } from "../types/message";

export function formatMessageStatus(status: bigint): OnChainMessageStatus {
  if (status === 2n) {
    return OnChainMessageStatus.CLAIMED;
  }

  if (status === 1n) {
    return OnChainMessageStatus.CLAIMABLE;
  }

  return OnChainMessageStatus.UNKNOWN;
}
