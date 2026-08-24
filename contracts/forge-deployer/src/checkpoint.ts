import { getAddress, isAddress, keccak256, toUtf8Bytes } from "ethers";

import { ChainName, DeploymentStep, StepId } from "./address-plan";

export const SCHEMA_VERSION = 2;
export const DEPLOYMENT_PROFILE = "forge-dev-v1";

export interface CheckpointIdentity {
  profile: string;
  schemaVersion: number;
  artifactDigest: string;
  initialL2StateRootHash: string;
  l2GenesisTimestamp: number;
  chainIds: { l1: string; l2: string };
  signers: { l1: string; l2: string };
  startingNonces: { l1: number; l2: number };
  configurationHash: string;
}

interface DeploymentConfiguration {
  rateLimitPeriod: string;
  rateLimitAmount: string;
  roles: {
    l1SecurityCouncil: string;
    l1RollupOperators: string[];
    l2SecurityCouncil: string;
    l1L2MessageSetter: string;
  };
}

export interface ExpectedDeployment {
  stepId: StepId;
  chain: ChainName;
  contractName: string;
  nonce: number;
  expectedAddress: string;
}

export interface CompletedDeployment {
  address: string;
  transactionHash: string;
  blockNumber: number;
  chainId: string;
  recovered: boolean;
}

export interface InFlightDeployment {
  stepId: StepId;
  chain: ChainName;
  contractName: string;
  nonce: number;
  expectedAddress: string;
}

export interface DeploymentCheckpoint extends CheckpointIdentity {
  expectedDeployments: Record<string, ExpectedDeployment>;
  deployments: Record<string, CompletedDeployment>;
  inFlightDeployments: Record<string, InFlightDeployment>;
  completedSteps: StepId[];
  createdAt: string;
  updatedAt: string;
}

interface CreateCheckpointInput extends CheckpointIdentity {
  plan: DeploymentStep[];
}

export function deploymentConfigurationHash(configuration: DeploymentConfiguration): string {
  const normalized = {
    rateLimitPeriod: BigInt(configuration.rateLimitPeriod).toString(),
    rateLimitAmount: BigInt(configuration.rateLimitAmount).toString(),
    roles: {
      l1SecurityCouncil: getAddress(configuration.roles.l1SecurityCouncil),
      l1RollupOperators: configuration.roles.l1RollupOperators.map((address) => getAddress(address)),
      l2SecurityCouncil: getAddress(configuration.roles.l2SecurityCouncil),
      l1L2MessageSetter: getAddress(configuration.roles.l1L2MessageSetter),
    },
  };
  return keccak256(toUtf8Bytes(JSON.stringify(normalized)));
}

function normalizeIdentity(identity: CheckpointIdentity): CheckpointIdentity {
  return {
    ...identity,
    artifactDigest: identity.artifactDigest.toLowerCase(),
    initialL2StateRootHash: identity.initialL2StateRootHash.toLowerCase(),
    configurationHash: identity.configurationHash.toLowerCase(),
    signers: {
      l1: getAddress(identity.signers.l1),
      l2: getAddress(identity.signers.l2),
    },
  };
}

export function createCheckpoint(input: CreateCheckpointInput): DeploymentCheckpoint {
  const identity = normalizeIdentity(input);
  const now = new Date().toISOString();
  const expectedDeployments = Object.fromEntries(
    input.plan.flatMap((step) =>
      step.deployments.map((deployment) => [
        deployment.key,
        {
          stepId: step.id,
          chain: step.chain,
          contractName: deployment.contractName,
          nonce: deployment.nonce,
          expectedAddress: deployment.expectedAddress,
        },
      ]),
    ),
  );

  return {
    ...identity,
    startingNonces: input.startingNonces,
    expectedDeployments,
    deployments: {},
    inFlightDeployments: {},
    completedSteps: [],
    createdAt: now,
    updatedAt: now,
  };
}

function assertEqual(actual: string | number, expected: string | number, label: string): void {
  if (actual !== expected) throw new Error(`checkpoint ${label} mismatch: expected ${expected}, found ${actual}`);
}

export function assertCheckpointCompatible(checkpoint: DeploymentCheckpoint, identity: CheckpointIdentity): void {
  const expected = normalizeIdentity(identity);
  assertEqual(checkpoint.schemaVersion, expected.schemaVersion, "schema version");
  assertEqual(checkpoint.profile, expected.profile, "deployment profile");
  assertEqual(checkpoint.artifactDigest.toLowerCase(), expected.artifactDigest, "artifact digest");
  assertEqual(
    checkpoint.initialL2StateRootHash.toLowerCase(),
    expected.initialL2StateRootHash,
    "initial L2 state root",
  );
  assertEqual(checkpoint.l2GenesisTimestamp, expected.l2GenesisTimestamp, "L2 genesis timestamp");
  assertEqual(checkpoint.chainIds.l1, expected.chainIds.l1, "L1 chain ID");
  assertEqual(checkpoint.chainIds.l2, expected.chainIds.l2, "L2 chain ID");
  assertEqual(getAddress(checkpoint.signers.l1), expected.signers.l1, "L1 signer");
  assertEqual(getAddress(checkpoint.signers.l2), expected.signers.l2, "L2 signer");
  assertEqual(checkpoint.startingNonces.l1, expected.startingNonces.l1, "L1 starting nonce");
  assertEqual(checkpoint.startingNonces.l2, expected.startingNonces.l2, "L2 starting nonce");
  assertEqual(checkpoint.configurationHash.toLowerCase(), expected.configurationHash, "deployment configuration");
}

export function parseCheckpoint(raw: string): DeploymentCheckpoint {
  const parsed: unknown = JSON.parse(raw);
  if (!parsed || typeof parsed !== "object") throw new Error("checkpoint JSON must contain an object");
  const candidate = parsed as Partial<DeploymentCheckpoint>;
  if (
    typeof candidate.profile !== "string" ||
    typeof candidate.schemaVersion !== "number" ||
    typeof candidate.artifactDigest !== "string" ||
    typeof candidate.initialL2StateRootHash !== "string" ||
    typeof candidate.configurationHash !== "string" ||
    typeof candidate.l2GenesisTimestamp !== "number" ||
    !candidate.chainIds ||
    !candidate.signers ||
    !candidate.expectedDeployments ||
    !candidate.deployments ||
    !candidate.inFlightDeployments ||
    !Array.isArray(candidate.completedSteps)
  ) {
    throw new Error("checkpoint JSON is missing required fields");
  }
  if (
    !candidate.startingNonces ||
    !Number.isSafeInteger(candidate.startingNonces.l1) ||
    candidate.startingNonces.l1 < 0 ||
    !Number.isSafeInteger(candidate.startingNonces.l2) ||
    candidate.startingNonces.l2 < 0
  ) {
    throw new Error("checkpoint JSON has invalid or missing starting nonces");
  }
  if (!/^sha256:[0-9a-f]{64}$/.test(candidate.artifactDigest)) {
    throw new Error("checkpoint JSON has an invalid artifact digest");
  }
  for (const [key, record] of Object.entries(candidate.deployments)) {
    if (
      typeof record !== "object" ||
      record === null ||
      !isAddress(record.address) ||
      !/^0x[0-9a-fA-F]{64}$/.test(record.transactionHash) ||
      !Number.isSafeInteger(record.blockNumber) ||
      record.blockNumber < 0 ||
      typeof record.chainId !== "string" ||
      !/^[0-9]+$/.test(record.chainId) ||
      typeof record.recovered !== "boolean"
    ) {
      throw new Error(`checkpoint contains invalid deployment record ${key}`);
    }
  }
  for (const [key, intent] of Object.entries(candidate.inFlightDeployments)) {
    const expected = candidate.expectedDeployments[key];
    if (
      typeof intent !== "object" ||
      intent === null ||
      !expected ||
      intent.stepId !== expected.stepId ||
      intent.chain !== expected.chain ||
      typeof intent.contractName !== "string" ||
      intent.contractName.length === 0 ||
      intent.contractName !== expected.contractName ||
      !Number.isSafeInteger(intent.nonce) ||
      intent.nonce < 0 ||
      intent.nonce !== expected.nonce ||
      !isAddress(intent.expectedAddress) ||
      getAddress(intent.expectedAddress) !== getAddress(expected.expectedAddress)
    ) {
      throw new Error(`checkpoint contains invalid in-flight deployment ${key}`);
    }
  }
  return candidate as DeploymentCheckpoint;
}

export function touchCheckpoint(checkpoint: DeploymentCheckpoint): void {
  checkpoint.updatedAt = new Date().toISOString();
}
