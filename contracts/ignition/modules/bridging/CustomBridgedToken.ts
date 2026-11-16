import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { UNSAFE_ALLOW, deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Implementation + pinned OZ 4.x proxy,
 * `initializeV2(name, symbol, decimals, bridge)`.
 *
 * Parameters: `name`, `symbol`, `decimals`, `bridge`.
 */
export default buildModule("CustomBridgedToken", (m) => {
  const implementation = m.contract("CustomBridgedToken", [], { id: "CustomBridgedTokenImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "CustomBridgedToken",
    unsafeAllow: UNSAFE_ALLOW.CustomBridgedToken,
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
