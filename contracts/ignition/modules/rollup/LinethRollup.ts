import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { zeroAddress } from "viem";

import { UNSAFE_ALLOW, deployPinnedTransparentProxy } from "../lib/proxy.js";

import type {
  ContractFuture,
  IgnitionModuleBuilder,
  ModuleParameterRuntimeValue,
} from "@nomicfoundation/ignition-core";

/** The Multicall3 address. */
export const LIVENESS_RECOVERY_OPERATOR = "0xcA11bde05977b3631167028862bE2a173976CA11";

/**
 * Implementation + pinned OZ 4.x proxy, initialized with `initialize(InitializationData, livenessRecoveryOperator,
 * yieldManager)`. `verifier` is the `defaultVerifier`: a parameter for the standalone module, the freshly deployed
 * verifier in `stacks/L1Protocol`.
 *
 * Parameters: `initialStateRootHash`, `initialL2BlockNumber`, `genesisTimestamp`, `rateLimitPeriodInSeconds`,
 * `rateLimitAmountInWei`, `roleAddresses`, `pauseTypeRoles`, `unpauseTypeRoles`, `securityCouncil`, `addressFilter`,
 * `yieldManager`.
 */
export function addLinethRollup(
  m: IgnitionModuleBuilder,
  verifier: ContractFuture<string> | ModuleParameterRuntimeValue<string>,
) {
  const implementation = m.contract("LinethRollup", [], { id: "LinethRollupImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "LinethRollup",
    unsafeAllow: UNSAFE_ALLOW.LinethRollup,
    implementation,
    initializer: "initialize",
    initializerArgs: [
      {
        initialStateRootHash: m.getParameter<string>("initialStateRootHash"),
        initialL2BlockNumber: m.getParameter("initialL2BlockNumber"),
        genesisTimestamp: m.getParameter("genesisTimestamp"),
        defaultVerifier: verifier,
        rateLimitPeriodInSeconds: m.getParameter("rateLimitPeriodInSeconds"),
        rateLimitAmountInWei: m.getParameter("rateLimitAmountInWei"),
        roleAddresses: m.getParameter("roleAddresses"),
        pauseTypeRoles: m.getParameter("pauseTypeRoles"),
        unpauseTypeRoles: m.getParameter("unpauseTypeRoles"),
        defaultAdmin: m.getParameter<string>("securityCouncil"),
        shnarfProvider: zeroAddress,
        addressFilter: m.getParameter<string>("addressFilter"),
      },
      LIVENESS_RECOVERY_OPERATOR,
      m.getParameter<string>("yieldManager"),
    ],
  });
  const linethRollup = m.contractAt("LinethRollup", proxy, { id: "LinethRollup" });
  return { linethRollup, linethRollupImplementation: implementation, linethRollupProxy: proxy, proxyAdmin };
}

/** Extra parameter: `verifier`. */
export default buildModule("LinethRollup", (m) => addLinethRollup(m, m.getParameter<string>("verifier")));
