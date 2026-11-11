import { getTestAccounts } from "../../common/helpers/index.js";

export async function getRollupRevenueVaultAccountsFixture() {
  const [
    admin,
    invoiceSubmitter,
    burner,
    invoicePaymentReceiver,
    l1l2MessageSetter,
    l1LineaTokenBurner,
    nonAuthorizedAccount,
  ] = await getTestAccounts();
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
