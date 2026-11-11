import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New L2MessageService implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("L2MessageServiceUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "L2MessageService",
    unsafeAllow: ["constructor"],
  });
  return { implementation };
});
