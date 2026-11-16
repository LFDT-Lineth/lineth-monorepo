import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New L2MessageService implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("L2MessageServiceUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "L2MessageService",
    unsafeAllow: UNSAFE_ALLOW.L2MessageService,
  });
  return { implementation };
});
