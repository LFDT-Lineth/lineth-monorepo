import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** New TokenBridge implementation for the existing proxy (`ProxyAdmin.upgrade` calldata). Parameters: `proxy`. */
export default buildModule("TokenBridgeUpgrade", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "TokenBridge",
  });
  return { implementation };
});
