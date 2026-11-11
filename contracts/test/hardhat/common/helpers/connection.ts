import { network } from "hardhat";
import { after, before } from "node:test";

import type { NetworkConnection } from "hardhat/types/network";

let isolatedConnection: NetworkConnection | undefined;

/**
 * The network connection the shared test helpers use: the shared default-network connection, or the enclosing
 * suite's fresh chain when it called `useFreshChain()`. Call at runtime (inside fixtures, hooks or tests).
 */
export async function getConnection(): Promise<NetworkConnection> {
  return isolatedConnection ?? network.getOrCreate();
}

/**
 * Runs the enclosing suite on a brand-new simulated chain, the replacement for Hardhat 2's `hardhat_reset`
 * (unsupported in Hardhat 3). Only for suites whose assertions depend on a pristine chain (fixed deployer nonces,
 * addresses, block numbers): every spec file otherwise shares one chain, as under Hardhat 2.
 *
 * Call inside the suite's top-level `describe`, and obtain the connection in that suite with `getConnection()`.
 * Safe because the Hardhat node:test runner executes suites sequentially (`concurrency: false`).
 */
export function useFreshChain(): void {
  before(async () => {
    isolatedConnection = await network.create();
  });
  after(async () => {
    const connection = isolatedConnection;
    isolatedConnection = undefined;
    await connection?.close();
  });
}
