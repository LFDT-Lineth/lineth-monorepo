import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Implementation + pinned OZ 4.x proxy, `initialize(rateLimitPeriod, rateLimitAmount, securityCouncil, roleAddresses,
 * pauseTypeRoles, unpauseTypeRoles)`.
 *
 * Parameters: `rateLimitPeriod`, `rateLimitAmount`, `securityCouncil`, `roleAddresses`, `pauseTypeRoles`, `unpauseTypeRoles`.
 */

export default buildModule("L2MessageService", (m) => {
  const implementation = m.contract("L2MessageService", [], { id: "L2MessageServiceImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "L2MessageService",
    unsafeAllow: UNSAFE_ALLOW.L2MessageService,
    implementation,
    initializer: "initialize",
    initializerArgs: [
      m.getParameter("rateLimitPeriod"),
      m.getParameter("rateLimitAmount"),
      m.getParameter<string>("securityCouncil"),
      m.getParameter("roleAddresses"),
      m.getParameter("pauseTypeRoles"),
      m.getParameter("unpauseTypeRoles"),
    ],
  });
  const l2MessageService = m.contractAt("L2MessageService", proxy, { id: "L2MessageService" });
  return {
    l2MessageService,
    l2MessageServiceImplementation: implementation,
    l2MessageServiceProxy: proxy,
    proxyAdmin,
  };
});
