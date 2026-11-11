import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/** Replaces `15_deploy_UpgradeableConsolidationQueuePredeploy.ts` (tag `UpgradeableConsolidationQueuePredeploy`): implementation + pinned OZ 4.x proxy, `initialize()`. No parameters. */
export default buildModule("UpgradeableConsolidationQueuePredeploy", (m) => {
  const implementation = m.contract("UpgradeableConsolidationQueuePredeploy", [], {
    id: "UpgradeableConsolidationQueuePredeployImplementation",
  });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "UpgradeableConsolidationQueuePredeploy",
    unsafeAllow: ["constructor"],
    implementation,
    initializer: "initialize",
    initializerArgs: [],
  });
  const upgradeableConsolidationQueuePredeploy = m.contractAt("UpgradeableConsolidationQueuePredeploy", proxy, {
    id: "UpgradeableConsolidationQueuePredeploy",
  });
  return {
    upgradeableConsolidationQueuePredeploy,
    upgradeableConsolidationQueuePredeployImplementation: implementation,
    upgradeableConsolidationQueuePredeployProxy: proxy,
    proxyAdmin,
  };
});
