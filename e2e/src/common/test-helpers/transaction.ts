import { expect } from "@jest/globals";

export async function expectBlockedTransaction(sendTransactionPromise: Promise<`0x${string}`>): Promise<void> {
  await expect(sendTransactionPromise).rejects.toThrow("blocked");
}
