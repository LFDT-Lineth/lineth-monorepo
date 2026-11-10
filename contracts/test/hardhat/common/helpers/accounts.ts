import { type Address, getAddress } from "viem";

import { getConnection } from "./connection.js";

import type { WalletClient } from "@nomicfoundation/hardhat-viem/types";

/**
 * A test actor: a hardhat-viem wallet client that also carries its checksummed address. Send as this actor by
 * passing its account to the call, e.g. `contract.write.fn([...], { account: admin.account })` (works for
 * `simulate`, `estimateGas` and `read` too).
 */
export type TestAccount = WalletClient & { address: Address };

/**
 * Returns the default network's wallet clients as test accounts, in the same order as ethers' `getSigners()`.
 * Must be called at runtime (inside a fixture, hook or test), never at module top level.
 */
export async function getTestAccounts(): Promise<TestAccount[]> {
  const { viem } = await getConnection();
  const wallets = await viem.getWalletClients();
  // Checksummed, like ethers' `signer.address` (hardhat-viem reports JSON-RPC account addresses in lowercase).
  return wallets.map((wallet) => Object.assign(wallet, { address: getAddress(wallet.account.address) }));
}
