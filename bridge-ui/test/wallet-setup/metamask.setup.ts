import {
  L1_ACCOUNT_PRIVATE_KEY,
  L2_ACCOUNT_PRIVATE_KEY,
  LOCAL_L1_NETWORK,
  LOCAL_L2_NETWORK,
  METAMASK_PASSWORD,
  METAMASK_SEED_PHRASE,
} from "../constants";
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

  // Importing the L1 account
  // Metamask name: Account 2
  await metamask.importWalletFromPrivateKey(L1_ACCOUNT_PRIVATE_KEY);
  // Importing the L2 account
  // Metamask name: Account 3
  await metamask.importWalletFromPrivateKey(L2_ACCOUNT_PRIVATE_KEY);

  await metamask.openSettings();

  const SidebarMenus = metamask.homePage.selectors.settings.SettingsSidebarMenus;
  await metamask.openSidebarMenu(SidebarMenus.Advanced);

  await metamask.toggleDismissSecretRecoveryPhraseReminder();

  await metamask.addNetwork(LOCAL_L1_NETWORK);
  await metamask.addNetwork(LOCAL_L2_NETWORK);

  await metamask.switchNetwork("Local L1 network", true);

  await metamask.switchAccount("Account 2");
});
