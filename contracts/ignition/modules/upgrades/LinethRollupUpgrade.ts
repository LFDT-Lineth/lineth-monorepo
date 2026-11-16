import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New LinethRollup implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("LinethRollupUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "LinethRollup",
    unsafeAllow: UNSAFE_ALLOW.LinethRollup,
  });
  return { implementation };
});
