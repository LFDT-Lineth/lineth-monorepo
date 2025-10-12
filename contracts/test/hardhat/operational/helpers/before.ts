import { ethers } from "hardhat";

export async function getRollupRevenueVaultAccountsFixture() {
  const [
    admin,
    invoiceSubmitter,
    burner,
    invoicePaymentReceiver,
    l1l2MessageSetter,
    l1LineaTokenBurner,
    nonAuthorizedAccount,
  ] = await ethers.getSigners();
  return {
    admin,
    invoiceSubmitter,
    burner,
    invoicePaymentReceiver,
    l1l2MessageSetter,
    l1LineaTokenBurner,
    nonAuthorizedAccount,
  };
}
