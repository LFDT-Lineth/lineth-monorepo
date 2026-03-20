import { test } from "@playwright/test";

import { MESSAGE_TOO_OLD_THRESHOLD_DAYS } from "@/constants/message";
import { BridgeTransaction } from "@/types";

import { restoreFromTransactionCache } from "./restoreFromTransactionCache";

const { expect, describe } = test;

const ONE_DAY_MS = 24 * 60 * 60 * 1000;

function stubCompletedTx(timestampSeconds: bigint): BridgeTransaction {
  return { timestamp: timestampSeconds } as BridgeTransaction;
}

describe("restoreFromTransactionCache", () => {
  test("returns false when there is no cached transaction", () => {
    const map = new Map<string, BridgeTransaction>();
    const actions = {
      getCompleteTx: () => undefined,
      setCompleteTx: () => {},
      deleteCompleteTx: () => {},
    };

    const result = restoreFromTransactionCache(actions, 1, "0xhash", map, "0xhash");

    expect(result).toBe(false);
    expect(map.size).toBe(0);
  });

  test("returns true and copies cached tx into the map when cache is still valid", () => {
    const recentTs = BigInt(Math.floor((Date.now() - ONE_DAY_MS) / 1000));
    const cached = stubCompletedTx(recentTs);
    const map = new Map<string, BridgeTransaction>();
    const actions = {
      getCompleteTx: () => cached,
      setCompleteTx: () => {},
      deleteCompleteTx: () => {},
    };

    const result = restoreFromTransactionCache(actions, 1, "0xabc", map, "map-key");

    expect(result).toBe(true);
    expect(map.get("map-key")).toBe(cached);
  });

  test("deletes stale cache, returns false, and does not populate the map", () => {
    const staleTs = BigInt(Math.floor((Date.now() - (MESSAGE_TOO_OLD_THRESHOLD_DAYS + 1) * ONE_DAY_MS) / 1000));
    const cached = stubCompletedTx(staleTs);
    const map = new Map<string, BridgeTransaction>();
    let deletedKey: string | undefined;
    const actions = {
      getCompleteTx: () => cached,
      setCompleteTx: () => {},
      deleteCompleteTx: (key: string) => {
        deletedKey = key;
      },
    };

    const result = restoreFromTransactionCache(actions, 1, "0xabc", map, "map-key");

    expect(result).toBe(false);
    expect(deletedKey).toBe("1-0xabc");
    expect(map.size).toBe(0);
  });
});
