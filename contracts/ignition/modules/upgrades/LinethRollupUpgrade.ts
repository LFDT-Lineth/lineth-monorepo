import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New LinethRollup implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("LinethRollupUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "LinethRollup",
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
  });
  return { implementation };
});
