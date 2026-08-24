import { JsonRpcProvider, Wallet } from "ethers";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";

import { buildAddressPlan, DeploymentStep, PlannedDeployment } from "./address-plan";
import {
  assertCheckpointCompatible,
  assertNoInFlightDeployments,
  CheckpointIdentity,
  createCheckpoint,
  DEPLOYMENT_PROFILE,
  deploymentConfigurationHash,
  DeploymentCheckpoint,
  SCHEMA_VERSION,
} from "./checkpoint";
import { DeployerConfig, resolveRoleConfig, RoleConfig } from "./config";
import { decideStepAction } from "./decision";
import { assertDeployerCanPay } from "./funds";
import { resolveGenesisTimestamp } from "./genesis";
import { runStepScript } from "./process-runner";
import { CheckpointStore } from "./store";
import {
  feeBudgetPricePerGas,
  resolveL2DeployFeeOverrides,
  resolveOneModelFeeOverrides,
} from "../../common/helpers/feeOverrides";

interface ChainContext {
  l1Provider: JsonRpcProvider;
  l2Provider: JsonRpcProvider;
  chainIds: { l1: string; l2: string };
  signers: { l1: string; l2: string };
}

const PROFILE_DEPLOY_GAS_BUDGET = 50_000_000n;

async function waitForRpc(provider: JsonRpcProvider, label: string): Promise<void> {
  const timeoutMs = Number(process.env.RPC_READY_TIMEOUT_MS ?? "300000");
  if (!Number.isSafeInteger(timeoutMs) || timeoutMs <= 0) {
    throw new Error("RPC_READY_TIMEOUT_MS must be a positive integer");
  }
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      await provider.getBlockNumber();
      return;
    } catch (error) {
      lastError = error;
      await delay(3000);
    }
  }
  throw new Error(`${label} RPC was not ready within ${timeoutMs}ms`, { cause: lastError });
}

export function assertDistinctChainIds(l1ChainId: string, l2ChainId: string): void {
  if (l1ChainId === l2ChainId) throw new Error("L1 and L2 chain IDs must differ");
}

async function createChainContext(config: DeployerConfig): Promise<ChainContext> {
  const l1Provider = new JsonRpcProvider(config.l1RpcUrl);
  const l2Provider = new JsonRpcProvider(config.l2RpcUrl);
  await Promise.all([waitForRpc(l1Provider, "L1"), waitForRpc(l2Provider, "L2")]);

  const l1Wallet = new Wallet(config.l1PrivateKey, l1Provider);
  const l2Wallet = new Wallet(config.l2PrivateKey, l2Provider);
  const [l1Network, l2Network, l1Signer, l2Signer] = await Promise.all([
    l1Provider.getNetwork(),
    l2Provider.getNetwork(),
    l1Wallet.getAddress(),
    l2Wallet.getAddress(),
  ]);
  if (config.expectedL1DeployerAddress && config.expectedL1DeployerAddress !== l1Signer) {
    throw new Error(`L1_DEPLOYER_PRIVATE_KEY does not derive L1_DEPLOYER_ADDRESS`);
  }
  if (config.expectedL2DeployerAddress && config.expectedL2DeployerAddress !== l2Signer) {
    throw new Error(`L2_DEPLOYER_PRIVATE_KEY does not derive L2_DEPLOYER_ADDRESS`);
  }
  if (config.expectedL1ChainId && config.expectedL1ChainId !== l1Network.chainId.toString()) {
    throw new Error(`L1 RPC chain ID does not match EXPECTED_L1_CHAIN_ID`);
  }
  if (config.expectedL2ChainId && config.expectedL2ChainId !== l2Network.chainId.toString()) {
    throw new Error(`L2 RPC chain ID does not match EXPECTED_L2_CHAIN_ID`);
  }
  assertDistinctChainIds(l1Network.chainId.toString(), l2Network.chainId.toString());

  return {
    l1Provider,
    l2Provider,
    chainIds: { l1: l1Network.chainId.toString(), l2: l2Network.chainId.toString() },
    signers: { l1: l1Signer, l2: l2Signer },
  };
}

function assertPlanMatchesCheckpoint(checkpoint: DeploymentCheckpoint, plan: DeploymentStep[]): void {
  const planned = plan.flatMap((step) => step.deployments);
  if (Object.keys(checkpoint.expectedDeployments).length !== planned.length) {
    throw new Error("checkpoint expected deployment count does not match the current profile");
  }
  for (const deployment of planned) {
    const expected = checkpoint.expectedDeployments[deployment.key];
    if (
      !expected ||
      expected.contractName !== deployment.contractName ||
      expected.nonce !== deployment.nonce ||
      expected.expectedAddress.toLowerCase() !== deployment.expectedAddress.toLowerCase()
    ) {
      throw new Error(`checkpoint address plan mismatch for ${deployment.key}`);
    }
  }
}

async function inspectCode(
  provider: JsonRpcProvider,
  deployments: PlannedDeployment[],
): Promise<Record<string, boolean>> {
  return Object.fromEntries(
    await Promise.all(
      deployments.map(async (deployment) => [
        deployment.key,
        (await provider.getCode(deployment.expectedAddress)) !== "0x",
      ]),
    ),
  );
}

function findDeployment(plan: DeploymentStep[], key: string): PlannedDeployment {
  const deployment = plan.flatMap((step) => step.deployments).find((item) => item.key === key);
  if (!deployment) throw new Error(`deployment plan is missing ${key}`);
  return deployment;
}

function optionalEnvironment(name: string, value: string | undefined): NodeJS.ProcessEnv {
  return value === undefined ? {} : { [name]: value };
}

function buildStepEnvironment(
  step: DeploymentStep,
  config: DeployerConfig,
  roles: RoleConfig,
  context: ChainContext,
  checkpoint: DeploymentCheckpoint,
  plan: DeploymentStep[],
): NodeJS.ProcessEnv {
  const common: NodeJS.ProcessEnv = {
    NETWORK: "custom",
    DEPLOY_FORCED_TRANSACTION_GATEWAY: "false",
    L1_SECURITY_COUNCIL: roles.l1SecurityCouncil,
    LINETH_ROLLUP_OPERATORS: roles.l1RollupOperators.join(","),
    L2_SECURITY_COUNCIL: roles.l2SecurityCouncil,
    L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER: roles.l1L2MessageSetter,
    LINETH_ROLLUP_RATE_LIMIT_PERIOD: config.rateLimitPeriod,
    LINETH_ROLLUP_RATE_LIMIT_AMOUNT: config.rateLimitAmount,
    L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD: config.rateLimitPeriod,
    L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT: config.rateLimitAmount,
    LINETH_ROLLUP_ADDRESS: findDeployment(plan, "l1-rollup.proxy").expectedAddress,
    L2_MESSAGE_SERVICE_ADDRESS: findDeployment(plan, "l2-message-service.proxy").expectedAddress,
    CONTRACT_DEPLOY_ARTIFACTS_DIR: path.join(__dirname, "artifacts", "dynamic-artifacts"),
    ...optionalEnvironment("L1_DEPLOY_GAS_PRICE_WEI", config.l1DeployGasPriceWei),
    ...optionalEnvironment("L2_DEPLOY_GAS_PRICE_WEI", config.l2DeployGasPriceWei),
  };

  if (step.id === "l1-rollup") {
    return {
      ...common,
      RPC_URL: config.l1RpcUrl,
      DEPLOYER_PRIVATE_KEY: config.l1PrivateKey,
      VERIFIER_CONTRACT_NAME: "IntegrationTestTrueVerifier",
      INITIAL_L2_STATE_ROOT_HASH: config.initialL2StateRootHash,
      INITIAL_L2_BLOCK_NUMBER: "0",
      L2_GENESIS_TIMESTAMP: checkpoint.l2GenesisTimestamp.toString(),
      L1_NONCE: step.startingNonce.toString(),
    };
  }
  if (step.id === "l2-message-service") {
    return {
      ...common,
      RPC_URL: config.l2RpcUrl,
      DEPLOYER_PRIVATE_KEY: config.l2PrivateKey,
      L2_MESSAGE_SERVICE_CONTRACT_NAME: "L2MessageService",
      L2_NONCE: step.startingNonce.toString(),
    };
  }

  const deployOnL1 = step.chain === "l1";
  return {
    ...common,
    RPC_URL: deployOnL1 ? config.l1RpcUrl : config.l2RpcUrl,
    DEPLOYER_PRIVATE_KEY: deployOnL1 ? config.l1PrivateKey : config.l2PrivateKey,
    TOKEN_BRIDGE_L1: deployOnL1 ? "true" : "false",
    [deployOnL1 ? "L1_NONCE" : "L2_NONCE"]: step.startingNonce.toString(),
    REMOTE_CHAIN_ID: deployOnL1 ? context.chainIds.l2 : context.chainIds.l1,
    REMOTE_TOKEN_BRIDGE_ADDRESS: deployOnL1
      ? findDeployment(plan, "l2-token-bridge.proxy").expectedAddress
      : findDeployment(plan, "l1-token-bridge.proxy").expectedAddress,
  };
}

function recoverStep(checkpoint: DeploymentCheckpoint, step: DeploymentStep, chainId: string): void {
  for (const deployment of step.deployments) {
    const record = checkpoint.deployments[deployment.key];
    if (!record) throw new Error(`cannot recover ${step.id} without checkpoint record ${deployment.key}`);
    if (record.chainId !== chainId) {
      throw new Error(
        `cannot recover ${step.id}; ${deployment.key} has chain ID ${record.chainId}, expected ${chainId}`,
      );
    }
  }
  if (!checkpoint.completedSteps.includes(step.id)) checkpoint.completedSteps.push(step.id);
}

export async function runDeployment(config: DeployerConfig, store: CheckpointStore): Promise<DeploymentCheckpoint> {
  const context = await createChainContext(config);
  const roles = resolveRoleConfig(config, context.signers.l1, context.signers.l2);
  const l2GenesisTimestamp = await resolveGenesisTimestamp(context.l2Provider, config.l2GenesisTimestampOverride);
  const identity: CheckpointIdentity = {
    profile: DEPLOYMENT_PROFILE,
    schemaVersion: SCHEMA_VERSION,
    artifactDigest: config.artifactDigest,
    initialL2StateRootHash: config.initialL2StateRootHash,
    l2GenesisTimestamp,
    chainIds: context.chainIds,
    signers: context.signers,
    startingNonces: {
      l1: config.l1StartingNonce,
      l2: config.l2StartingNonce,
    },
    configurationHash: deploymentConfigurationHash({
      rateLimitPeriod: config.rateLimitPeriod,
      rateLimitAmount: config.rateLimitAmount,
      roles,
    }),
  };

  let checkpoint = await store.load();
  if (checkpoint) {
    assertCheckpointCompatible(checkpoint, identity);
  } else {
    const [currentL1Nonce, currentL2Nonce] = await Promise.all([
      context.l1Provider.getTransactionCount(context.signers.l1, "pending"),
      context.l2Provider.getTransactionCount(context.signers.l2, "pending"),
    ]);
    if (currentL1Nonce !== config.l1StartingNonce) {
      throw new Error(
        `no checkpoint and L1 signer nonce is ${currentL1Nonce}, expected ${config.l1StartingNonce}; refusing deployment`,
      );
    }
    if (currentL2Nonce !== config.l2StartingNonce) {
      throw new Error(
        `no checkpoint and L2 signer nonce is ${currentL2Nonce}, expected ${config.l2StartingNonce}; refusing deployment`,
      );
    }
    checkpoint = createCheckpoint({
      ...identity,
      plan: buildAddressPlan({
        l1Signer: context.signers.l1,
        l2Signer: context.signers.l2,
        l1StartingNonce: config.l1StartingNonce,
        l2StartingNonce: config.l2StartingNonce,
      }),
    });
    await store.save(checkpoint);
  }

  const plan = buildAddressPlan({
    l1Signer: context.signers.l1,
    l2Signer: context.signers.l2,
    l1StartingNonce: checkpoint.startingNonces.l1,
    l2StartingNonce: checkpoint.startingNonces.l2,
  });
  assertPlanMatchesCheckpoint(checkpoint, plan);
  assertNoInFlightDeployments(checkpoint);

  const fundingVerified = { l1: false, l2: false };
  for (const step of plan) {
    const provider = step.chain === "l1" ? context.l1Provider : context.l2Provider;
    const signer = step.chain === "l1" ? context.signers.l1 : context.signers.l2;
    const chainId = step.chain === "l1" ? context.chainIds.l1 : context.chainIds.l2;
    const [codeByKey, currentNonce] = await Promise.all([
      inspectCode(provider, step.deployments),
      provider.getTransactionCount(signer, "pending"),
    ]);
    const decision = decideStepAction({ step, checkpoint, codeByKey, currentNonce });

    if (decision.action === "skip") {
      console.log(`Verified ${step.id}; skipping deployment`);
      continue;
    }
    if (decision.action === "recover") {
      recoverStep(checkpoint, step, chainId);
      await store.save(checkpoint);
      console.log(`Recovered ${step.id} from deterministic on-chain addresses`);
      continue;
    }

    if (!fundingVerified[step.chain]) {
      if (step.chain === "l1") {
        const [fees, balance] = await Promise.all([
          resolveOneModelFeeOverrides(context.l1Provider, "L1_DEPLOY_GAS_PRICE_WEI"),
          context.l1Provider.getBalance(context.signers.l1),
        ]);
        assertDeployerCanPay("L1", context.signers.l1, balance, fees, PROFILE_DEPLOY_GAS_BUDGET);
      } else {
        const fees = resolveL2DeployFeeOverrides();
        const balance =
          feeBudgetPricePerGas(fees) === 0n ? 0n : await context.l2Provider.getBalance(context.signers.l2);
        assertDeployerCanPay("L2", context.signers.l2, balance, fees, PROFILE_DEPLOY_GAS_BUDGET);
      }
      fundingVerified[step.chain] = true;
    }

    console.log(`Deploying ${step.id}`);
    await runStepScript({
      scriptPath: path.join(__dirname, "steps", `${step.script}.js`),
      environment: buildStepEnvironment(step, config, roles, context, checkpoint, plan),
      step,
      checkpoint,
      store,
      sensitiveValues: config.sensitiveValues,
    });
    const verifiedCode = await inspectCode(provider, step.deployments);
    if (!Object.values(verifiedCode).every(Boolean)) {
      throw new Error(`${step.id} script completed but one or more expected addresses have no bytecode`);
    }
    if (!step.deployments.every((deployment) => checkpoint.deployments[deployment.key])) {
      throw new Error(`${step.id} script completed without a durable record for every deployment`);
    }
    checkpoint.completedSteps.push(step.id);
    await store.save(checkpoint);
  }

  return checkpoint;
}
