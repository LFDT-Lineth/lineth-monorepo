import { fromUnixTime } from "date-fns/fromUnixTime";
import { compareAsc } from "date-fns/compareAsc";
import { subDays } from "date-fns/subDays";
import { MESSAGE_TOO_OLD_THRESHOLD_DAYS } from "@/constants";

export function isTimestampTooOld(timestamp: bigint): boolean {
  const currentTimestamp = new Date();
  return (
    compareAsc(
      fromUnixTime(Number(timestamp.toString())),
      subDays(currentTimestamp, MESSAGE_TOO_OLD_THRESHOLD_DAYS),
    ) === -1
  );
}
