import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import BridgedTokenModule from "../bridging/BridgedToken.js";
import { addTokenBridge } from "../bridging/TokenBridge.js";

/**
 * Replaces `hardhat deploy --tags BridgedToken,TokenBridge`: the TokenBridge is initialized with the freshly deployed
 * BridgedToken beacon (no in-process handoff).
 * Parameters: `TokenBridgeWithBeacon.*` (the TokenBridge parameters except `tokenBeacon`).
 */
export default buildModule("TokenBridgeWithBeacon", (m) => {
  const { bridgedTokenBeacon, bridgedTokenImplementation } = m.useModule(BridgedTokenModule);
  const tokenBridge = addTokenBridge(m, bridgedTokenBeacon);
  return { bridgedTokenBeacon, bridgedTokenImplementation, ...tokenBridge };
});
