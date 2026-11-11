import { expect } from "chai";

import { type TestAccount } from "./accounts.js";
import { expectRevertWithReason } from "./expectations.js";
import { buildAccessErrorMessage } from "./general.js";

import type { Abi, Address, Hash, Hex } from "viem";

/**
 * Interface for contracts that implement AccessControl
 */
export interface AccessControlContract {
  abi: Abi;
  address: Address;
  read: {
    hasRole(args: readonly [Hex, Address]): Promise<boolean>;
    getRoleAdmin(args: readonly [Hex]): Promise<Hex>;
  };
  write: {
    grantRole(args: readonly [Hex, Address]): Promise<Hash>;
    revokeRole(args: readonly [Hex, Address]): Promise<Hash>;
    renounceRole(args: readonly [Hex, Address]): Promise<Hash>;
  };
}

/**
 * Role assignment configuration for batch operations
 */
export interface RoleAssignment {
  role: Hex;
  account: TestAccount;
}

/**
 * Verifies that an account has a specific role
 * @param contract - Contract with AccessControl
 * @param role - Role hash to check
 * @param account - Account to verify
 */
export async function expectHasRole<T extends AccessControlContract>(
  contract: T,
  role: Hex,
  account: { address: Address } | Address,
): Promise<void> {
  const address = typeof account === "string" ? account : account.address;
  const hasRole = await contract.read.hasRole([role, address]);
  expect(hasRole).to.be.true;
}

/**
 * Verifies that an account does not have a specific role
 * @param contract - Contract with AccessControl
 * @param role - Role hash to check
 * @param account - Account to verify
 */
export async function expectDoesNotHaveRole<T extends AccessControlContract>(
  contract: T,
  role: Hex,
  account: { address: Address } | Address,
): Promise<void> {
  const address = typeof account === "string" ? account : account.address;
  const hasRole = await contract.read.hasRole([role, address]);
  expect(hasRole).to.be.false;
}

/**
 * Verifies that multiple role assignments are in place
 * @param contract - Contract with AccessControl
 * @param assignments - Array of role-account pairs to verify
 */
export async function expectHasRoles<T extends AccessControlContract>(
  contract: T,
  assignments: RoleAssignment[],
): Promise<void> {
  await Promise.all(assignments.map(({ role, account }) => expectHasRole(contract, role, account)));
}

/**
 * Grants multiple roles in parallel
 * @param contract - Contract with AccessControl, whose default sender holds the admin role
 * @param assignments - Array of role-account pairs to grant
 */
export async function grantRoles<T extends AccessControlContract>(
  contract: T,
  assignments: RoleAssignment[],
): Promise<void> {
  await Promise.all(assignments.map(({ role, account }) => contract.write.grantRole([role, account.address])));
}

/**
 * Revokes multiple roles in parallel
 * @param contract - Contract with AccessControl, whose default sender holds the admin role
 * @param assignments - Array of role-account pairs to revoke
 */
export async function revokeRoles<T extends AccessControlContract>(
  contract: T,
  assignments: RoleAssignment[],
): Promise<void> {
  await Promise.all(assignments.map(({ role, account }) => contract.write.revokeRole([role, account.address])));
}

/**
 * Expects a transaction to revert due to missing role (AccessControl error format)
 * @param asyncCall - Promise of the transaction that should revert
 * @param account - Account attempting the action
 * @param requiredRole - Role that is required but missing
 */
export async function expectAccessControlRevert(
  asyncCall: Promise<unknown>,
  account: TestAccount,
  requiredRole: Hex,
): Promise<void> {
  await expectRevertWithReason(asyncCall, buildAccessErrorMessage(account, requiredRole));
}

/**
 * Tests that an action succeeds with the correct role and fails without it
 * @param contract - Contract with AccessControl
 * @param authorizedAccount - Account with the required role
 * @param unauthorizedAccount - Account without the required role
 * @param requiredRole - Role required for the action
 * @param actionFn - Performs the action as the given account (e.g. `contract.write.fn([...], { account: account.account })`)
 */
export async function testRoleBasedAccess<T extends AccessControlContract>(
  contract: T,
  authorizedAccount: TestAccount,
  unauthorizedAccount: TestAccount,
  requiredRole: Hex,
  actionFn: (account: TestAccount) => Promise<unknown>,
): Promise<void> {
  // Verify unauthorized account cannot perform action
  await expectAccessControlRevert(actionFn(unauthorizedAccount), unauthorizedAccount, requiredRole);

  // Verify authorized account can perform action
  await actionFn(authorizedAccount);
}

/**
 * Tests that granting a role enables the action and revoking disables it
 * @param contract - Contract with AccessControl, whose default sender holds the admin role
 * @param account - Account to test with
 * @param role - Role to grant/revoke
 * @param actionFn - Performs the role-protected action as the given account
 */
export async function testRoleGrantAndRevoke<T extends AccessControlContract>(
  contract: T,
  account: TestAccount,
  role: Hex,
  actionFn: (account: TestAccount) => Promise<unknown>,
): Promise<void> {
  // Verify action fails without role
  await expectAccessControlRevert(actionFn(account), account, role);

  // Grant role and verify action succeeds
  await contract.write.grantRole([role, account.address]);
  await expectHasRole(contract, role, account);

  // Revoke role and verify action fails again
  await contract.write.revokeRole([role, account.address]);
  await expectDoesNotHaveRole(contract, role, account);
  await expectAccessControlRevert(actionFn(account), account, role);
}

/**
 * Verifies the admin role for a given role
 * @param contract - Contract with AccessControl
 * @param role - Role to check admin for
 * @param expectedAdminRole - Expected admin role
 */
export async function expectRoleAdmin<T extends AccessControlContract>(
  contract: T,
  role: Hex,
  expectedAdminRole: Hex,
): Promise<void> {
  const adminRole = await contract.read.getRoleAdmin([role]);
  expect(adminRole).to.equal(expectedAdminRole);
}
