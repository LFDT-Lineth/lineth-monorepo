import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New RollupRevenueVault implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("RollupRevenueVaultUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "RollupRevenueVault",
    unsafeAllow: UNSAFE_ALLOW.RollupRevenueVault,
  });
  return { implementation };
});
