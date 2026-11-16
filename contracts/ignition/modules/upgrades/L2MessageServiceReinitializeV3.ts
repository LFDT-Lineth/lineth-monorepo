import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** Sample: new L2MessageService implementation + `reinitializeV3()` (`ProxyAdmin.upgradeAndCall` calldata). Parameters: `proxy`. */
export default buildModule("L2MessageServiceReinitializeV3", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "L2MessageService",
    unsafeAllow: UNSAFE_ALLOW.L2MessageService,
    reinitializer: { functionName: "reinitializeV3" },
  });
  return { implementation };
});
