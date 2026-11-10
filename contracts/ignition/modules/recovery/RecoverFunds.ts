import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `10_deploy_RecoverFunds.ts` (tag `RecoverFunds`): implementation + pinned OZ 4.x proxy,
 * `initialize(securityCouncil, executor)`.
 *
 * Parameters: `securityCouncil`, `executor`.
 */
export default buildModule("RecoverFunds", (m) => {
  const implementation = m.contract("RecoverFunds", [], { id: "RecoverFundsImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "RecoverFunds",
    unsafeAllow: ["constructor"],
    implementation,
    initializer: "initialize",
    initializerArgs: [m.getParameter<string>("securityCouncil"), m.getParameter<string>("executor")],
  });
  const recoverFunds = m.contractAt("RecoverFunds", proxy, { id: "RecoverFunds" });
  return { recoverFunds, recoverFundsImplementation: implementation, recoverFundsProxy: proxy, proxyAdmin };
});
