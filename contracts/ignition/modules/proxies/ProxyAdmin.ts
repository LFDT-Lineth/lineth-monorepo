import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { pinnedProxyAdminArtifact } from "../lib/pinned.js";

/**
 * Shared OpenZeppelin 4.x `ProxyAdmin` (pinned mainnet bytecode), owned by the deployer.
 *
 * Every upgradeable module reaches it through `m.useModule(ProxyAdminModule)`, so all proxies deployed into the same
 * Ignition deployment id share one admin, like the shared admin the OZ Hardhat 2 plugin kept in `.openzeppelin/<network>.json`.
 * A new deployment id deploys a new admin.
 */
export default buildModule("ProxyAdmin", (m) => {
  const proxyAdmin = m.contract("ProxyAdmin", pinnedProxyAdminArtifact, [], { id: "ProxyAdmin" });
  return { proxyAdmin };
});
