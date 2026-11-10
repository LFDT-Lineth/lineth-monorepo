/**
 * Pinned, audited bytecode exposed as Ignition artifacts (`m.contract(name, artifact, args)`).
 *
 * - `deployments/bytecode/mainnet-proxy`: OpenZeppelin 4.x `ProxyAdmin` / `TransparentUpgradeableProxy` (v4.8.3) and
 *   `UpgradeableBeacon` (v4.4.1), solc 0.8.9, byte-identical to the mainnet fleet. Every upgradeable / beacon module
 *   deploys these, never the OpenZeppelin 5 proxies of hardhat-upgrades 3+.
 * - `deploy/V1/L2MessageServiceV1Deployed.json`: the Linea mainnet L2MessageService V1 implementation.
 * - `deployments/bytecode/2026-01-14`: YieldManager, ValidatorContainerProofVerifier, LidoStVaultYieldProviderFactory.
 */
import { isHex } from "viem";

import l2MessageServiceV1Json from "../../../deploy/V1/L2MessageServiceV1Deployed.json" with { type: "json" };
import lidoStVaultYieldProviderFactoryJson from "../../../deployments/bytecode/2026-01-14/LidoStVaultYieldProviderFactory.json" with { type: "json" };
import validatorContainerProofVerifierJson from "../../../deployments/bytecode/2026-01-14/ValidatorContainerProofVerifier.json" with { type: "json" };
import yieldManagerJson from "../../../deployments/bytecode/2026-01-14/YieldManager.json" with { type: "json" };
import proxyAdminJson from "../../../deployments/bytecode/mainnet-proxy/ProxyAdmin.json" with { type: "json" };
import transparentUpgradeableProxyJson from "../../../deployments/bytecode/mainnet-proxy/TransparentUpgradeableProxy.json" with { type: "json" };
import upgradeableBeaconJson from "../../../deployments/bytecode/mainnet-proxy/UpgradeableBeacon.json" with { type: "json" };

import type { Artifact } from "@nomicfoundation/ignition-core";
import type { Abi, Hex } from "viem";

type PinnedJson = { contractName: string; abi: readonly unknown[]; bytecode: string; deployedBytecode: string };

export type PinnedArtifact = Artifact<Abi> & { bytecode: Hex; deployedBytecode: Hex };

const ABI_ITEM_TYPES = new Set(["function", "event", "error", "constructor", "fallback", "receive"]);

function isAbi(value: readonly unknown[]): value is Abi {
  return value.every(
    (item) =>
      typeof item === "object" && item !== null && "type" in item && ABI_ITEM_TYPES.has(String(item.type)),
  );
}

function assertHex(value: string, context: string): Hex {
  if (!isHex(value)) {
    throw new Error(`${context}: expected 0x-prefixed hex bytecode`);
  }
  return value;
}

/** Builds the Ignition artifact shape (`hh3-artifact-1`) around pinned bytecode and its ABI. */
function toIgnitionArtifact(json: PinnedJson, sourceName: string, contractName = json.contractName): PinnedArtifact {
  if (!isAbi(json.abi)) {
    throw new Error(`${sourceName}:${contractName}: invalid ABI`);
  }
  return {
    _format: "hh3-artifact-1",
    contractName,
    sourceName,
    abi: json.abi,
    bytecode: assertHex(json.bytecode, `${sourceName}:${contractName} bytecode`),
    deployedBytecode: assertHex(json.deployedBytecode, `${sourceName}:${contractName} deployedBytecode`),
    linkReferences: {},
    deployedLinkReferences: {},
  };
}

export const pinnedProxyAdminArtifact = toIgnitionArtifact(proxyAdminJson, "pinned/mainnet-proxy/ProxyAdmin.sol");
export const pinnedTransparentUpgradeableProxyArtifact = toIgnitionArtifact(
  transparentUpgradeableProxyJson,
  "pinned/mainnet-proxy/TransparentUpgradeableProxy.sol",
);
export const pinnedUpgradeableBeaconArtifact = toIgnitionArtifact(
  upgradeableBeaconJson,
  "pinned/mainnet-proxy/UpgradeableBeacon.sol",
);
export const pinnedL2MessageServiceV1Artifact = toIgnitionArtifact(
  l2MessageServiceV1Json,
  "pinned/V1/L2MessageService.sol",
  "L2MessageServiceLineaMainnet",
);
export const pinnedYieldManager20260114Artifact = toIgnitionArtifact(
  yieldManagerJson,
  "pinned/2026-01-14/YieldManager.sol",
);
export const pinnedValidatorContainerProofVerifier20260114Artifact = toIgnitionArtifact(
  validatorContainerProofVerifierJson,
  "pinned/2026-01-14/ValidatorContainerProofVerifier.sol",
);
export const pinnedLidoStVaultYieldProviderFactory20260114Artifact = toIgnitionArtifact(
  lidoStVaultYieldProviderFactoryJson,
  "pinned/2026-01-14/LidoStVaultYieldProviderFactory.sol",
);
