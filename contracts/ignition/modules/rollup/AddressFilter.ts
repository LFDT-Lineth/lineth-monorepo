import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { PRECOMPILES_ADDRESSES } from "../../../common/constants/precompiles.js";

/**
 * `AddressFilter(deployer, precompiles)`, the deployer (Ignition account 0) being the temporary `DEFAULT_ADMIN_ROLE`
 * holder. No parameters. `afterDeploy` in `AddressFilter.deploy.ts` sends the `setFilteredStatus` batches (their number
 * depends on an operator-supplied file, which a module cannot express) and hands the role to the Security Council.
 */
export default buildModule("AddressFilter", (m) => {
  const addressFilter = m.contract("AddressFilter", [m.getAccount(0), [...PRECOMPILES_ADDRESSES]], {
    id: "AddressFilter",
  });
  return { addressFilter };
});
