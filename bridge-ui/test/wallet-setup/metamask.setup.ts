import { LINEA_SEPOLIA_NETWORK, METAMASK_PASSWORD, METAMASK_SEED_PHRASE, TEST_PRIVATE_KEY } from "../constants";
import { defineWalletSetup } from "@synthetixio/synpress";
import { MetaMask, getExtensionId } from "@synthetixio/synpress/playwright";

export default defineWalletSetup(METAMASK_PASSWORD, async (context, walletPage) => {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  //@ts-ignore
  const extensionId = await getExtensionId(context, "MetaMask");
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  //@ts-ignore
  const metamask = new MetaMask(context, walletPage, METAMASK_PASSWORD, extensionId);
  await metamask.importWallet(METAMASK_SEED_PHRASE);
  await metamask.importWalletFromPrivateKey(TEST_PRIVATE_KEY);

  await metamask.openSettings();

  const SidebarMenus = metamask.homePage.selectors.settings.SettingsSidebarMenus;
  await metamask.openSidebarMenu(SidebarMenus.Advanced);

  await metamask.toggleDismissSecretRecoveryPhraseReminder();

  await metamask.addNetwork(LINEA_SEPOLIA_NETWORK);
  await metamask.switchNetwork("Sepolia", true);

  await metamask.page.click(metamask.homePage.selectors.accountMenu.accountButton);
  await metamask.page.locator(metamask.homePage.selectors.accountMenu.accountNames).last().click();
});
