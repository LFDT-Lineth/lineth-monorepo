import { GetBlockReturnType } from "viem";
import { fromUnixTime } from "date-fns/fromUnixTime";
import { compareAsc } from "date-fns/compareAsc";
import { subDays } from "date-fns/subDays";

// Transactions with an age > TOO_OLD_IN_DAYS_THRESHOLD will not be shown in the TransactionHistory
const TOO_OLD_IN_DAYS_THRESHOLD = 90;

export function isBlockTooOld(block: GetBlockReturnType): boolean {
  const currentTimestamp = new Date();
  return (
    compareAsc(
      fromUnixTime(Number(block.timestamp.toString())),
      subDays(currentTimestamp, TOO_OLD_IN_DAYS_THRESHOLD),
    ) === -1
  );
}
