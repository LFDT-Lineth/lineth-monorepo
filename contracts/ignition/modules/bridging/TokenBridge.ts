import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

import type {
  ContractFuture,
  IgnitionModuleBuilder,
  ModuleParameterRuntimeValue,
} from "@nomicfoundation/ignition-core";

/**
 * Implementation + pinned OZ 4.x proxy, `initialize(InitializationData)`. `tokenBeacon` is a parameter for the standalone
 * module and the freshly deployed beacon in `chains/TokenBridgeWithBeacon`.
 *
 * Parameters: `securityCouncil`, `messageService`, `sourceChainId`, `targetChainId`, `remoteSender`, `reservedTokens`,
 * `roleAddresses`, `pauseTypeRoles`, `unpauseTypeRoles`.
 */
export function addTokenBridge(
  m: IgnitionModuleBuilder,
  tokenBeacon: ContractFuture<string> | ModuleParameterRuntimeValue<string>,
) {
  const implementation = m.contract("TokenBridge", [], { id: "TokenBridgeImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "TokenBridge",
    implementation,
    initializer: "initialize",
    initializerArgs: [
      {
        defaultAdmin: m.getParameter<string>("securityCouncil"),
        messageService: m.getParameter<string>("messageService"),
        tokenBeacon,
        sourceChainId: m.getParameter("sourceChainId"),
        targetChainId: m.getParameter("targetChainId"),
        remoteSender: m.getParameter<string>("remoteSender"),
        reservedTokens: m.getParameter("reservedTokens"),
        roleAddresses: m.getParameter("roleAddresses"),
        pauseTypeRoles: m.getParameter("pauseTypeRoles"),
        unpauseTypeRoles: m.getParameter("unpauseTypeRoles"),
      },
    ],
  });
  const tokenBridge = m.contractAt("TokenBridge", proxy, { id: "TokenBridge" });
  return { tokenBridge, tokenBridgeImplementation: implementation, tokenBridgeProxy: proxy, proxyAdmin };
}

/** Replaces `06_deploy_TokenBridge.ts` (tag `TokenBridge`). Extra parameter: `tokenBeacon`. */
export default buildModule("TokenBridge", (m) => addTokenBridge(m, m.getParameter<string>("tokenBeacon")));
