import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedBeacon } from "../lib/proxy.js";

/** Replaces `05_deploy_BridgedToken.ts` (tag `BridgedToken`): BridgedToken behind the pinned OZ 4.x beacon. */
export default buildModule("BridgedToken", (m) => {
  const bridgedTokenImplementation = m.contract("BridgedToken", [], { id: "BridgedTokenImplementation" });
  const bridgedTokenBeacon = deployPinnedBeacon(m, {
    name: "BridgedToken",
    implementation: bridgedTokenImplementation,
  });
  return { bridgedTokenBeacon, bridgedTokenImplementation };
});
