import type { Chain, Hash, PublicClient, Transport, WaitForTransactionReceiptReturnType } from "viem";

/**
 * Waits for `hash` to be mined and throws if it reverted. Unlike a bare `waitForTransactionReceipt`, this has no
 * 180 s timeout (`timeout: 0` disables viem's timer) and does not hand back a `status: "reverted"` receipt.
 */
export async function waitForSuccessfulReceipt<chain extends Chain | undefined>(
  publicClient: PublicClient<Transport, chain>,
  hash: Hash,
): Promise<WaitForTransactionReceiptReturnType<chain>> {
  const receipt = await publicClient.waitForTransactionReceipt({ hash, timeout: 0 });
  if (receipt.status === "reverted") {
    throw new Error(`Transaction ${hash} reverted in block ${receipt.blockNumber}.`);
  }
  return receipt;
}
