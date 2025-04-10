import { testWithSynpress } from "@synthetixio/synpress";
import { test as advancedFixtures } from "../advancedFixtures";
import { TEST_URL, USDC_SYMBOL, USDC_AMOUNT, WEI_AMOUNT, ETH_SYMBOL } from "../constants";

const test = testWithSynpress(advancedFixtures);

const { expect, describe } = test;

// There are known lines causing flaky E2E tests in this test suite, these are annotated by 'bridge-ui-known-flaky-line'
describe("L1 > L2 via Native Bridge", () => {
  describe("No blockchain tx cases", () => {
    test.describe.configure({ mode: "parallel" });

    test("should successfully go to the bridge UI page", async ({ page }) => {
      const pageUrl = page.url();
      expect(pageUrl).toEqual(TEST_URL);
    });

    test("should have 'Native Bridge' button link on homepage", async ({ clickNativeBridgeButton }) => {
      const nativeBridgeBtn = await clickNativeBridgeButton();
      await expect(nativeBridgeBtn).toBeVisible();
    });

    test("should connect MetaMask to dapp correctly", async ({ connectMetamaskToDapp, clickNativeBridgeButton }) => {
      await clickNativeBridgeButton();
      await connectMetamaskToDapp();
    });

    test("should be able to load the transaction history", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeTransactionHistory,
    }) => {
      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeTransactionHistory();

      const txHistoryHeading = page.getByRole("heading").filter({ hasText: "Transaction History" });
      await expect(txHistoryHeading).toBeVisible();
    });

    test("should be able to switch to test networks", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeFormSettings,
      toggleShowTestNetworksInNativeBridgeForm,
    }) => {
      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeFormSettings();
      await toggleShowTestNetworksInNativeBridgeForm();

      // Should have Sepolia text visible
      const sepoliaText = page.getByText("Sepolia").first();
      await expect(sepoliaText).toBeVisible();
    });

    test("should not be able to approve on the wrong network", async ({
      page,
      metamask,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeFormSettings,
      toggleShowTestNetworksInNativeBridgeForm,
      selectTokenAndInputAmount,
      switchToEthereumMainnet,
    }) => {
      test.setTimeout(60_000);

      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeFormSettings();
      await toggleShowTestNetworksInNativeBridgeForm();

      await switchToEthereumMainnet();
      await selectTokenAndInputAmount(USDC_SYMBOL, USDC_AMOUNT);

      // Should have 'Switch to Sepolia' network button visible and enabled
      const switchBtn = page.getByRole("button", { name: "Switch to Sepolia", exact: true });
      await expect(switchBtn).toBeVisible();
      await expect(switchBtn).toBeEnabled();

      // Do network switch
      await switchBtn.click();
      await metamask.approveSwitchNetwork();

      // After network switch, should have 'Approve Token' button visible and enabled
      const approvalButton = page.getByRole("button", { name: "Approve Token", exact: true });
      await expect(approvalButton).toBeVisible();
      await expect(approvalButton).toBeEnabled();
    });
  });

  describe("Blockchain tx cases", () => {
    // If not serial risk colliding nonces -> transactions cancelling each other out
    test.describe.configure({ retries: 1, timeout: 120_000, mode: "serial" });

    test("should be able to initiate bridging ETH from L1 to L2 in testnet", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeFormSettings,
      toggleShowTestNetworksInNativeBridgeForm,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
    }) => {
      // Setup testnet UI
      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeFormSettings();
      await toggleShowTestNetworksInNativeBridgeForm();

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

    test("should be able to initiate bridging USDC from L1 to L2 in testnet", async ({
      getNativeBridgeTransactionsCount,
      waitForNewTxAdditionToTxList,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeFormSettings,
      toggleShowTestNetworksInNativeBridgeForm,
      selectTokenAndInputAmount,
      doInitiateBridgeTransaction,
      openNativeBridgeTransactionHistory,
      closeNativeBridgeTransactionHistory,
      doTokenApprovalIfNeeded,
    }) => {
      // Setup testnet UI
      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeFormSettings();
      await toggleShowTestNetworksInNativeBridgeForm();

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

    test("should be able to claim if available READY_TO_CLAIM transactions", async ({
      page,
      connectMetamaskToDapp,
      clickNativeBridgeButton,
      openNativeBridgeFormSettings,
      toggleShowTestNetworksInNativeBridgeForm,
      openNativeBridgeTransactionHistory,
      getNativeBridgeTransactionsCount,
      switchToLineaSepolia,
      doClaimTransaction,
      waitForTxListUpdateForClaimTx,
    }) => {
      await connectMetamaskToDapp();
      await clickNativeBridgeButton();
      await openNativeBridgeFormSettings();
      await toggleShowTestNetworksInNativeBridgeForm();

      // Switch to L2 network
      await switchToLineaSepolia();

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
