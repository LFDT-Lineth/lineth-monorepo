import { Page } from "@playwright/test";

export async function getNativeBridgeTransactionsCountImpl(page: Page): Promise<number> {
  const txList = page.getByTestId("native-bridge-transaction-history-list");
  const noTransactionsYetText = page.getByText("No transactions yet");
  // Either `txList` or `noTransactionsYetText` will appear. Should be mutually exclusive.
  await Promise.race([txList.waitFor({ state: "visible" }), noTransactionsYetText.waitFor({ state: "visible" })]);

  // Check which element is actually visible
  const isTxListVisible = await txList.isVisible();
  if (!isTxListVisible) return 0;
  const txs = txList.getByRole("listitem");
  const txCount = txs.count();
  return txCount;
}
