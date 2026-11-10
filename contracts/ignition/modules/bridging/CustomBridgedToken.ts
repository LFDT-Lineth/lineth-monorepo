import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `08_deploy_CustomBridgedToken.ts` (tag `CustomBridgedToken`): implementation + pinned OZ 4.x proxy,
 * `initializeV2(name, symbol, decimals, bridge)`.
 *
 * Parameters: `name`, `symbol`, `decimals`, `bridge`.
 */
export default buildModule("CustomBridgedToken", (m) => {
  const implementation = m.contract("CustomBridgedToken", [], { id: "CustomBridgedTokenImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "CustomBridgedToken",
    unsafeAllow: ["constructor", "missing-initializer"],
    implementation,
    initializer: "initializeV2",
    initializerArgs: [
      m.getParameter<string>("name"),
      m.getParameter<string>("symbol"),
      m.getParameter("decimals"),
      m.getParameter<string>("bridge"),
    ],
  });
  const customBridgedToken = m.contractAt("CustomBridgedToken", proxy, { id: "CustomBridgedToken" });
  return {
    customBridgedToken,
    customBridgedTokenImplementation: implementation,
    customBridgedTokenProxy: proxy,
    proxyAdmin,
  };
});
