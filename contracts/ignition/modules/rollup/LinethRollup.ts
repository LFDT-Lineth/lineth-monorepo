import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { zeroAddress } from "viem";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

import type {
  ContractFuture,
  IgnitionModuleBuilder,
  ModuleParameterRuntimeValue,
} from "@nomicfoundation/ignition-core";

/** Hard-coded in the Hardhat 2 script as well (Multicall3 address). */
export const LIVENESS_RECOVERY_OPERATOR = "0xcA11bde05977b3631167028862bE2a173976CA11";

/**
 * Implementation + pinned OZ 4.x proxy, initialized with `initialize(InitializationData, livenessRecoveryOperator,
 * yieldManager)`. `verifier` is the `defaultVerifier`: a parameter for the standalone module, the freshly deployed
 * verifier in `chains/L1Protocol`.
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
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
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

/** Replaces `03_deploy_LinethRollup.ts` (tag `LinethRollup`). Extra parameter: `verifier`. */
export default buildModule("LinethRollup", (m) => addLinethRollup(m, m.getParameter<string>("verifier")));
