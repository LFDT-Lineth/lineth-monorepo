import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployUpgradeImplementation } from "../lib/proxy.js";

/** Sample: new TokenBridge implementation + `reinitializeV3()` (`ProxyAdmin.upgradeAndCall` calldata). Parameters: `proxy`. */
export default buildModule("TokenBridgeReinitializeV3", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "TokenBridge",
    reinitializer: { functionName: "reinitializeV3" },
  });
  return { implementation };
});
