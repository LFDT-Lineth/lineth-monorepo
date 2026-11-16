import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New YieldManager implementation (constructor `linethRollup`) for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`, `linethRollup`. */
export default buildModule("YieldManagerUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "YieldManager",
    unsafeAllow: UNSAFE_ALLOW.YieldManager,
    constructorArgs: [m.getParameter<string>("linethRollup")],
  });
  return { implementation };
});
