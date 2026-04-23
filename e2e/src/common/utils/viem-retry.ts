import { BlockNotFoundError, type Client, type PublicActions } from "viem";
import { estimateFeesPerGas } from "viem/actions";

import { awaitUntil } from "./wait";

/**
 * Retries a viem action that throws `BlockNotFoundError`. This error surfaces
 * when the RPC returns HTTP 200 with `{"result": null}` for block lookups
 * (e.g. Besu PoS around fork-choice updates). viem's transport-level retry
 * doesn't cover this case because it happens at the action layer.
 *
 * Other errors are propagated immediately.
 */
export async function withRetryOnBlockNotFound<T>(
  fn: () => Promise<T>,
  options: { pollingIntervalMs?: number; timeoutMs?: number } = {},
): Promise<T> {
  const { pollingIntervalMs = 250, timeoutMs = 10_000 } = options;

  const sentinel: unique symbol = Symbol("block-not-found");
  type Result = T | typeof sentinel;

  const result = await awaitUntil<Result>(
    async () => {
      try {
        return await fn();
      } catch (error) {
        if (error instanceof BlockNotFoundError) return sentinel;
        throw error;
      }
    },
    (value): value is T => value !== sentinel,
    { pollingIntervalMs, timeoutMs },
  );

  return result as T;
}

type EstimateFeesPerGasAction = PublicActions["estimateFeesPerGas"];

/**
 * viem client extension that wraps `estimateFeesPerGas` with
 * `withRetryOnBlockNotFound`. Apply via `.extend(createBlockNotFoundRetryExtension())`
 * so every call site on the extended client gets retry behaviour automatically.
 */
export function createBlockNotFoundRetryExtension() {
  return (client: Client): { estimateFeesPerGas: EstimateFeesPerGasAction } => ({
    estimateFeesPerGas: ((args) =>
      withRetryOnBlockNotFound(() => estimateFeesPerGas(client, args))) as EstimateFeesPerGasAction,
  });
}
