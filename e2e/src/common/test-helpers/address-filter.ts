import { type Address, type Hex, type PublicClient, type WalletClient } from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { AddressFilterAbi, LineaRollupV8Abi } from "../../generated";

// Well-known Hardhat account #3 — pre-funded on the local L1 genesis and granted
// DEFAULT_ADMIN_ROLE on the AddressFilter contract. Local-only test infrastructure.
const L1_SECURITY_COUNCIL_PRIVATE_KEY: Hex = "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6";

let onChainFilterOperationQueue: Promise<void> = Promise.resolve();

function withOnChainFilterLock<T>(operation: () => Promise<T>): Promise<T> {
  const result = onChainFilterOperationQueue.then(operation, operation);
  onChainFilterOperationQueue = result.then(
    () => undefined,
    () => undefined,
  );
  return result;
}

async function setOnChainFilteredStatus(
  l1PublicClient: PublicClient,
  adminWalletClient: WalletClient,
  addressFilterAddress: Address,
  addresses: readonly Address[],
  isFiltered: boolean,
): Promise<void> {
  if (!adminWalletClient.account) {
    throw new Error("setOnChainFilteredStatus: adminWalletClient must have an account");
  }
  const hash = await adminWalletClient.writeContract({
    address: addressFilterAddress,
    abi: AddressFilterAbi,
    functionName: "setFilteredStatus",
    args: [[...addresses], isFiltered],
    account: adminWalletClient.account,
    chain: adminWalletClient.chain,
  });
  const receipt = await l1PublicClient.waitForTransactionReceipt({ hash, timeout: 30_000 });
  if (receipt.status !== "success") {
    throw new Error(`setFilteredStatus(${isFiltered}) reverted. txHash=${hash}`);
  }
}

/**
 * Adds the given addresses to the on-chain `AddressFilter` contract before running `run()`,
 * and removes them afterwards. The on-chain filter must contain any address that the rollup's
 * finalization data references via `filteredAddresses`, otherwise finalization reverts with
 * `AddressIsNotFiltered`.
 *
 * Reads `addressFilter()` from the LineaRollup contract to discover the filter address.
 */
export async function withOnChainFilteredAddresses(
  l1PublicClient: PublicClient,
  adminWalletClient: WalletClient,
  lineaRollupAddress: Address,
  addresses: readonly Address[],
  run: () => Promise<void>,
): Promise<void> {
  if (addresses.length === 0) {
    await run();
    return;
  }

  const addressFilterAddress = (await l1PublicClient.readContract({
    address: lineaRollupAddress,
    abi: LineaRollupV8Abi,
    functionName: "addressFilter",
  })) as Address;

  // Lock only around the on-chain admin txs to serialize nonces on the security-council
  // account; the wait is lock-free so concurrent tests can keep different addresses in
  // the AddressFilter simultaneously.
  await withOnChainFilterLock(() =>
    setOnChainFilteredStatus(l1PublicClient, adminWalletClient, addressFilterAddress, addresses, true),
  );
  try {
    await run();
  } finally {
    await withOnChainFilterLock(() =>
      setOnChainFilteredStatus(l1PublicClient, adminWalletClient, addressFilterAddress, addresses, false),
    );
  }
}

/**
 * Returns a viem `PrivateKeyAccount` for the L1 security council, which holds
 * `DEFAULT_ADMIN_ROLE` on the on-chain `AddressFilter`. Local-only.
 */
export function getL1SecurityCouncilAccount() {
  return privateKeyToAccount(L1_SECURITY_COUNCIL_PRIVATE_KEY);
}
