/**
 * Define fixtures to be loaded in the 'before' block using Hardhat 'loadFixture()' function, e.g.
 
  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getRoleAddressesFixture);
  });

 */

import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { LINEA_ROLLUP_V8_ROLES, VALIDIUM_ROLES } from "contracts/common/constants";
import { generateRoleAssignments } from "contracts/common/helpers";

import { OPERATOR_ROLE } from "../../common/constants";
import { getAccountsFixture } from "../../common/helpers/before";

// Re-export getAccountsFixture from common/helpers for backward compatibility
export { getAccountsFixture };

export async function getRoleAddressesFixture() {
  const { securityCouncil, operator } = await loadFixture(getAccountsFixture);
  const roleAddresses = generateRoleAssignments(LINEA_ROLLUP_V8_ROLES, await securityCouncil.getAddress(), [
    {
      role: OPERATOR_ROLE,
      addresses: [operator.address],
    },
  ]);
  return roleAddresses;
}

export async function getValidiumRoleAddressesFixture() {
  const { securityCouncil, operator } = await loadFixture(getAccountsFixture);
  const roleAddresses = generateRoleAssignments(VALIDIUM_ROLES, await securityCouncil.getAddress(), [
    {
      role: OPERATOR_ROLE,
      addresses: [operator.address],
    },
  ]);
  return roleAddresses;
}
