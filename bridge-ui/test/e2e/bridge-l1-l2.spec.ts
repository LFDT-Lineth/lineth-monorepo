import { testWithSynpress } from "@synthetixio/synpress";

import { test as advancedFixtures } from "../advancedFixtures";
import {
  USDC_SYMBOL,
  USDC_AMOUNT,
  WEI_AMOUNT,
  ETH_SYMBOL,
  LOCAL_L2_NETWORK,
  ERC20_SYMBOL,
  ERC20_AMOUNT,
  L1_ACCOUNT_METAMASK_NAME,
} from "../constants";

const test = testWithSynpress(advancedFixtures);

const { expect, describe } = test;

describe("L1 > L2 via Native Bridge", () => {
  describe("No blockchain tx cases", () => {
    test.describe.configure({ mode: "parallel" });

    test("should display fees and bridge information when not connected", async ({
      page,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      clickFirstVisitModalConfirmButton,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      test.setTimeout(60_000);

      await clickTermsOfServiceModalAcceptButton();
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT, false);

      const receivedAmount = page.getByTestId("received-amount-text");
      await expect(receivedAmount).toBeVisible();
      await expect(receivedAmount).toHaveText(`${WEI_AMOUNT} ETH`);
    });

    test("should see Free gas fees for ETH transfer to L2", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      openGasFeeModal,
      clickFirstVisitModalConfirmButton,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      test.setTimeout(60_000);

      await clickTermsOfServiceModalAcceptButton();
      await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT);
      await openGasFeeModal();

      // Assert text items
      const l2NetworkFeeText = page.getByText(`${LOCAL_L2_NETWORK.name} fee`);
      const freeText = page.getByText("Free");
      await expect(l2NetworkFeeText).toBeVisible();
      await expect(freeText).toBeVisible();
      const listItem = page
        .locator("li")
        .filter({
          has: l2NetworkFeeText,
        })
        .filter({
          has: freeText,
        });
      await expect(listItem).toBeVisible();
    });

    // This test is skipped because CCTP is not supported on the local stack.
    test.skip("should not see Free gas fees for USDC transfer to L2", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      openGasFeeModal,
      clickFirstVisitModalConfirmButton,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      test.setTimeout(60_000);

      await clickTermsOfServiceModalAcceptButton();
      await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      await selectTokenAndInputAmount(USDC_SYMBOL, USDC_AMOUNT);
      await openGasFeeModal();

      // Assert text items
      const freeText = page.getByText("Free");
      await expect(freeText).not.toBeVisible();
    });
  });

  describe("Blockchain tx cases", () => {
    // If not serial risk colliding nonces -> transactions cancelling each other out
    test.describe.configure({ retries: 1, timeout: 120_000, mode: "serial" });

    test("should be able to initiate bridging ETH from L1 to L2", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      clickFirstVisitModalConfirmButton,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      // Setup testnet UI
      await clickTermsOfServiceModalAcceptButton();
      await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Get # of txs in txHistory before doing bridge tx, so that we can later confirm that our bridge tx shows up in the txHistory.
      await openNativeBridgeTransactionHistory();
      const txnsLengthBefore = await getNativeBridgeTransactionsCount();
      await closeNativeBridgeTransactionHistory();

      // // Actual bridging actions
      await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT);
      await doInitiateBridgeTransaction();

      // Check that our bridge tx shows up in the tx history
      await waitForNewTxAdditionToTxList(txnsLengthBefore);
    });

    test("should be able to initiate bridging ERC20 from L1 to L2", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      clickFirstVisitModalConfirmButton,
      doTokenApprovalIfNeeded,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      // Setup testnet UI
      await clickTermsOfServiceModalAcceptButton();
      await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Get # of txs in txHistory before doing bridge tx, so that we can later confirm that our bridge tx shows up in the txHistory.
      await openNativeBridgeTransactionHistory();
      const txnsLengthBefore = await getNativeBridgeTransactionsCount();
      await closeNativeBridgeTransactionHistory();

      // // Actual bridging actions
      await selectTokenAndInputAmount(ERC20_SYMBOL, ERC20_AMOUNT);
      await doTokenApprovalIfNeeded();
      await doInitiateBridgeTransaction();

      // Check that our bridge tx shows up in the tx history
      await waitForNewTxAdditionToTxList(txnsLengthBefore);
    });

    // This test is skipped because CCTP is not supported on the local stack.
    test.skip("should be able to initiate bridging USDC from L1 to L2 in testnet", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      doTokenApprovalIfNeeded,
      clickFirstVisitModalConfirmButton,
      clickTermsOfServiceModalAcceptButton,
    }) => {
      // Setup testnet UI
      await clickTermsOfServiceModalAcceptButton();
      await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
      await clickNativeBridgeButton();
      await clickFirstVisitModalConfirmButton();

      // Get # of txs in txHistory before doing bridge tx, so that we can later confirm that our bridge tx shows up in the txHistory.
      await openNativeBridgeTransactionHistory();
      const txnsLengthBefore = await getNativeBridgeTransactionsCount();
      await closeNativeBridgeTransactionHistory();

      // Actual bridging actions
      await selectTokenAndInputAmount(USDC_SYMBOL, USDC_AMOUNT);
      await doTokenApprovalIfNeeded();
      await doInitiateBridgeTransaction();

      // Check that our bridge tx shows up in the tx history
      await waitForNewTxAdditionToTxList(txnsLengthBefore);
    });
  });
});
