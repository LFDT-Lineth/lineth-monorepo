import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

import type { IgnitionModuleBuilder } from "@nomicfoundation/ignition-core";

/**
 * Implementation + pinned OZ 4.x proxy, `initialize(rateLimitPeriod, rateLimitAmount, securityCouncil, roleAddresses,
 * pauseTypeRoles, unpauseTypeRoles)`.
 *
 * Parameters: `rateLimitPeriod`, `rateLimitAmount`, `securityCouncil`, `roleAddresses`, `pauseTypeRoles`, `unpauseTypeRoles`.
 */
export function addL2MessageService(m: IgnitionModuleBuilder) {
  const implementation = m.contract("L2MessageService", [], { id: "L2MessageServiceImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "L2MessageService",
    unsafeAllow: ["constructor"],
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
}

/** Replaces `04_deploy_L2MessageService.ts` (tag `L2MessageService`). */
export default buildModule("L2MessageService", (m) => addL2MessageService(m));
