import { ethers } from "hardhat";

/**
 * Common accounts fixture for all tests.
 * Use with `loadFixture(getAccountsFixture)` to ensure consistent signers across tests.
 *
 * Note: This is the canonical source for account fixtures. The rollup/helpers/before.ts
 * re-exports this and adds role-specific fixtures.
 */
export async function getAccountsFixture() {
  const [
    admin,
    securityCouncil,
    operator,
    nonAuthorizedAccount,
    alternateShnarfProviderAddress,
    nativeYieldOperator,
    l2YieldRecipient,
    operationalSafe,
  ] = await ethers.getSigners();
  return {
    admin,
    securityCouncil,
    operator,
    nonAuthorizedAccount,
    alternateShnarfProviderAddress,
    nativeYieldOperator,
    l2YieldRecipient,
    operationalSafe,
  };
}
