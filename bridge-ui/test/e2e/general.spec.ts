import { testWithSynpress } from "@synthetixio/synpress";

import { test as advancedFixtures } from "../advancedFixtures";
import { TEST_URL, WEI_AMOUNT, ETH_SYMBOL, LOCAL_L2_NETWORK, L1_ACCOUNT_METAMASK_NAME } from "../constants";

const test = testWithSynpress(advancedFixtures);

const { expect, describe } = test;

// There are known lines causing flaky E2E tests in this test suite, these are annotated by 'bridge-ui-known-flaky-line'
describe("General", () => {
  test.describe.configure({ mode: "parallel" });

  test("should successfully go to the bridge UI page", async ({ page }) => {
    const pageUrl = page.url();
    expect(pageUrl).toEqual(TEST_URL);
  });

  test("should have 'Native Bridge' button link on homepage", async ({
    clickNativeBridgeButton,
    clickFirstVisitModalConfirmButton,
    clickTermsOfServiceModalAcceptButton,
  }) => {
    await clickTermsOfServiceModalAcceptButton();
    const nativeBridgeBtn = await clickNativeBridgeButton();
    await clickFirstVisitModalConfirmButton();
    await expect(nativeBridgeBtn).toBeVisible();
  });

  test("should not display transaction history when not connected", async ({
    page,
    clickNativeBridgeButton,
    clickFirstVisitModalConfirmButton,
    clickTermsOfServiceModalAcceptButton,
    openNativeBridgeTransactionHistory,
  }) => {
    await clickTermsOfServiceModalAcceptButton();
    await clickNativeBridgeButton();
    await clickFirstVisitModalConfirmButton();
    await openNativeBridgeTransactionHistory();

    const connectWalletText = page.getByTestId("tx-history-connect-your-wallet-text");
    await expect(connectWalletText).toHaveText("Please connect your wallet.");
  });

  test("should connect MetaMask to dapp correctly", async ({
    connectMetamaskToDapp,
    clickNativeBridgeButton,
    clickFirstVisitModalConfirmButton,
    clickTermsOfServiceModalAcceptButton,
  }) => {
    await clickTermsOfServiceModalAcceptButton();
    await clickNativeBridgeButton();
    await clickFirstVisitModalConfirmButton();
    await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
  });

  test("should be able to load the transaction history", async ({
    page,
    connectMetamaskToDapp,
    clickNativeBridgeButton,
    openNativeBridgeTransactionHistory,
    clickFirstVisitModalConfirmButton,
    clickTermsOfServiceModalAcceptButton,
  }) => {
    await clickTermsOfServiceModalAcceptButton();
    await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
    await clickNativeBridgeButton();
    await clickFirstVisitModalConfirmButton();
    await openNativeBridgeTransactionHistory();

    const txHistoryHeading = page.getByRole("heading").filter({ hasText: "Transaction History" });
    await expect(txHistoryHeading).toBeVisible();
  });

  test("should not be able to bridge on the wrong network", async ({
    page,
    metamask,
    connectMetamaskToDapp,
    clickNativeBridgeButton,
    selectTokenAndInputAmount,
    swapChain,
    clickFirstVisitModalConfirmButton,
    clickTermsOfServiceModalAcceptButton,
  }) => {
    test.setTimeout(60_000);
    await clickTermsOfServiceModalAcceptButton();
    await connectMetamaskToDapp(L1_ACCOUNT_METAMASK_NAME);
    await clickNativeBridgeButton();
    await clickFirstVisitModalConfirmButton();

    await swapChain();
    await selectTokenAndInputAmount(ETH_SYMBOL, WEI_AMOUNT);

    // Should have 'Switch to Local L2 Network' network button visible and enabled
    const switchBtn = page.getByRole("button", { name: `Switch to ${LOCAL_L2_NETWORK.name}`, exact: true });
    await expect(switchBtn).toBeVisible({ timeout: 10_000 });
    await expect(switchBtn).toBeEnabled({ timeout: 10_000 });

    // Do network switch
    await switchBtn.click();
    await metamask.approveSwitchNetwork();

    // After network switch, should have 'Bridge' button visible and enabled
    const bridgeButton = page.getByRole("button", { name: "Bridge", exact: true });
    await expect(bridgeButton).toBeVisible({ timeout: 10_000 });
    await expect(bridgeButton).toBeEnabled({ timeout: 10_000 });
  });
});
