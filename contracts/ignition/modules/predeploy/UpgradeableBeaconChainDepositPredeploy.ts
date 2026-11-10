import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/** Replaces `17_deploy_UpgradeableBeaconChainDepositPredeploy.ts` (tag `UpgradeableBeaconChainDepositPredeploy`): implementation + pinned OZ 4.x proxy, `initialize()`. No parameters. */
export default buildModule("UpgradeableBeaconChainDepositPredeploy", (m) => {
  const implementation = m.contract("UpgradeableBeaconChainDepositPredeploy", [], {
    id: "UpgradeableBeaconChainDepositPredeployImplementation",
  });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "UpgradeableBeaconChainDepositPredeploy",
    unsafeAllow: ["constructor"],
    implementation,
    initializer: "initialize",
    initializerArgs: [],
  });
  const upgradeableBeaconChainDepositPredeploy = m.contractAt("UpgradeableBeaconChainDepositPredeploy", proxy, {
    id: "UpgradeableBeaconChainDepositPredeploy",
  });
  return {
    upgradeableBeaconChainDepositPredeploy,
    upgradeableBeaconChainDepositPredeployImplementation: implementation,
    upgradeableBeaconChainDepositPredeployProxy: proxy,
    proxyAdmin,
  };
});
