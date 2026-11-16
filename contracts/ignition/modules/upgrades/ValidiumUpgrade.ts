import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New Validium implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("ValidiumUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "Validium",
    unsafeAllow: UNSAFE_ALLOW.Validium,
  });
  return { implementation };
});
