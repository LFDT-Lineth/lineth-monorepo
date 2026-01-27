import { testWithSynpress } from "@synthetixio/synpress";

import { test as advancedFixtures } from "../advancedFixtures";
import { WEI_AMOUNT, ETH_SYMBOL, ERC20_SYMBOL, ERC20_AMOUNT, L2_ACCOUNT_METAMASK_NAME } from "../constants";

const test = testWithSynpress(advancedFixtures);

const { expect, describe } = test;

describe("L2 > L1 via Native Bridge", () => {
  describe("No blockchain tx cases", () => {
    test.describe.configure({ mode: "parallel" });

    test("should display fees and bridge information when not connected", async ({
      page,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      clickFirstVisitModalConfirmButton,
      switchToL2Network,
    }) => {
      test.setTimeout(60_000);

      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await switchToL2Network();

      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT, false);

      const receivedAmount = page.getByTestId("received-amount-text");
      await expect(receivedAmount).toBeVisible();
    });

    test("should automatically set bridge mode to 'manual' for L2 to L1 bridge", async ({
      page,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      clickFirstVisitModalConfirmButton,
      swapChain,
      connectMetamaskToDapp,
    }) => {
      test.setTimeout(60_000);

      await connectMetamaskToDapp(L2_ACCOUNT_METAMASK_NAME); // Connect to L2 account
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await swapChain();

      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT, false);

      const manualModeBtn = page.getByTestId("manual-mode-btn");
      await expect(manualModeBtn).toBeVisible();
      await expect(manualModeBtn).toHaveText("Manual");
    });

    test("should not see Free gas fees for ETH transfer to L1", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      openGasFeeModal,
      switchToL2Network,
      clickFirstVisitModalConfirmButton,
    }) => {
      test.setTimeout(60_000);

      await connectMetamaskToDapp(L2_ACCOUNT_METAMASK_NAME); // Connect to L2 account
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await switchToL2Network();
      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT);
      await openGasFeeModal();

      // Assert text items
      const freeText = page.getByText("Free");
      await expect(freeText).not.toBeVisible();
    });
  });

  describe("Blockchain tx cases", () => {
    // If not serial risk colliding nonces -> transactions cancelling each other out
    test.describe.configure({ retries: 1, timeout: 120_000, mode: "serial" });

    test("should be able to initiate bridging ETH from L2 to L1", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      clickFirstVisitModalConfirmButton,
      switchToL2Network,
      swapChain,
    }) => {
      // Setup testnet UI
      await connectMetamaskToDapp(L2_ACCOUNT_METAMASK_NAME); // Connect to L2 account
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Switch to L2 network
      await switchToL2Network();

      // Get # of txs in txHistory before doing bridge tx, so that we can later confirm that our bridge tx shows up in the txHistory.
      await openNativeBridgeTransactionHistory();
      const txnsLengthBefore = await getNativeBridgeTransactionsCount();
      await closeNativeBridgeTransactionHistory();

      // Swap chain to L2
      await swapChain();

      // // Actual bridging actions
      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT);
      await doInitiateBridgeTransaction();

      // Check that our bridge tx shows up in the tx history
      await waitForNewTxAdditionToTxList(txnsLengthBefore);
    });

    test("should be able to initiate bridging ERC20 from L2 to L1", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      clickFirstVisitModalConfirmButton,
      switchToL2Network,
      swapChain,
      doTokenApprovalIfNeeded,
    }) => {
      // Setup testnet UI
      await connectMetamaskToDapp(L2_ACCOUNT_METAMASK_NAME); // Connect to L2 account
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Switch to L2 network
      await switchToL2Network();

      // Get # of txs in txHistory before doing bridge tx, so that we can later confirm that our bridge tx shows up in the txHistory.
      await openNativeBridgeTransactionHistory();
      const txnsLengthBefore = await getNativeBridgeTransactionsCount();
      await closeNativeBridgeTransactionHistory();

      // Swap chain to L2
      await swapChain();

      // // Actual bridging actions
      await selectTokenAndInputAmount(ERC20_SYMBOL, ERC20_AMOUNT);
      await doTokenApprovalIfNeeded();
      await doInitiateBridgeTransaction();

      // Check that our bridge tx shows up in the tx history
      await waitForNewTxAdditionToTxList(txnsLengthBefore);
    });

    test("should be able to claim if available READY_TO_CLAIM transactions", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeTransactionHistory,
      getNativeBridgeTransactionsCount,
      doClaimTransaction,
      waitForTxListUpdateForClaimTx,
      clickFirstVisitModalConfirmButton,
    }) => {
      await connectMetamaskToDapp(L2_ACCOUNT_METAMASK_NAME); // Connect to L2 account
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Load tx history
      await openNativeBridgeTransactionHistory();
      await getNativeBridgeTransactionsCount();

      // Find and click READY_TO_CLAIM TX
      const readyToClaimTx = page.getByRole("listitem").filter({ hasText: "Ready to claim" });
      const readyToClaimCount = await readyToClaimTx.count();
      if (readyToClaimCount === 0) return;
      await readyToClaimTx.first().click();

      await doClaimTransaction();

      // Check that tx history has updated accordingly
      await waitForTxListUpdateForClaimTx(readyToClaimCount);
    });
  });
});
