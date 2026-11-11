import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/** Replaces `14_deploy_UpgradeableWithdrawalQueuePredeploy.ts` (tag `UpgradeableWithdrawalQueuePredeploy`): implementation + pinned OZ 4.x proxy, `initialize()`. No parameters. */
export default buildModule("UpgradeableWithdrawalQueuePredeploy", (m) => {
  const implementation = m.contract("UpgradeableWithdrawalQueuePredeploy", [], {
    id: "UpgradeableWithdrawalQueuePredeployImplementation",
  });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "UpgradeableWithdrawalQueuePredeploy",
    unsafeAllow: ["constructor"],
    implementation,
    initializer: "initialize",
    initializerArgs: [],
  });
  const upgradeableWithdrawalQueuePredeploy = m.contractAt("UpgradeableWithdrawalQueuePredeploy", proxy, {
    id: "UpgradeableWithdrawalQueuePredeploy",
  });
  return {
    upgradeableWithdrawalQueuePredeploy,
    upgradeableWithdrawalQueuePredeployImplementation: implementation,
    upgradeableWithdrawalQueuePredeployProxy: proxy,
    proxyAdmin,
  };
});
