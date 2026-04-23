import { expect } from "@jest/globals";

import { awaitUntil } from "../utils/wait";

import type { GetTransactionReceiptErrorType, Hash, PublicClient } from "viem";

const BLOCKS_TO_WAIT = 5n;

/**
 * Asserts that a transaction was blocked by the sequencer's deny list.
 *
 * Handles two sequencer behaviours:
 * 1. Synchronous RPC rejection — `sendTransaction` rejects with a "blocked" error.
 * 2. Async pool discard — `sendTransaction` returns a hash that is never mined.
 *    Waits for BLOCKS_TO_WAIT blocks to be produced before checking, so a tx that
 *    is merely pending is not mistaken for a blocked one.
 */
export async function expectBlockedTransaction(
  client: PublicClient,
  sendTransactionPromise: Promise<Hash>,
): Promise<void> {
  let hash: Hash;

  try {
    hash = await sendTransactionPromise;
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    expect(message.toLowerCase()).toContain("blocked");
    return;
  }

  // Transaction was accepted into the pool — wait for enough blocks to pass so
  // that any pending-but-not-blocked tx would have been mined, then verify no receipt.
  const targetBlock = (await client.getBlockNumber()) + BLOCKS_TO_WAIT;
  await awaitUntil(
    () => client.getBlockNumber(),
    (current) => current >= targetBlock,
  );

  try {
    await client.getTransactionReceipt({ hash });
    throw new Error(`expectBlockedTransaction: transaction ${hash} was mined but should have been blocked`);
  } catch (error) {
    if (error instanceof Error && error.message.includes("expectBlockedTransaction")) {
      throw error;
    }

    const e = error as GetTransactionReceiptErrorType;
    if (e.name !== "TransactionReceiptNotFoundError") {
      throw error;
    }
  }
}
