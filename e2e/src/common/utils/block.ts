import { Client, Transport, GetBlockParameters, GetBlockReturnType, Chain, Account } from "viem";
import { getBlockNumber, getBlock } from "viem/actions";

import { awaitUntil } from "./wait";
import { createTestLogger } from "../../config/logger/logger";

const logger = createTestLogger();

export async function pollForBlockNumber(
  client: Client,
  expectedBlockNumber: bigint,
  pollingIntervalMs: number = 500,
  timeoutMs: number = 2 * 60 * 1000,
): Promise<boolean> {
  try {
    await awaitUntil(
      async () => await getBlockNumber(client),
      (a: bigint) => a >= expectedBlockNumber,
      {
        pollingIntervalMs,
        timeoutMs,
      },
    );
    return true;
  } catch {
    return false;
  }
}

export async function getBlockByNumberOrBlockTag<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  params: GetBlockParameters,
): Promise<GetBlockReturnType | null> {
  try {
    const block = await getBlock(client, params);
    return block;
  } catch (error) {
    logger.warn(
      `Failed to get block. params=${JSON.stringify(params)} error=${error instanceof Error ? error.message : String(error)}`,
    );
    return null;
  }
}
