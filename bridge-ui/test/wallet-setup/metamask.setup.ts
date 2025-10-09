import { Locator, Page } from "@playwright/test";
import {
  L1_ACCOUNT_ADDRESS,
  L1_ACCOUNT_PRIVATE_KEY,
  L2_ACCOUNT_PRIVATE_KEY,
  LOCAL_L1_NETWORK,
  LOCAL_L2_NETWORK,
  METAMASK_PASSWORD,
  METAMASK_SEED_PHRASE,
} from "../constants";
import { defineWalletSetup } from "@synthetixio/synpress";
import { MetaMask, getExtensionId } from "@synthetixio/synpress/playwright";
import { z } from "zod";

const closeRenameAccountButtonSelector = 'button[aria-label="Close"]';

export default defineWalletSetup(METAMASK_PASSWORD, async (context, walletPage) => {
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  //@ts-ignore
  const extensionId = await getExtensionId(context, "MetaMask");
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  //@ts-ignore
  const metamask = new MetaMask(context, walletPage, METAMASK_PASSWORD, extensionId);
  await metamask.importWallet(METAMASK_SEED_PHRASE);

  // Importing the L1 account
  await metamask.importWalletFromPrivateKey(L1_ACCOUNT_PRIVATE_KEY);
  // Importing the L2 account
  await metamask.importWalletFromPrivateKey(L2_ACCOUNT_PRIVATE_KEY);

  await metamask.openSettings();

  const SidebarMenus = metamask.homePage.selectors.settings.SettingsSidebarMenus;
  await metamask.openSidebarMenu(SidebarMenus.Advanced);

  await metamask.toggleDismissSecretRecoveryPhraseReminder();

  await metamask.addNetwork(LOCAL_L1_NETWORK);
  await metamask.addNetwork(LOCAL_L2_NETWORK);

  await metamask.switchNetwork("Local L1 network", true);

  const l1AccountIndex = await findAccountIndexByAddress(metamask, L1_ACCOUNT_ADDRESS);
  const l2AccountIndex = l1AccountIndex + 1;

  await metamask.renameAccount(`Account ${l1AccountIndex}`, "L1 Account");
  await metamask.homePage.page.locator(closeRenameAccountButtonSelector).click();
  await metamask.renameAccount(`Account ${l2AccountIndex}`, "L2 Account");
  await metamask.homePage.page.locator(closeRenameAccountButtonSelector).click();

  await metamask.switchAccount("L1 Account");
});

export async function findAccountIndexByAddress(metamask: MetaMask, accountAddress: string) {
  const accountNames = await getAllAcccountNames(
    metamask.homePage.page,
    metamask.homePage.selectors.accountMenu.accountButton,
    metamask.homePage.selectors.accountMenu.accountNames,
  );

  for (let i = 0; i < accountNames.length; i++) {
    await metamask.switchAccount(`Account ${i + 1}`);
    const currentAddress = await metamask.getAccountAddress();
    if (currentAddress.toLowerCase() === accountAddress.toLowerCase()) {
      return i + 1;
    }
  }

  throw new Error(`Account with address ${accountAddress} not found`);
}

async function getAllAcccountNames(page: Page, accountButtonSelector: string, accountMenuAccountNamesSelector: string) {
  const menuLocator = page.locator(accountButtonSelector);
  await menuLocator.click();
  const accountNamesLocators = await page.locator(accountMenuAccountNamesSelector).all();

  const accountNames = await allTextContents(accountNamesLocators);

  const accountIndex = accountNames.length - 1;

  // Click on the last account to close the menu
  await accountNamesLocators[accountIndex]!.click();

  return accountNames;
}

export async function allTextContents(locators: Locator[]) {
  const names = await Promise.all(locators.map((locator) => locator.textContent()));
  return names.map((name) => z.string().parse(name));
}
