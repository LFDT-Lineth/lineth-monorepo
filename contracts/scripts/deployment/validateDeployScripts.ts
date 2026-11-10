#!/usr/bin/env tsx
/**
 * Validates the Ignition deployment setup:
 * - initializer signatures used by the modules exist in the compiled ABIs;
 * - every module under `ignition/modules/` is named after its `buildModule` id (unique, so `hardhat ignition deploy
 *   <Name>` resolves it), every `<Module>.deploy.ts` companion has its module, and no module uses `deployProxy`;
 * - registry-backed lookups in the companions resolve on known networks, and registry keys are never read straight
 *   from `process.env`.
 *
 * Run after `pnpm -F contracts run build`.
 */
import fs from "fs";
import path from "path";
import { toFunctionSignature } from "viem";

import {
  EMPTY_INITIALIZE_SIGNATURE,
  LINETH_ROLLUP_INITIALIZE_SIGNATURE,
  L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
  ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE,
  VALIDIUM_INITIALIZE_SIGNATURE,
  YIELD_MANAGER_INITIALIZE_SIGNATURE,
} from "../../common/constants/general.js";
import {
  getPopulatedAddresses,
  isMultiRegistryEntry,
  loadRegistry,
  lookupRegistryEntry,
  REGISTRY_NETWORKS,
} from "../../common/helpers/addressRegistry.js";
import { findModuleFiles } from "../../hardhat-plugins/deploy/companion.js";

import type { Abi } from "viem";

const CONTRACTS_DIR = path.join(import.meta.dirname, "..", "..");
const BUILD_DIR = path.join(CONTRACTS_DIR, "build");
const IGNITION_MODULES_DIR = path.join(CONTRACTS_DIR, "ignition", "modules");
const COMPANION_SUFFIX = ".deploy.ts";
const LOCAL_DEPLOYMENT_ARTIFACTS_DIR = path.join(import.meta.dirname, "..", "..", "local-deployments-artifacts");

type ValidationIssue = {
  category: "signature" | "registry" | "artifact" | "module";
  target: string;
  message: string;
};

type InitializerCheck = {
  label: string;
  artifactRelativePath: string;
  signature: string;
};

type RegistryLookupCheck = {
  helper: "requireAddressFromRegistryOrEnv" | "requireAddressesFromRegistryOrEnv";
  sourceFile: string;
  contractKey: string;
  envVarName: string;
};

const INITIALIZER_CHECKS: InitializerCheck[] = [
  {
    label: "LinethRollup",
    artifactRelativePath: "src/rollup/LinethRollup.sol/LinethRollup.json",
    signature: LINETH_ROLLUP_INITIALIZE_SIGNATURE,
  },
  {
    label: "Validium",
    artifactRelativePath: "src/rollup/Validium.sol/Validium.json",
    signature: VALIDIUM_INITIALIZE_SIGNATURE,
  },
  {
    label: "L2MessageService",
    artifactRelativePath: "src/messaging/l2/L2MessageService.sol/L2MessageService.json",
    signature: L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
  },
  {
    label: "RollupRevenueVault",
    artifactRelativePath: "src/operational/RollupRevenueVault.sol/RollupRevenueVault.json",
    signature: ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE,
  },
  {
    label: "YieldManager",
    artifactRelativePath: "src/yield/YieldManager.sol/YieldManager.json",
    signature: YIELD_MANAGER_INITIALIZE_SIGNATURE,
  },
  {
    label: "RecoverFunds",
    artifactRelativePath: "src/recovery/RecoverFunds.sol/RecoverFunds.json",
    signature: "initialize(address,address)",
  },
  {
    label: "CustomBridgedToken",
    artifactRelativePath: "src/bridging/token/CustomBridgedToken.sol/CustomBridgedToken.json",
    signature: "initializeV2(string,string,uint8,address)",
  },
  {
    label: "UpgradeableWithdrawalQueuePredeploy",
    artifactRelativePath:
      "src/predeploy/UpgradeableWithdrawalQueuePredeploy.sol/UpgradeableWithdrawalQueuePredeploy.json",
    signature: EMPTY_INITIALIZE_SIGNATURE,
  },
  {
    label: "UpgradeableConsolidationQueuePredeploy",
    artifactRelativePath:
      "src/predeploy/UpgradeableConsolidationQueuePredeploy.sol/UpgradeableConsolidationQueuePredeploy.json",
    signature: EMPTY_INITIALIZE_SIGNATURE,
  },
  {
    label: "UpgradeableBeaconChainDepositPredeploy",
    artifactRelativePath:
      "src/predeploy/UpgradeableBeaconChainDepositPredeploy.sol/UpgradeableBeaconChainDepositPredeploy.json",
    signature: EMPTY_INITIALIZE_SIGNATURE,
  },
];

function loadArtifactAbi(relativePath: string): Abi {
  const filePath = path.join(BUILD_DIR, relativePath);
  if (!fs.existsSync(filePath)) {
    throw new Error(`Missing artifact ${relativePath}. Run pnpm -F contracts run build first.`);
  }
  const artifact: unknown = JSON.parse(fs.readFileSync(filePath, "utf-8"));
  if (artifact === null || typeof artifact !== "object" || !("abi" in artifact) || !Array.isArray(artifact.abi)) {
    throw new Error(`Artifact ${relativePath} has no ABI.`);
  }
  const abi: Abi = artifact.abi;
  return abi;
}

function normalizeSignature(signature: string): string {
  return signature.replace(/\s+/g, "");
}

function validateInitializerSignatures(): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  for (const check of INITIALIZER_CHECKS) {
    try {
      const abi = loadArtifactAbi(check.artifactRelativePath);
      const expected = normalizeSignature(check.signature);
      const found = abi.some(
        (item) => item.type === "function" && normalizeSignature(toFunctionSignature(item)) === expected,
      );
      if (!found) {
        issues.push({
          category: "signature",
          target: check.label,
          message: `No initializer "${check.signature}" in ${check.artifactRelativePath}`,
        });
      }
    } catch (error) {
      issues.push({
        category: "signature",
        target: check.label,
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  return issues;
}

/** The env/registry inputs of deployments: the `<Module>.deploy.ts` companions. */
function loadDeployInputSources(): Array<{ fileName: string; source: string }> {
  const companions: string[] = [];
  const walk = (directory: string) => {
    for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
      const fullPath = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        walk(fullPath);
      } else if (entry.name.endsWith(COMPANION_SUFFIX)) {
        companions.push(fullPath);
      }
    }
  };
  walk(IGNITION_MODULES_DIR);
  return companions.map((fullPath) => ({
    fileName: path.relative(CONTRACTS_DIR, fullPath),
    source: fs.readFileSync(fullPath, "utf-8"),
  }));
}

function validateModuleLayout(companions: Array<{ fileName: string }>): { issues: ValidationIssue[]; count: number } {
  const issues: ValidationIssue[] = [];
  const moduleFiles = findModuleFiles(IGNITION_MODULES_DIR);
  const pathsByName = new Map<string, string[]>();

  for (const modulePath of moduleFiles) {
    const name = path.basename(modulePath, ".ts");
    const relative = path.relative(CONTRACTS_DIR, modulePath);
    pathsByName.set(name, [...(pathsByName.get(name) ?? []), relative]);
    const source = fs.readFileSync(modulePath, "utf-8");
    if (!source.includes(`buildModule("${name}"`) && !/^export default build\w+Module\(/m.test(source)) {
      issues.push({
        category: "module",
        target: relative,
        message: `The file name must be the module id: expected buildModule("${name}", ...).`,
      });
    }
    if (/deployProxy\s*\(/.test(source)) {
      issues.push({
        category: "module",
        target: relative,
        message: "Modules must deploy the pinned OZ 4.x proxies (ignition/modules/lib/proxy.ts), not deployProxy.",
      });
    }
  }

  for (const [name, paths] of pathsByName) {
    if (paths.length > 1) {
      issues.push({ category: "module", target: name, message: `Module name used twice: ${paths.join(", ")}` });
    }
  }

  for (const { fileName } of companions) {
    const modulePath = path.join(CONTRACTS_DIR, fileName.replace(COMPANION_SUFFIX, ".ts"));
    if (!fs.existsSync(modulePath)) {
      issues.push({ category: "module", target: fileName, message: "Companion without a module next to it." });
    }
  }

  return { issues, count: moduleFiles.length };
}

function loadLocalDeploymentArtifactSources(): Array<{ fileName: string; source: string }> {
  return fs
    .readdirSync(LOCAL_DEPLOYMENT_ARTIFACTS_DIR)
    .filter((fileName) => fileName.endsWith(".ts"))
    .map((fileName) => ({
      fileName,
      source: fs.readFileSync(path.join(LOCAL_DEPLOYMENT_ARTIFACTS_DIR, fileName), "utf-8"),
    }));
}

function extractRegistryLookupChecks(sources: Array<{ fileName: string; source: string }>): RegistryLookupCheck[] {
  const checks: RegistryLookupCheck[] = [];
  // Matches the parameter builders / deploy tasks (networkName, context.networkName, connection.networkName) and
  // standalone scripts that resolve the network via getDeploymentNetworkName() stored in a local variable.
  const callPattern =
    /(requireAddressFromRegistryOrEnv|requireAddressesFromRegistryOrEnv|getAddressesFromRegistryOrEnv)\(\s*(?:[\w.]*networkName|network\.name|hre\.network\.name)\s*,\s*"([^"]+)"\s*,\s*"([^"]+)"/gs;

  for (const { fileName, source } of sources) {
    for (const match of source.matchAll(callPattern)) {
      const helper = match[1]!;
      checks.push({
        helper: (helper === "getAddressesFromRegistryOrEnv"
          ? "requireAddressesFromRegistryOrEnv"
          : helper) as RegistryLookupCheck["helper"],
        sourceFile: fileName,
        contractKey: match[2]!,
        envVarName: match[3]!,
      });
    }
  }

  return checks;
}

function collectRegistryContractKeys(): Set<string> {
  const keys = new Set<string>();
  for (const network of REGISTRY_NETWORKS) {
    const registry = loadRegistry(network);
    if (!registry) {
      continue;
    }
    for (const key of Object.keys(registry.contracts)) {
      keys.add(key);
    }
  }
  return keys;
}

function isProcessEnvAssignment(source: string, matchEndIndex: number): boolean {
  return /^\s*=/.test(source.slice(matchEndIndex, matchEndIndex + 8));
}

function validateRegistryEnvUsage(sources: Array<{ fileName: string; source: string }>): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const registryKeys = collectRegistryContractKeys();
  const processEnvPattern = /process\.env(?:\.([A-Z0-9_]+)|\[\s*["']([A-Z0-9_]+)["']\s*\])/g;
  const validateAddressPattern = /validateAddressEnvVar\(\s*"([^"]+)"/g;
  const getRequiredEnvVarPattern = /getRequiredEnvVar\(\s*"([^"]+)"/g;

  for (const { fileName, source } of sources) {
    for (const match of source.matchAll(processEnvPattern)) {
      const key = match[1] ?? match[2];
      if (!key || !registryKeys.has(key) || isProcessEnvAssignment(source, match.index + match[0].length)) {
        continue;
      }
      issues.push({
        category: "registry",
        target: `${fileName}/${key}`,
        message: `Registry key "${key}" is read directly from process.env. Use requireAddressFromRegistryOrEnv/requireAddressesFromRegistryOrEnv so registered values are used on known networks.`,
      });
    }

    for (const match of source.matchAll(validateAddressPattern)) {
      const key = match[1]!;
      if (!registryKeys.has(key)) {
        continue;
      }
      issues.push({
        category: "registry",
        target: `${fileName}/${key}`,
        message: `Registry key "${key}" is validated as env-only. Use requireAddressFromRegistryOrEnv so registered values are used on known networks.`,
      });
    }

    for (const match of source.matchAll(getRequiredEnvVarPattern)) {
      const key = match[1]!;
      if (!registryKeys.has(key)) {
        continue;
      }
      issues.push({
        category: "registry",
        target: `${fileName}/${key}`,
        message: `Registry key "${key}" is read via getRequiredEnvVar. Use requireAddressFromRegistryOrEnv/requireAddressesFromRegistryOrEnv so registry values take precedence over stale env vars.`,
      });
    }
  }

  return issues;
}

function validateRegistryLookups(checks: RegistryLookupCheck[]): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  for (const network of REGISTRY_NETWORKS) {
    const registry = loadRegistry(network);
    if (!registry) {
      issues.push({
        category: "registry",
        target: network,
        message: `Registry file missing for network "${network}"`,
      });
      continue;
    }

    for (const check of checks) {
      const entry = lookupRegistryEntry(registry, check.contractKey, check.envVarName);
      if (!entry) {
        continue;
      }

      if (check.helper === "requireAddressFromRegistryOrEnv" && isMultiRegistryEntry(entry)) {
        issues.push({
          category: "registry",
          target: `${network}/${check.contractKey}`,
          message: `${check.sourceFile} uses the single-address helper, but the registry entry is an addresses array.`,
        });
        continue;
      }

      if (getPopulatedAddresses(entry) === undefined) {
        issues.push({
          category: "registry",
          target: `${network}/${check.contractKey}`,
          message: `Registry entry is zero/unpopulated but is consumed by ${check.sourceFile}; env var needed at deploy time.`,
        });
      }
    }
  }

  return issues;
}

function validateLocalDeploymentArtifacts(sources: Array<{ fileName: string; source: string }>): ValidationIssue[] {
  const issues: ValidationIssue[] = [];
  const forbiddenEnvVars = [
    "PLONKVERIFIER_ADDRESS",
    "LINETH_ROLLUP_PAUSE_TYPE_ROLES",
    "LINETH_ROLLUP_UNPAUSE_TYPE_ROLES",
    "VALIDIUM_INITIAL_STATE_ROOT_HASH",
    "VALIDIUM_INITIAL_L2_BLOCK_NUMBER",
    "VALIDIUM_SECURITY_COUNCIL",
    "VALIDIUM_GENESIS_TIMESTAMP",
  ];

  for (const { fileName, source } of sources) {
    for (const envVar of forbiddenEnvVars) {
      if (source.includes(envVar)) {
        issues.push({
          category: "artifact",
          target: `${fileName}/${envVar}`,
          message: `Local deployment artifact script uses "${envVar}", which does not match the E2E make target env vars.`,
        });
      }
    }

    if (source.includes("process.env.PRIVATE_KEY")) {
      issues.push({
        category: "artifact",
        target: `${fileName}/PRIVATE_KEY`,
        message: "Local deployment artifact scripts should use DEPLOYER_PRIVATE_KEY to match E2E make targets.",
      });
    }
  }

  return issues;
}

function main(): void {
  const deployInputSources = loadDeployInputSources();
  const localDeploymentArtifactSources = loadLocalDeploymentArtifactSources();
  const registryLookupChecks = [
    ...extractRegistryLookupChecks(deployInputSources),
    ...extractRegistryLookupChecks(localDeploymentArtifactSources),
  ];
  const signatureIssues = validateInitializerSignatures();
  const registryIssues = [
    ...validateRegistryLookups(registryLookupChecks),
    ...validateRegistryEnvUsage(deployInputSources),
    ...validateRegistryEnvUsage(localDeploymentArtifactSources),
  ];
  const artifactIssues = validateLocalDeploymentArtifacts(localDeploymentArtifactSources);
  const { issues: moduleIssues, count: moduleCount } = validateModuleLayout(deployInputSources);
  const issues = [...signatureIssues, ...moduleIssues, ...registryIssues, ...artifactIssues];

  if (issues.length === 0) {
    console.log(
      `Deploy validation passed (signatures + ${moduleCount} Ignition modules + registry usage in ${registryLookupChecks.length} lookups + local artifact scripts).`,
    );
    return;
  }

  console.error(`Deploy script validation failed with ${issues.length} issue(s):\n`);
  for (const issue of issues) {
    console.error(`- [${issue.category}] ${issue.target}: ${issue.message}`);
  }
  process.exitCode = 1;
}

main();
