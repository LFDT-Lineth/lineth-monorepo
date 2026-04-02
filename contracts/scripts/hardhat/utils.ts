import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { DeployProxyOptions } from "@openzeppelin/hardhat-upgrades/dist/utils";
import { AbstractSigner, ContractFactory, JsonRpcProvider, Provider } from "ethers";
import { ethers, upgrades } from "hardhat";
import { FactoryOptions, HardhatEthersHelpers } from "hardhat/types";

import {
  clearUiWorkflowStatus,
  isSignerUiEnabled,
  resolveUiRunner,
  setUiTransactionContext,
  setUiWorkflowStatus,
} from "./signer-ui-bridge";

type RunnerOrProvider = AbstractSigner | Provider | JsonRpcProvider | HardhatEthersHelpers["provider"] | null;

function jsonSafeForUi(value: unknown): unknown {
  if (value === undefined) {
    return undefined;
  }
  if (value === null) {
    return null;
  }
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (value instanceof Uint8Array) {
    return `0x${Buffer.from(value).toString("hex")}`;
  }
  if (Array.isArray(value)) {
    return value.map((item) => jsonSafeForUi(item));
  }
  if (typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, item]) => [key, jsonSafeForUi(item)]),
    );
  }
  return value;
}

function openZeppelinProxyKindFromOpts(opts?: DeployProxyOptions): "transparent" | "uups" | "beacon" {
  const k = opts?.kind;
  if (k === "transparent" || k === "uups" || k === "beacon") {
    return k;
  }
  return "transparent";
}

function pushUiDeployContext(
  contractName: string,
  details: {
    constructorArgs?: unknown;
    initializerArgs?: unknown;
    proxyOptions?: string;
    notes?: string;
    openZeppelinProxyKind?: "transparent" | "uups" | "beacon";
  },
): void {
  if (!isSignerUiEnabled()) {
    return;
  }

  setUiTransactionContext({
    contractName,
    constructorArgs: details.constructorArgs,
    initializerArgs: details.initializerArgs,
    proxyOptions: details.proxyOptions,
    notes: details.notes,
    openZeppelinProxyKind: details.openZeppelinProxyKind,
  });
}

function tryStringifyProxyOpts(opts?: DeployProxyOptions): string | undefined {
  if (opts === undefined) {
    return undefined;
  }
  try {
    const serialized = JSON.stringify(jsonSafeForUi(opts), null, 2);
    return serialized.length > 1200 ? `${serialized.slice(0, 1200)}…` : serialized;
  } catch {
    return "(proxy options not serializable)";
  }
}

type DeploymentTxLike = {
  nonce?: number;
  hash?: string;
  gasPrice?: bigint | null;
  maxFeePerGas?: bigint | null;
  maxPriorityFeePerGas?: bigint | null;
  gasLimit?: bigint | null;
  blockNumber?: number | null;
};

type DeploymentResultLike = {
  deploymentTransaction(): DeploymentTxLike | null;
};

type WaitableDeploymentLike<T extends DeploymentResultLike> = {
  waitForDeployment(): Promise<T>;
};

function logStandardDeploymentTx(contractName: string, deployTx: DeploymentTxLike | null): void {
  console.log(`${contractName} deployment transaction has been sent, waiting...`, {
    nonce: deployTx?.nonce,
    hash: deployTx?.hash,
    gasPrice: deployTx?.gasPrice?.toString(),
    maxFeePerGas: deployTx?.maxFeePerGas?.toString(),
    maxPriorityFeePerGas: deployTx?.maxPriorityFeePerGas?.toString(),
    gasLimit: deployTx?.gasLimit?.toString(),
  });
}

function logUpgradableDeploymentTx(contractName: string, deployTx: DeploymentTxLike | null): void {
  console.log(`Upgradable ${contractName} deployment transaction has been sent, waiting...`, {
    hash: deployTx?.hash,
    gasPrice: deployTx?.gasPrice?.toString(),
    gasLimit: deployTx?.gasLimit?.toString(),
  });
}

async function waitForDeploymentWithUiWorkflow<T extends DeploymentResultLike>(
  contractName: string,
  contract: WaitableDeploymentLike<T>,
): Promise<T> {
  setUiWorkflowStatus("waiting_for_transaction_receipt", `Waiting for transaction receipt for ${contractName}.`);
  try {
    return await contract.waitForDeployment();
  } finally {
    clearUiWorkflowStatus();
  }
}

async function withUiReceiptWorkflow<T>(contractName: string, action: () => Promise<T>): Promise<T> {
  setUiWorkflowStatus("waiting_for_transaction_receipt", `Waiting for transaction receipt for ${contractName}.`);
  try {
    return await action();
  } finally {
    clearUiWorkflowStatus();
  }
}

function logStandardDeploymentComplete(contractName: string, startTime: number, deployed: DeploymentResultLike): void {
  const timeDiff = performance.now() - startTime;
  console.log(
    `${contractName} deployed: time=${timeDiff / 1000}s blockNumber=${deployed.deploymentTransaction()?.blockNumber}` +
      ` tx-hash=${deployed.deploymentTransaction()?.hash}`,
  );
}

function logUpgradableDeploymentComplete(
  contractName: string,
  startTime: number,
  deployed: DeploymentResultLike,
): void {
  const timeDiff = performance.now() - startTime;
  console.log(
    `${contractName} artifact has been deployed in ${timeDiff / 1000}s tx-hash=${deployed.deploymentTransaction()?.hash}`,
  );
}

async function deployFromFactory(
  contractName: string,
  runnerOrProvider: RunnerOrProvider = null,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: any[]
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  const runner = await resolveUiRunner(runnerOrProvider);
  if (!skipLog) {
    const signerAddress = "getAddress" in runner ? await runner.getAddress() : undefined;
    console.log(`Going to deploy ${contractName} with account ${signerAddress}...`);
  }

  const factory = await ethers.getContractFactory(contractName, runner);
  pushUiDeployContext(contractName, { constructorArgs: jsonSafeForUi(args) });
  const contract = await factory.deploy(...args);
  if (!skipLog) {
    logStandardDeploymentTx(contractName, contract.deploymentTransaction());
  }
  const afterDeploy = await waitForDeploymentWithUiWorkflow(contractName, contract);
  if (!skipLog) {
    logStandardDeploymentComplete(contractName, startTime, afterDeploy);
  }
  return contract;
}

async function deployFromFactoryWithOpts(
  contractName: string,
  runnerOrProvider: RunnerOrProvider = null,
  factoryOpts: FactoryOptions,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ...args: any[]
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  const runner = await resolveUiRunner(runnerOrProvider);
  if (!skipLog) {
    const signerAddress = "getAddress" in runner ? await runner.getAddress() : undefined;
    console.log(`Going to deploy ${contractName} with account ${signerAddress}...`);
  }

  const factory = await ethers.getContractFactory(contractName, factoryOpts);
  pushUiDeployContext(contractName, { constructorArgs: jsonSafeForUi(args) });
  const contract = await factory.connect(runner).deploy(...args);
  if (!skipLog) {
    logStandardDeploymentTx(contractName, contract.deploymentTransaction());
  }
  const afterDeploy = await waitForDeploymentWithUiWorkflow(contractName, contract);
  if (!skipLog) {
    logStandardDeploymentComplete(contractName, startTime, afterDeploy);
  }
  return contract;
}

async function deployUpgradableFromFactory(
  contractName: string,
  args?: unknown[],
  opts?: DeployProxyOptions,
  factoryOpts?: FactoryOptions,
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  const runner = await resolveUiRunner();
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory = factoryOpts
    ? await ethers.getContractFactory(contractName, factoryOpts)
    : await ethers.getContractFactory(contractName, runner);
  pushUiDeployContext(contractName, {
    initializerArgs: jsonSafeForUi(args ?? []),
    constructorArgs: jsonSafeForUi(opts?.constructorArgs),
    proxyOptions: tryStringifyProxyOpts(opts),
    openZeppelinProxyKind: openZeppelinProxyKindFromOpts(opts),
  });
  const contract = await withUiReceiptWorkflow(contractName, async () => {
    const deployed = await upgrades.deployProxy(factory.connect(runner), args, opts);
    await deployed.waitForDeployment();
    return deployed;
  });
  if (!skipLog) {
    logUpgradableDeploymentTx(contractName, contract.deploymentTransaction());
  }
  if (!skipLog) {
    logUpgradableDeploymentComplete(contractName, startTime, contract);
  }
  return contract;
}

async function deployUpgradableWithAbiAndByteCode(
  deployer: SignerWithAddress | AbstractSigner,
  contractName: string,
  abi: string,
  byteCode: string,
  args?: unknown[],
  opts?: DeployProxyOptions,
) {
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory: ContractFactory = new ContractFactory(abi, byteCode, deployer);

  pushUiDeployContext(contractName, {
    initializerArgs: jsonSafeForUi(args ?? []),
    constructorArgs: jsonSafeForUi(opts?.constructorArgs),
    proxyOptions: tryStringifyProxyOpts(opts),
    openZeppelinProxyKind: openZeppelinProxyKindFromOpts(opts),
  });
  const contract = await withUiReceiptWorkflow(contractName, async () => {
    const deployed = await upgrades.deployProxy(factory, args, opts);
    await deployed.waitForDeployment();
    return deployed;
  });

  if (!skipLog) {
    logUpgradableDeploymentTx(contractName, contract.deploymentTransaction());
  }
  if (!skipLog) {
    console.log(`${contractName} artifact has been deployed in tx-hash=${contract.deploymentTransaction()?.hash}`);
  }
  return contract;
}

async function deployUpgradableFromFactoryWithConstructorArgs(
  contractName: string,
  constructorArgs: unknown[] = [],
  initializerArgs: unknown[] = [],
  opts: DeployProxyOptions = {},
  factoryOpts?: FactoryOptions,
) {
  const startTime = performance.now();
  const skipLog = process.env.SKIP_DEPLOY_LOG === "true" || false;
  const runner = await resolveUiRunner();
  if (!skipLog) {
    console.log(`Going to deploy upgradable ${contractName}`);
  }
  const factory = factoryOpts
    ? await ethers.getContractFactory(contractName, factoryOpts)
    : await ethers.getContractFactory(contractName, runner);
  pushUiDeployContext(contractName, {
    constructorArgs: jsonSafeForUi(constructorArgs),
    initializerArgs: jsonSafeForUi(initializerArgs),
    proxyOptions: tryStringifyProxyOpts(opts),
    openZeppelinProxyKind: openZeppelinProxyKindFromOpts(opts),
  });
  const contract = await withUiReceiptWorkflow(contractName, async () => {
    const deployed = await upgrades.deployProxy(factory.connect(runner), initializerArgs, {
      ...opts,
      constructorArgs,
    });
    await deployed.waitForDeployment();
    return deployed;
  });
  if (!skipLog) {
    logUpgradableDeploymentTx(contractName, contract.deploymentTransaction());
  }
  if (!skipLog) {
    logUpgradableDeploymentComplete(contractName, startTime, contract);
  }
  return contract;
}

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable`);
  }

  return envVariable;
}

export {
  deployFromFactory,
  deployFromFactoryWithOpts,
  deployUpgradableFromFactory,
  deployUpgradableWithAbiAndByteCode,
  deployUpgradableFromFactoryWithConstructorArgs,
  requireEnv,
};
