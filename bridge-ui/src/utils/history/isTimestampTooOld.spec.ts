import { test } from "@playwright/test";

import { MESSAGE_TOO_OLD_THRESHOLD_DAYS } from "@/constants/message";

import { isTimestampTooOld } from "./isTimestampTooOld";

const { expect, describe } = test;

describe("isTimestampTooOld", () => {
  const ONE_DAY_MS = 24 * 60 * 60 * 1000;

  test("should return false for current timestamp", () => {
    const currentTimestamp = BigInt(Math.floor(Date.now() / 1000));

    const result = isTimestampTooOld(currentTimestamp);
    expect(result).toBe(false);
  });

  test("should return false for a recent timestamp (1 day old)", () => {
    const recentTimestamp = BigInt(Math.floor((Date.now() - ONE_DAY_MS) / 1000));

    const result = isTimestampTooOld(recentTimestamp);
    expect(result).toBe(false);
  });

  test("should return false for a timestamp just under 90 days old", () => {
    // Use 89.5 days to ensure we're clearly within the threshold
    const justUnder90DaysTimestamp = BigInt(
      Math.floor((Date.now() - (MESSAGE_TOO_OLD_THRESHOLD_DAYS - 0.5) * ONE_DAY_MS) / 1000),
    );

    const result = isTimestampTooOld(justUnder90DaysTimestamp);
    expect(result).toBe(false);
  });

  test("should return true for a timestamp older than 90 days", () => {
    const olderThanNinetyDaysTimestamp = BigInt(
      Math.floor((Date.now() - (MESSAGE_TOO_OLD_THRESHOLD_DAYS + 1) * ONE_DAY_MS) / 1000),
    );

    const result = isTimestampTooOld(olderThanNinetyDaysTimestamp);
    expect(result).toBe(true);
  });
});
