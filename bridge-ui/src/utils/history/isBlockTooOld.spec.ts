import { test } from "@playwright/test";
import { GetBlockReturnType } from "viem";

import { MESSAGE_TOO_OLD_THRESHOLD_DAYS } from "@/constants/message";

import { isBlockTooOld } from "./isBlockTooOld";

const { expect, describe } = test;

describe("isBlockTooOld", () => {
  const ONE_DAY_MS = 24 * 60 * 60 * 1000;

  test("should return false for current timestamp", () => {
    const currentTimestamp = Math.floor(Date.now() / 1000);
    const mockBlock: GetBlockReturnType = {
      timestamp: BigInt(currentTimestamp),
    } as GetBlockReturnType;

    const result = isBlockTooOld(mockBlock);
    expect(result).toBe(false);
  });

  test("should return false for a recent block (1 day old)", () => {
    const recentTimestamp = Math.floor((Date.now() - ONE_DAY_MS) / 1000);
    const mockBlock: GetBlockReturnType = {
      timestamp: BigInt(recentTimestamp),
    } as GetBlockReturnType;

    const result = isBlockTooOld(mockBlock);
    expect(result).toBe(false);
  });

  test("should return false for a block just under 90 days old", () => {
    // Use 89.5 days to ensure we're clearly within the threshold
    const justUnder90DaysTimestamp = Math.floor(
      (Date.now() - (MESSAGE_TOO_OLD_THRESHOLD_DAYS - 0.5) * ONE_DAY_MS) / 1000,
    );
    const mockBlock: GetBlockReturnType = {
      timestamp: BigInt(justUnder90DaysTimestamp),
    } as GetBlockReturnType;

    const result = isBlockTooOld(mockBlock);
    expect(result).toBe(false);
  });

  test("should return true for a block older than 90 days", () => {
    const olderThanNinetyDaysTimestamp = Math.floor(
      (Date.now() - (MESSAGE_TOO_OLD_THRESHOLD_DAYS + 1) * ONE_DAY_MS) / 1000,
    );
    const mockBlock: GetBlockReturnType = {
      timestamp: BigInt(olderThanNinetyDaysTimestamp),
    } as GetBlockReturnType;

    const result = isBlockTooOld(mockBlock);
    expect(result).toBe(true);
  });
});
