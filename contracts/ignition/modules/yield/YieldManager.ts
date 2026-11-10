import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";
import { zeroAddress } from "viem";

import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `21_deploy_YieldManager.ts` (tag `YieldManager`): YieldManager (implementation with constructor `linethRollup` +
 * pinned OZ 4.x proxy, `initialize(YieldManagerInitializationData)`), ValidatorContainerProofVerifier,
 * LidoStVaultYieldProviderFactory, and one LidoStVaultYieldProvider created through the factory.
 *
 * Parameters: `linethRollup`, `yieldManagerInitializationData`, `verifierAdmin`, `gIFirstValidator`,
 * `gIPendingPartialWithdrawalsRoot`, `vaultHub`, `vaultFactory`, `steth`.
 */
export default buildModule("YieldManager", (m) => {
  const linethRollup = m.getParameter<string>("linethRollup");

  const implementation = m.contract("YieldManager", [linethRollup], { id: "YieldManagerImplementation" });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "YieldManager",
    unsafeAllow: ["constructor", "incorrect-initializer-order", "state-variable-immutable", "delegatecall"],
    validationConstructorArgs: [zeroAddress],
    implementation,
    initializer: "initialize",
    initializerArgs: [m.getParameter("yieldManagerInitializationData")],
  });
  const yieldManager = m.contractAt("YieldManager", proxy, { id: "YieldManager" });

  const validatorContainerProofVerifier = m.contract(
    "ValidatorContainerProofVerifier",
    [
      m.getParameter<string>("verifierAdmin"),
      m.getParameter<string>("gIFirstValidator"),
      m.getParameter<string>("gIPendingPartialWithdrawalsRoot"),
    ],
    { id: "ValidatorContainerProofVerifier" },
  );

  const lidoStVaultYieldProviderFactory = m.contract(
    "LidoStVaultYieldProviderFactory",
    [
      linethRollup,
      proxy,
      m.getParameter<string>("vaultHub"),
      m.getParameter<string>("vaultFactory"),
      m.getParameter<string>("steth"),
      validatorContainerProofVerifier,
    ],
    { id: "LidoStVaultYieldProviderFactory" },
  );

  const createYieldProvider = m.call(lidoStVaultYieldProviderFactory, "createLidoStVaultYieldProvider", [], {
    id: "CreateLidoStVaultYieldProvider",
  });
  const yieldProviderAddress = m.readEventArgument(
    createYieldProvider,
    "LidoStVaultYieldProviderCreated",
    "providerAddress",
    { id: "LidoStVaultYieldProviderAddress", emitter: lidoStVaultYieldProviderFactory },
  );
  const lidoStVaultYieldProvider = m.contractAt("LidoStVaultYieldProvider", yieldProviderAddress, {
    id: "LidoStVaultYieldProvider",
  });

  return {
    yieldManager,
    yieldManagerImplementation: implementation,
    yieldManagerProxy: proxy,
    proxyAdmin,
    validatorContainerProofVerifier,
    lidoStVaultYieldProviderFactory,
    lidoStVaultYieldProvider,
  };
});
