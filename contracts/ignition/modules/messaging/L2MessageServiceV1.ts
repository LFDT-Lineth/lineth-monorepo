import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { pinnedL2MessageServiceV1Artifact } from "../lib/pinned.js";
import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `04_deploy_L2MessageServiceV1.ts` (tag `L2MessageServiceLineaMainnet`): the Linea mainnet V1 implementation
 * from `deploy/V1/L2MessageServiceV1Deployed.json` (pinned bytecode) + pinned OZ 4.x proxy,
 * `initialize(securityCouncil, l1l2MessageSetter, rateLimitPeriod, rateLimitAmount)`.
 *
 * Parameters: `securityCouncil`, `l1l2MessageSetter`, `rateLimitPeriod`, `rateLimitAmount`.
 */
export default buildModule("L2MessageServiceV1", (m) => {
  const implementation = m.contract("L2MessageServiceLineaMainnet", pinnedL2MessageServiceV1Artifact, [], {
    id: "L2MessageServiceV1Implementation",
  });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "L2MessageServiceV1",
    implementation,
    initializer: "initialize",
    initializerArgs: [
      m.getParameter<string>("securityCouncil"),
      m.getParameter<string>("l1l2MessageSetter"),
      m.getParameter("rateLimitPeriod"),
      m.getParameter("rateLimitAmount"),
    ],
  });
  const l2MessageService = m.contractAt("L2MessageServiceLineaMainnet", pinnedL2MessageServiceV1Artifact, proxy, {
    id: "L2MessageServiceV1",
  });
  return {
    l2MessageService,
    l2MessageServiceImplementation: implementation,
    l2MessageServiceProxy: proxy,
    proxyAdmin,
  };
});
