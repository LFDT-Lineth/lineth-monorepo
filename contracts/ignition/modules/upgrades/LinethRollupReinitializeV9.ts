import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployUpgradeImplementation } from "../lib/proxy.js";

/** New LinethRollup implementation + `reinitializeLineaRollupV9` (`ProxyAdmin.upgradeAndCall` calldata). Parameters: `proxy`, `forcedTransactionFeeInWei`, `addressFilter`. */
export default buildModule("LinethRollupReinitializeV9", (m) => {
  const implementation = deployUpgradeImplementation(m, {
    contractName: "LinethRollup",
    unsafeAllow: UNSAFE_ALLOW.LinethRollup,
    reinitializer: {
      functionName: "reinitializeLineaRollupV9",
      args: [m.getParameter("forcedTransactionFeeInWei"), m.getParameter<string>("addressFilter")],
    },
  });
  return { implementation };
});
