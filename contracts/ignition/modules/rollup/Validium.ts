import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { zeroAddress } from "viem";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `03_deploy_Validium.ts` (tag `Validium`): implementation + pinned OZ 4.x proxy, `initialize(InitializationData)`.
 *
 * Parameters: `initialStateRootHash`, `initialL2BlockNumber`, `genesisTimestamp`, `verifier`, `rateLimitPeriodInSeconds`,
 * `rateLimitAmountInWei`, `roleAddresses`, `pauseTypeRoles`, `unpauseTypeRoles`, `securityCouncil`, `addressFilter`.
 */
export default buildModule("Validium", (m) => {
  const implementation = m.contract("Validium", [], { id: "ValidiumImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "Validium",
    unsafeAllow: ["constructor", "incorrect-initializer-order"],
    implementation,
    initializer: "initialize",
    initializerArgs: [
      {
        initialStateRootHash: m.getParameter<string>("initialStateRootHash"),
        initialL2BlockNumber: m.getParameter("initialL2BlockNumber"),
        genesisTimestamp: m.getParameter("genesisTimestamp"),
        defaultVerifier: m.getParameter<string>("verifier"),
        rateLimitPeriodInSeconds: m.getParameter("rateLimitPeriodInSeconds"),
        rateLimitAmountInWei: m.getParameter("rateLimitAmountInWei"),
        roleAddresses: m.getParameter("roleAddresses"),
        pauseTypeRoles: m.getParameter("pauseTypeRoles"),
        unpauseTypeRoles: m.getParameter("unpauseTypeRoles"),
        defaultAdmin: m.getParameter<string>("securityCouncil"),
        shnarfProvider: zeroAddress,
        addressFilter: m.getParameter<string>("addressFilter"),
      },
    ],
  });
  const validium = m.contractAt("Validium", proxy, { id: "Validium" });
  return { validium, validiumImplementation: implementation, validiumProxy: proxy, proxyAdmin };
});
