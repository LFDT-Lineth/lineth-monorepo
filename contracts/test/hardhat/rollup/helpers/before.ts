/**
 * Define fixtures to be loaded in the 'before' block using the network helpers' 'loadFixture()' function, e.g.
 
  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin, securityCouncil, operator, nonAuthorizedAccount } = await networkHelpers.loadFixture(getAccountsFixture));
    roleAddresses = await networkHelpers.loadFixture(getRoleAddressesFixture);
  });

 */

import { LINETH_ROLLUP_V8_ROLES, VALIDIUM_ROLES } from "../../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../../common/helpers/index.js";
import { OPERATOR_ROLE } from "../../common/constants/index.js";
import { getAccountsFixture } from "../../common/helpers/before.js";

// Re-export getAccountsFixture from common/helpers for backward compatibility
export { getAccountsFixture };

export async function getRoleAddressesFixture() {
  const { securityCouncil, operator } = await getAccountsFixture();
  const roleAddresses = generateRoleAssignments(LINETH_ROLLUP_V8_ROLES, securityCouncil.address, [
    {
      role: OPERATOR_ROLE,
      addresses: [operator.address],
    },
  ]);
  return roleAddresses;
}

export async function getValidiumRoleAddressesFixture() {
  const { securityCouncil, operator } = await getAccountsFixture();
  const roleAddresses = generateRoleAssignments(VALIDIUM_ROLES, securityCouncil.address, [
    {
      role: OPERATOR_ROLE,
      addresses: [operator.address],
    },
  ]);
  return roleAddresses;
}
