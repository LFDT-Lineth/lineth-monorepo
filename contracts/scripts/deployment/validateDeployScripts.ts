#!/usr/bin/env ts-node
/**
 * Validates deploy script initializer signatures against compiled contract ABIs and
 * checks that registry-backed lookups used by deploy scripts resolve on known networks.
 *
 * Run after `pnpm -F contracts run build`.
 */
import { Interface, InterfaceAbi } from "ethers";
import fs from "fs";
import path from "path";

import {
  EMPTY_INITIALIZE_SIGNATURE,
  LINEA_ROLLUP_INITIALIZE_SIGNATURE,
  L2_MESSAGE_SERVICE_INITIALIZE_SIGNATURE,
  ROLLUP_REVENUE_VAULT_INITIALIZE_SIGNATURE,
  VALIDIUM_INITIALIZE_SIGNATURE,
  YIELD_MANAGER_INITIALIZE_SIGNATURE,
} from "../../common/constants/general";
import {
  getPopulatedAddresses,
  isMultiRegistryEntry,
  loadRegistry,
  lookupRegistryEntry,
  REGISTRY_NETWORKS,
} from "../../common/helpers/addressRegistry";

const BUILD_DIR = path.join(__dirname, "..", "..", "build");
const DEPLOY_DIR = path.join(__dirname, "..", "..", "deploy");
const LOCAL_DEPLOYMENT_ARTIFACTS_DIR = path.join(__dirname, "..", "..", "local-deployments-artifacts");

type ValidationIssue = {
  category: "signature" | "registry" | "artifact";
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
    label: "LineaRollup",
    artifactRelativePath: "src/rollup/LineaRollup.sol/LineaRollup.json",
    signature: LINEA_ROLLUP_INITIALIZE_SIGNATURE,
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

function loadArtifact(relativePath: string): { abi: InterfaceAbi } {
  const filePath = path.join(BUILD_DIR, relativePath);
  if (!fs.existsSync(filePath)) {
    throw new Error(`Missing artifact ${relativePath}. Run pnpm -F contracts run build first.`);
  }
  return JSON.parse(fs.readFileSync(filePath, "utf-8")) as { abi: InterfaceAbi };
}

function validateInitializerSignatures(): ValidationIssue[] {
  const issues: ValidationIssue[] = [];

  for (const check of INITIALIZER_CHECKS) {
    try {
      const artifact = loadArtifact(check.artifactRelativePath);
      const iface = new Interface(artifact.abi);
      const fn = iface.getFunction(check.signature);
      if (!fn) {
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

function loadDeployScriptSources(): Array<{ fileName: string; source: string }> {
  return fs
    .readdirSync(DEPLOY_DIR)
    .filter((fileName) => fileName.endsWith(".ts"))
    .map((fileName) => ({
      fileName,
      source: fs.readFileSync(path.join(DEPLOY_DIR, fileName), "utf-8"),
    }));
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
  // Matches both Hardhat scripts (network.name / hre.network.name) and standalone scripts
  // that resolve the network via getDeploymentNetworkName() stored in a local variable.
  const callPattern =
    /(requireAddressFromRegistryOrEnv|requireAddressesFromRegistryOrEnv|getAddressesFromRegistryOrEnv)\(\s*(?:networkName|network\.name|hre\.network\.name)\s*,\s*"([^"]+)"\s*,\s*"([^"]+)"/gs;

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
    "LINEA_ROLLUP_PAUSE_TYPE_ROLES",
    "LINEA_ROLLUP_UNPAUSE_TYPE_ROLES",
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
  const deployScriptSources = loadDeployScriptSources();
  const localDeploymentArtifactSources = loadLocalDeploymentArtifactSources();
  const registryLookupChecks = [
    ...extractRegistryLookupChecks(deployScriptSources),
    ...extractRegistryLookupChecks(localDeploymentArtifactSources),
  ];
  const signatureIssues = validateInitializerSignatures();
  const registryIssues = [
    ...validateRegistryLookups(registryLookupChecks),
    ...validateRegistryEnvUsage(deployScriptSources),
    ...validateRegistryEnvUsage(localDeploymentArtifactSources),
  ];
  const artifactIssues = validateLocalDeploymentArtifacts(localDeploymentArtifactSources);
  const issues = [...signatureIssues, ...registryIssues, ...artifactIssues];

  if (issues.length === 0) {
    console.log("Deploy script validation passed (signatures + registry usage + local artifact scripts).");
    return;
  }

  console.error(`Deploy script validation failed with ${issues.length} issue(s):\n`);
  for (const issue of issues) {
    console.error(`- [${issue.category}] ${issue.target}: ${issue.message}`);
  }
  process.exitCode = 1;
}

main();
