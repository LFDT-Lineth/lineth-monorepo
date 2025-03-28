import { metaMaskFixtures } from "@synthetixio/synpress/playwright";
import setup from "./wallet-setup/metamask.setup";
import { Locator } from "@playwright/test";
import { getNativeBridgeTransactionsCountImpl, selectTokenAndWaitForBalance } from "./utils";
import { LINEA_SEPOLIA_NETWORK, POLLING_INTERVAL } from "./constants";
/**
 * NB: There is an issue with Synpress `metaMaskFixtures` extension functions wherein extension functions
 * may not be able to reuse other extension functions. This is especially the case when advanced operations
 * on the 'Page' object are done. It seems that the 'Page' object does not remain the same in a nested
 * extension function call between the different layers of nesting.
 *
 * Nested `Metamask` object uses however seem ok.
 */
export const test = metaMaskFixtures(setup).extend<{
  // Bridge UI Actions
  clickNativeBridgeButton: () => Promise<Locator>;
  openNativeBridgeTransactionHistory: () => Promise<void>;
  closeNativeBridgeTransactionHistory: () => Promise<void>;
  openNativeBridgeFormSettings: () => Promise<void>;
  toggleShowTestNetworksInNativeBridgeForm: () => Promise<void>;
  getNativeBridgeTransactionsCount: () => Promise<number>;
  selectTokenAndInputAmount: (tokenSymbol: string, amount: string) => Promise<void>;
  waitForNewTxAdditionToTxList: (txCountBeforeUpdate: number) => Promise<void>;
  waitForTxListUpdateForClaimTx: (claimTxCountBeforeUpdate: number) => Promise<void>;

  // Metamask Actions - Should be ok to reuse within other fixture functions
  connectMetamaskToDapp: () => Promise<void>;
  waitForTransactionToConfirm: () => Promise<void>;
  confirmTransactionAndWaitForInclusion: () => Promise<void>;
  switchToLineaSepolia: () => Promise<void>;

  // Composite Bridge UI + Metamask Actions
  doTokenApprovalIfNeeded: () => Promise<void>;
  doInitiateBridgeTransaction: () => Promise<void>;
  doClaimTransaction: () => Promise<void>;
}>({
  // Bridge UI Actions
  clickNativeBridgeButton: async ({ page }, use) => {
    await use(async () => {
      const nativeBridgeBtn = page.getByRole("link", { name: "Native Bridge", exact: true });
      await nativeBridgeBtn.click();
      return nativeBridgeBtn;
    });
  },
  openNativeBridgeTransactionHistory: async ({ page }, use) => {
    await use(async () => {
      const txHistoryIconButton = page.getByTestId("native-bridge-transaction-history-icon");
      await txHistoryIconButton.click();
    });
  },
  closeNativeBridgeTransactionHistory: async ({ page }, use) => {
    await use(async () => {
      const backButton = page.getByTestId("transaction-history-close-btn");
      await backButton.click();
    });
  },
  openNativeBridgeFormSettings: async ({ page }, use) => {
    await use(async () => {
      const formSettingsIconButton = page.getByTestId("native-bridge-form-settings-icon");
      await formSettingsIconButton.click();
    });
  },
  toggleShowTestNetworksInNativeBridgeForm: async ({ page }, use) => {
    await use(async () => {
      await page.getByTestId("native-bridge-test-network-toggle").click();
    });
  },
  getNativeBridgeTransactionsCount: async ({ page }, use) => {
    await use(async () => {
      return await getNativeBridgeTransactionsCountImpl(page);
    });
  },
  selectTokenAndInputAmount: async ({ page }, use) => {
    await use(async (tokenSymbol: string, amount: string) => {
      // Wait for page to retrieve blockchain token balance
      await selectTokenAndWaitForBalance(tokenSymbol, page);

      // Input amount
      const amountInput = page.getByRole("textbox", { name: "0", exact: true });
      await amountInput.fill(amount);

      // Wait for "Receive amount" to populate, we need to fetch blockchain data before proceeding
      const receivedAmountField = page.getByTestId("received-amount-text");
      await receivedAmountField.waitFor({ state: "visible" });

      // Check if there are sufficient funds available
      const insufficientFundsButton = page.getByRole("button", { name: "Insufficient funds", exact: true });
      if ((await insufficientFundsButton.count()) > 0)
        throw "Insufficient funds available, please add some funds before running the test";
    });
  },
  waitForNewTxAdditionToTxList: async ({ page }, use) => {
    await use(async (txCountBeforeUpdate: number) => {
      const maxTries = 10;
      let tryCount = 0;
      let listUpdated = false;
      do {
        const newTxCount = await getNativeBridgeTransactionsCountImpl(page);
        listUpdated = newTxCount !== txCountBeforeUpdate;
        tryCount++;
        await page.waitForTimeout(POLLING_INTERVAL);
      } while (!listUpdated && tryCount < maxTries);
    });
  },
  waitForTxListUpdateForClaimTx: async ({ page }, use) => {
    await use(async (claimTxCountBeforeUpdate: number) => {
      const maxTries = 10;
      const readyToClaimTx = page.getByRole("listitem").filter({ hasText: "Ready to claim" });
      let tryCount = 0;
      let listUpdated = false;
      do {
        const newReadyToClaimCount = await readyToClaimTx.count();
        listUpdated = newReadyToClaimCount === claimTxCountBeforeUpdate + 1;
        tryCount++;
        await page.waitForTimeout(POLLING_INTERVAL);
      } while (!listUpdated && tryCount < maxTries);
    });
  },

  // Metamask Actions - Should be ok to reuse within other fixture functions
  connectMetamaskToDapp: async ({ page, metamask }, use) => {
    await use(async () => {
      // Click Connect button
      const connectBtn = page.getByRole("button", { name: "Connect", exact: true }).first();
      await connectBtn.click();

      // Click on 'Metamask' on the wallet dropdown menu
      const metamaskBtnInDropdownList = page.getByRole("button").filter({ hasText: "MetaMask" }).first();
      await metamaskBtnInDropdownList.click();

      await metamask.connectToDapp();
      await metamask.goBackToHomePage();
      await page.bringToFront();
    });
  },
  waitForTransactionToConfirm: async ({ metamask }, use) => {
    await use(async () => {
      await metamask.page.bringToFront();
      await metamask.page.reload();

      const activityButton = metamask.page.locator("button", { hasText: "Activity" });
      await activityButton.waitFor();
      // Sometimes a "What's new" modal pops up on Metamask. We assume this becomes visible at the same time as the Activity button
      // This modal causes flaky tests because it appears unpredictably, and blocks other actions.
      const gotItButton = metamask.page.locator("button", { hasText: "Got it" });
      if (await gotItButton.isVisible()) await gotItButton.click();
      // Click Activity button
      await activityButton.click();

      let txCount = await metamask.page
        .locator(metamask.homePage.selectors.activityTab.pendingApprovedTransactions)
        .count();
      while (txCount > 0) {
        txCount = await metamask.page
          .locator(metamask.homePage.selectors.activityTab.pendingApprovedTransactions)
          .count();
      }
    });
  },
  confirmTransactionAndWaitForInclusion: async ({ page, metamask, waitForTransactionToConfirm }, use) => {
    await use(async () => {
      await metamask.confirmTransaction();
      await waitForTransactionToConfirm();
      await page.bringToFront();
    });
  },
  switchToLineaSepolia: async ({ metamask }, use) => {
    await use(async () => {
      await metamask.switchNetwork(LINEA_SEPOLIA_NETWORK.name, true);
    });
  },

  // Composite Bridge UI + Metamask Actions
  doTokenApprovalIfNeeded: async ({ page, metamask, waitForTransactionToConfirm }, use) => {
    await use(async () => {
      // Check if approval required
      const approvalButton = page.getByRole("button", { name: "Approve Token", exact: true });
      if ((await approvalButton.count()) === 0) return;
      await approvalButton.click();

      // Handle Metamask approval UI
      await metamask.approveTokenPermission();
      await waitForTransactionToConfirm();

      // Close 'Transaction successful' modal
      await page.bringToFront();
      const closeModalBtn = page.getByRole("button", { name: "Bridge your token", exact: true });
      await closeModalBtn.click();
    });
  },
  doInitiateBridgeTransaction: async ({ page, confirmTransactionAndWaitForInclusion }, use) => {
    await use(async () => {
      // Click "Bridge" button
      const bridgeButton = page.getByRole("button", { name: "Bridge", exact: true });
      await bridgeButton.waitFor();
      await bridgeButton.click();

      // Click "Confirm and Bridge" button
      const confirmAndBridgeButton = page.getByTestId("confirm-and-bridge-btn");
      await expect(confirmAndBridgeButton).toBeVisible();
      await expect(confirmAndBridgeButton).toBeEnabled();
      await confirmAndBridgeButton.click();

      // Confirm Metamask Tx and wait for blockchain inclusion
      // Should be ok to reuse this fixture function because it doesn't do much on the `Page` object
      await confirmTransactionAndWaitForInclusion();

      // Click on 'View transactions' button on the 'Transaction confirmed' modal
      const viewTxButton = page.getByRole("button", { name: "View transactions", exact: true });
      await viewTxButton.click();
    });
  },
  doClaimTransaction: async ({ page, confirmTransactionAndWaitForInclusion }, use) => {
    await use(async () => {
      // Click on 'Claim' button
      const claimButton = page.getByRole("button", { name: "Claim", exact: true });
      await expect(claimButton).toBeVisible();
      await expect(claimButton).toBeEnabled();
      await claimButton.click();

      // Confirm Metamask Tx and wait for blockchain inclusion
      // Should be ok to reuse this fixture function because it doesn't do much on the `Page` object
      await confirmTransactionAndWaitForInclusion();

      // Should finish on tx history page
    });
  },
});

export const { expect, describe } = test;
