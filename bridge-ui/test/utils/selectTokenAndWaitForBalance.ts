import { Page } from "@playwright/test";
import { POLLING_INTERVAL, PAGE_TIMEOUT } from "../constants";

export async function selectTokenAndWaitForBalance(tokenSymbol: string, page: Page) {
  const openModalBtn = page.getByTestId("native-bridge-open-token-list-modal");
  await openModalBtn.click();
  // Wait for API request to retrieve blockchain balance.
  const tokenBalance = page.getByTestId(`token-details-${tokenSymbol.toLowerCase()}-amount`);
  console.log(`Fetching token balance for ${tokenSymbol}`);

  // Timeout implementation
  let fetchTokenTimeUsed = 0;
  // bridge-ui-known-flaky-line - Sometimes the RPC call to get ETH/ERC20 balance fails
  while ((await tokenBalance.textContent()) === `0 ${tokenSymbol}`) {
    if (fetchTokenTimeUsed >= PAGE_TIMEOUT)
      throw `Could not find any balance for ${tokenSymbol}, does the testing wallet have funds?`;
    await page.waitForTimeout(POLLING_INTERVAL);
    fetchTokenTimeUsed += POLLING_INTERVAL;
  }
  console.log(`Selected token balance: ${await tokenBalance.textContent()}`);

  // Select token
  await page.getByTestId(`token-details-${tokenSymbol.toLowerCase()}-btn`).click();
}
