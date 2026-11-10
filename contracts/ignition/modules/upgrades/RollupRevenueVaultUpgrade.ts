import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New RollupRevenueVault implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("RollupRevenueVaultUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "RollupRevenueVault",
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  });
  return { implementation };
});
