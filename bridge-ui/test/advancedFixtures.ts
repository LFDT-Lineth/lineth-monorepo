import { getExtensionId, metaMaskFixtures } from "@synthetixio/synpress/playwright";
import { Locator, Page } from "@playwright/test";
import setup from "./wallet-setup/metamask.setup";
import { getNativeBridgeTransactionsCountImpl, selectTokenAndWaitForBalance } from "./utils";
import { L1_ACCOUNT_METAMASK_NAME, LOCAL_L2_NETWORK, PAGE_TIMEOUT, POLLING_INTERVAL } from "./constants";

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
  clickFirstVisitModalConfirmButton: () => Promise<void>;
  clickNativeBridgeButton: () => Promise<Locator>;
  openNativeBridgeTransactionHistory: () => Promise<void>;
  closeNativeBridgeTransactionHistory: () => Promise<void>;
  openNativeBridgeFormSettings: () => Promise<void>;
  toggleShowTestNetworksInNativeBridgeForm: () => Promise<void>;
  getNativeBridgeTransactionsCount: () => Promise<number>;
  selectTokenAndInputAmount: (tokenSymbol: string, amount: string, waitForBalance?: boolean) => Promise<void>;
  waitForNewTxAdditionToTxList: (txCountBeforeUpdate: number) => Promise<void>;
  waitForTxListUpdateForClaimTx: (claimTxCountBeforeUpdate: number) => Promise<void>;
  openGasFeeModal: () => Promise<void>;

  // Metamask Actions - Should be ok to reuse within other fixture functions
  connectMetamaskToDapp: (account: string) => Promise<void>;
  openMetamaskActivityPage: () => Promise<void>;
  submitERC20ApprovalTx: () => Promise<void>;
  waitForTransactionToConfirm: () => Promise<void>;
  confirmTransactionAndWaitForInclusion: () => Promise<void>;
  switchToL2Network: () => Promise<void>;
  swapChain: () => Promise<void>;

  // Composite Bridge UI + Metamask Actions
  doTokenApprovalIfNeeded: () => Promise<void>;
  doInitiateBridgeTransaction: () => Promise<void>;
  doClaimTransaction: () => Promise<void>;
}>({
  clickFirstVisitModalConfirmButton: async ({ page }, use) => {
    await use(async () => {
      const confirmButton = page.getByTestId("first-visit-modal-confirm-btn");
      await expect(confirmButton).toBeVisible();
      await expect(confirmButton).toBeEnabled();
      await confirmButton.click();
    });
  },
  // Bridge UI Actions
  clickNativeBridgeButton: async ({ page }, use) => {
    await use(async () => {
      const nativeBridgeLink = page.getByTestId("nav-item-Native-Bridge");
      await nativeBridgeLink.click();
      return nativeBridgeLink;
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
    await use(async (tokenSymbol: string, amount: string, waitForBalance = true) => {
      // Wait for page to retrieve blockchain token balance
      await selectTokenAndWaitForBalance(tokenSymbol, page, waitForBalance);

      // Input amount
      const amountInput = page.getByTestId("amount-input");
      await amountInput.fill(amount);

      // Wait for "Receive amount" to populate, we need to fetch blockchain data before proceeding
      const receivedAmountField = page.getByTestId("received-amount-text");
      await receivedAmountField.waitFor({ state: "visible", timeout: PAGE_TIMEOUT });

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
  openGasFeeModal: async ({ page }, use) => {
    await use(async () => {
      const gasFeeBtn = page.getByRole("button", { name: "fee-chain-icon" });
      await expect(gasFeeBtn).not.toContainText("0.00000000", { timeout: PAGE_TIMEOUT });
      await gasFeeBtn.click();
    });
  },

  // Metamask Actions - Should be ok to reuse within other fixture functions
  connectMetamaskToDapp: async ({ page, metamask }, use) => {
    await use(async (account: string) => {
      await page.waitForLoadState("domcontentloaded", { timeout: PAGE_TIMEOUT });

      await page.emulateMedia({ reducedMotion: "reduce" });
      await page.addStyleTag({
        content: `*,*::before,*::after { transition: none !important; animation: none !important }`,
      });

      // Click Connect button
      const connectBtn = page.getByRole("button", { name: "Connect", exact: true }).first();
      await connectBtn.click();

      // Click on 'Metamask' on the wallet dropdown menu
      const metamaskBtnInDropdownList = page.getByRole("button").filter({ hasText: "MetaMask" }).first();
      await metamaskBtnInDropdownList.click();

      if (account !== L1_ACCOUNT_METAMASK_NAME) {
        // Switch to the specified account (used for L2 to L1 tests)
        await metamask.switchAccount(account);
      }

      await metamask.connectToDapp([account]);
      await metamask.goBackToHomePage();
      await page.bringToFront();
    });
  },
  openMetamaskActivityPage: async ({ metamask }, use) => {
    await use(async () => {
      await metamask.page.bringToFront();
      await metamask.page.reload();

      const activityButton = metamask.page.locator("button", { hasText: "Activity" });
      await activityButton.waitFor();
      // bridge-ui-known-flaky-line - Sometimes and unpredictably a "What's new" modal pops up on Metamask. This modal blocks other actions.
      // We assume that the this button is available at the same time that the Activity button becomes available
      const gotItButton = metamask.page.locator("button", { hasText: "Got it" });
      if (await gotItButton.isVisible()) await gotItButton.click();
      // Click Activity button
      await activityButton.click();
    });
  },
  // We use this instead of metamask.approveTokenPermission because we found the original method flaky
  submitERC20ApprovalTx: async ({ context, page, metamask }, use) => {
    await use(async () => {
      metamask.approveTokenPermission;
      // Need to wait for Metamask Notification page to exist, does not exist immediately after clicking 'Approve' button.
      // In Synpress source code, they use this logic in every method interacting with the Metamask notification page.
      const extensionId = await getExtensionId(context, "MetaMask");
      const notificationPageUrl = `chrome-extension://${extensionId}/notification.html`;
      while (
        metamask.page
          .context()
          .pages()
          .find((page) => page.url().includes(notificationPageUrl)) === undefined
      ) {
        await page.waitForTimeout(POLLING_INTERVAL);
      }
      const notificationPage = metamask.page
        .context()
        .pages()
        .find((page) => page.url().includes(notificationPageUrl)) as Page;
      await notificationPage.waitForLoadState("domcontentloaded", { timeout: PAGE_TIMEOUT });
      await notificationPage.waitForLoadState("networkidle", { timeout: PAGE_TIMEOUT });
      await metamask.page.reload();
      // Unsure if commented out below are required to mitigate flakiness
      // await metamask.page.waitForLoadState("domcontentloaded", { timeout: PAGE_TIMEOUT });
      // await metamask.page.waitForLoadState("networkidle", { timeout: PAGE_TIMEOUT });
      const nextBtn = metamask.page.getByRole("button", { name: "Next", exact: true });
      // Unsure if commented out below are required to mitigate flakiness
      // await expect(nextBtn).toBeVisible();
      // await expect(nextBtn).toBeEnabled();
      await nextBtn.click();
      const approveMMBtn = metamask.page.getByRole("button", { name: "Approve", exact: true });
      await approveMMBtn.click();
    });
  },
  waitForTransactionToConfirm: async ({ metamask, openMetamaskActivityPage }, use) => {
    await use(async () => {
      await openMetamaskActivityPage();
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
  switchToL2Network: async ({ metamask }, use) => {
    await use(async () => {
      await metamask.switchNetwork(LOCAL_L2_NETWORK.name, true);
    });
  },

  // Composite Bridge UI + Metamask Actions
  doTokenApprovalIfNeeded: async ({ page, submitERC20ApprovalTx, waitForTransactionToConfirm }, use) => {
    await use(async () => {
      // Check if approval required
      const approvalButton = page.getByRole("button", { name: "Approve Token", exact: true });
      if ((await approvalButton.count()) === 0) return;
      await approvalButton.click();

      // Handle Metamask approval UI
      await submitERC20ApprovalTx();
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
  swapChain: async ({ page }, use) => {
    await use(async () => {
      const swapChainButton = page.getByTestId("swap-chain-button");
      await expect(swapChainButton).toBeVisible();
      await expect(swapChainButton).toBeEnabled();
      await swapChainButton.click();
    });
  },
});

export const { expect, describe } = test;
