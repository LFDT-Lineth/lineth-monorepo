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
 * Verifies that an account has a specific role
 * @param contract - Contract with AccessControl
 * @param role - Role hash to check
 * @param account - Account to verify
 */
export async function expectHasRole(
  contract: AccessControlContract,
  role: Hex,
  account: { address: Address } | Address,
): Promise<void> {
  const address = typeof account === "string" ? account : account.address;
  const hasRole = await contract.read.hasRole([role, address]);
  expect(hasRole).to.be.true;
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
