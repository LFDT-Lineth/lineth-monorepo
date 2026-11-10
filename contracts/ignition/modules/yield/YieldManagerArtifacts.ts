import { buildModule } from "@nomicfoundation/hardhat-ignition/modules";

import {
  pinnedLidoStVaultYieldProviderFactory20260114Artifact,
  pinnedValidatorContainerProofVerifier20260114Artifact,
  pinnedYieldManager20260114Artifact,
} from "../lib/pinned.js";
import { deployPinnedTransparentProxy } from "../lib/proxy.js";

/**
 * Replaces `21_deploy_YieldManagerArtifacts.ts` (tag `YieldManagerArtifacts`): same graph as `YieldManager.ts`, but every
 * contract is the audited bytecode pinned in `deployments/bytecode/2026-01-14` (verify from git tag
 * `contract-audit-2026-01-14`, commit 25e323d055dec40ef167a190c71c30aa9bf92c23).
 *
 * Parameters: `linethRollup`, `yieldManagerInitializationData`, `verifierAdmin`, `gIFirstValidator`,
 * `gIPendingPartialWithdrawalsRoot`, `vaultHub`, `vaultFactory`, `steth`.
 */
export default buildModule("YieldManagerArtifacts", (m) => {
  const linethRollup = m.getParameter<string>("linethRollup");

  const implementation = m.contract("YieldManager", pinnedYieldManager20260114Artifact, [linethRollup], {
    id: "YieldManagerImplementation",
  });
  const { proxy, proxyAdmin } = deployPinnedTransparentProxy(m, {
    name: "YieldManager",
    implementation,
    initializer: "initialize",
    initializerArgs: [m.getParameter("yieldManagerInitializationData")],
  });
  const yieldManager = m.contractAt("YieldManager", pinnedYieldManager20260114Artifact, proxy, { id: "YieldManager" });

  const validatorContainerProofVerifier = m.contract(
    "ValidatorContainerProofVerifier",
    pinnedValidatorContainerProofVerifier20260114Artifact,
    [
      m.getParameter<string>("verifierAdmin"),
      m.getParameter<string>("gIFirstValidator"),
      m.getParameter<string>("gIPendingPartialWithdrawalsRoot"),
    ],
    { id: "ValidatorContainerProofVerifier" },
  );

  const lidoStVaultYieldProviderFactory = m.contract(
    "LidoStVaultYieldProviderFactory",
    pinnedLidoStVaultYieldProviderFactory20260114Artifact,
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
  // The provider is created by the pinned factory; the compiled ABI is only used to address it (nothing is deployed).
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
