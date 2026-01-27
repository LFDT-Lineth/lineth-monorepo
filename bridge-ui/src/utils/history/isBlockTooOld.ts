import { compareAsc } from "date-fns/compareAsc";
import { fromUnixTime } from "date-fns/fromUnixTime";
import { subDays } from "date-fns/subDays";
import { GetBlockReturnType } from "viem";

import { MESSAGE_TOO_OLD_THRESHOLD_DAYS } from "@/constants";

export function isBlockTooOld(block: GetBlockReturnType): boolean {
  const currentTimestamp = new Date();
  return (
    compareAsc(
      fromUnixTime(Number(block.timestamp.toString())),
      subDays(currentTimestamp, MESSAGE_TOO_OLD_THRESHOLD_DAYS),
    ) === -1
  );
}
