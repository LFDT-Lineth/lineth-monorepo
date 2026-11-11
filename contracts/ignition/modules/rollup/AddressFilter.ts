import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { PRECOMPILES_ADDRESSES } from "../../../common/constants/precompiles.js";

/**
 * Replaces the deployment half of `21_deploy_AddressFilter.ts` (tag `AddressFilter`): `AddressFilter(deployer, precompiles)`,
 * the deployer (Ignition account 0) being the temporary `DEFAULT_ADMIN_ROLE` holder. No parameters.
 *
 * The rest of the Hardhat 2 script depends on the length of an operator-supplied file and on comparing the deployer with
 * `L1_SECURITY_COUNCIL`, which an Ignition module cannot express: `afterDeploy` in `AddressFilter.deploy.ts` sends the
 * `setFilteredStatus` batches and hands `DEFAULT_ADMIN_ROLE` to the Security Council.
 */
export default buildModule("AddressFilter", (m) => {
  const addressFilter = m.contract("AddressFilter", [m.getAccount(0), [...PRECOMPILES_ADDRESSES]], {
    id: "AddressFilter",
  });
  return { addressFilter };
});
