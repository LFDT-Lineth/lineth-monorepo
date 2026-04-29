import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { expect } from "chai";
import { BaseContract } from "ethers";

import { expectRevertWithReason } from "./expectations";
import { buildAccessErrorMessage } from "./general";

/**
 * Interface for contracts that implement AccessControl
 */
export interface AccessControlContract extends BaseContract {
  hasRole(role: string, account: string): Promise<boolean>;
  grantRole(role: string, account: string): Promise<unknown>;
  revokeRole(role: string, account: string): Promise<unknown>;
  renounceRole(role: string, account: string): Promise<unknown>;
  getRoleAdmin(role: string): Promise<string>;
}

/**
 * Role assignment configuration for batch operations
 */
export interface RoleAssignment {
  role: string;
  account: SignerWithAddress;
}

/**
 * Verifies that an account has a specific role
 * @param contract - Contract with AccessControl
 * @param role - Role hash to check
 * @param account - Account to verify
 */
export async function expectHasRole<T extends AccessControlContract>(
  contract: T,
  role: string,
  account: SignerWithAddress | string,
): Promise<void> {
  const address = typeof account === "string" ? account : account.address;
  const hasRole = await contract.hasRole(role, address);
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
  role: string,
  account: SignerWithAddress | string,
): Promise<void> {
  const address = typeof account === "string" ? account : account.address;
  const hasRole = await contract.hasRole(role, address);
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
 * @param contract - Contract with AccessControl (connected to admin)
 * @param assignments - Array of role-account pairs to grant
 */
export async function grantRoles<T extends AccessControlContract>(
  contract: T,
  assignments: RoleAssignment[],
): Promise<void> {
  await Promise.all(assignments.map(({ role, account }) => contract.grantRole(role, account.address)));
}

/**
 * Revokes multiple roles in parallel
 * @param contract - Contract with AccessControl (connected to admin)
 * @param assignments - Array of role-account pairs to revoke
 */
export async function revokeRoles<T extends AccessControlContract>(
  contract: T,
  assignments: RoleAssignment[],
): Promise<void> {
  await Promise.all(assignments.map(({ role, account }) => contract.revokeRole(role, account.address)));
}

/**
 * Expects a transaction to revert due to missing role (AccessControl error format)
 * @param asyncCall - Promise of the transaction that should revert
 * @param account - Account attempting the action
 * @param requiredRole - Role that is required but missing
 */
export async function expectAccessControlRevert(
  asyncCall: Promise<unknown>,
  account: SignerWithAddress,
  requiredRole: string,
): Promise<void> {
  await expectRevertWithReason(asyncCall, buildAccessErrorMessage(account, requiredRole));
}

/**
 * Tests that an action succeeds with the correct role and fails without it
 * @param contract - Contract with AccessControl
 * @param authorizedAccount - Account with the required role
 * @param unauthorizedAccount - Account without the required role
 * @param requiredRole - Role required for the action
 * @param actionFn - Function that performs the action (receives connected contract)
 */
export async function testRoleBasedAccess<T extends AccessControlContract>(
  contract: T,
  authorizedAccount: SignerWithAddress,
  unauthorizedAccount: SignerWithAddress,
  requiredRole: string,
  actionFn: (connectedContract: T) => Promise<unknown>,
): Promise<void> {
  // Verify unauthorized account cannot perform action
  const unauthorizedContract = contract.connect(unauthorizedAccount) as T;
  await expectAccessControlRevert(actionFn(unauthorizedContract), unauthorizedAccount, requiredRole);

  // Verify authorized account can perform action
  const authorizedContract = contract.connect(authorizedAccount) as T;
  await actionFn(authorizedContract);
}

/**
 * Tests that granting a role enables the action and revoking disables it
 * @param contract - Contract with AccessControl (connected to admin)
 * @param account - Account to test with
 * @param role - Role to grant/revoke
 * @param actionFn - Function that performs the role-protected action
 */
export async function testRoleGrantAndRevoke<T extends AccessControlContract>(
  contract: T,
  account: SignerWithAddress,
  role: string,
  actionFn: (connectedContract: T) => Promise<unknown>,
): Promise<void> {
  // Verify action fails without role
  const connectedContract = contract.connect(account) as T;
  await expectAccessControlRevert(actionFn(connectedContract), account, role);

  // Grant role and verify action succeeds
  await contract.grantRole(role, account.address);
  await expectHasRole(contract, role, account);

  // Revoke role and verify action fails again
  await contract.revokeRole(role, account.address);
  await expectDoesNotHaveRole(contract, role, account);
  await expectAccessControlRevert(actionFn(connectedContract), account, role);
}

/**
 * Verifies the admin role for a given role
 * @param contract - Contract with AccessControl
 * @param role - Role to check admin for
 * @param expectedAdminRole - Expected admin role
 */
export async function expectRoleAdmin<T extends AccessControlContract>(
  contract: T,
  role: string,
  expectedAdminRole: string,
): Promise<void> {
  const adminRole = await contract.getRoleAdmin(role);
  expect(adminRole).to.equal(expectedAdminRole);
}
