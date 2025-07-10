import { MESSAGE_ANCHORED_STATUS, MESSAGE_CLAIMED_STATUS } from "../constants";
import { OnChainMessageStatus } from "../enums";

/**
 * Converts a numeric message status code into an enumerated value representing the message's status on the blockchain.
 *
 * This function maps specific numeric status codes to their corresponding enumerated values, such as `CLAIMED` or `CLAIMABLE`, based on predefined constants. If the status code does not match any known value, it defaults to `UNKNOWN`.
 *
 * @param {bigint} status - The numeric status code of the message as retrieved from the blockchain.
 * @returns {OnChainMessageStatus} The enumerated value representing the message's status, such as `CLAIMED`, `CLAIMABLE`, or `UNKNOWN`.
 */
export function formatMessageStatus(status: bigint): OnChainMessageStatus {
  if (status === BigInt(MESSAGE_CLAIMED_STATUS)) {
    return OnChainMessageStatus.CLAIMED;
  }

  if (status === BigInt(MESSAGE_ANCHORED_STATUS)) {
    return OnChainMessageStatus.CLAIMABLE;
  }

  return OnChainMessageStatus.UNKNOWN;
}
