/**
 * Define fixtures to be loaded in the 'before' block using Hardhat 'loadFixture()' function, e.g.
 
  before(async () => {
    ({ admin, securityCouncil, operator, nonAuthorizedAccount } = await loadFixture(getAccountsFixture));
    roleAddresses = await loadFixture(getRoleAddressesFixture);
  });

 */

import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { ethers } from "hardhat";
import { LINEA_ROLLUP_ROLES } from "contracts/common/constants";
import { generateRoleAssignments } from "contracts/common/helpers";
import { OPERATOR_ROLE } from "../../common/constants";

// Use in `loadFixture(getAccountsFixture))` and not as a standalone function.
// This will ensure that the same return values will be retrieved across all invocations.
export async function getAccountsFixture() {
  const [admin, securityCouncil, operator, nonAuthorizedAccount] = await ethers.getSigners();
  return { admin, securityCouncil, operator, nonAuthorizedAccount };
}

export async function getRoleAddressesFixture() {
  const { securityCouncil, operator } = await loadFixture(getAccountsFixture);
  const roleAddresses = generateRoleAssignments(LINEA_ROLLUP_ROLES, await securityCouncil.getAddress(), [
    {
      role: OPERATOR_ROLE,
      addresses: [operator.address],
    },
  ]);
  return roleAddresses;
}
