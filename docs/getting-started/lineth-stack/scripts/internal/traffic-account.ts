import * as fs from "node:fs";
import * as path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

import { Contract, JsonRpcProvider, Wallet, isAddress } from "ethers";

import { LOCAL_L2_POLICY_DEFAULTS, envValue, parseDecimalWei, readDotEnvContents } from "./sepolia-policy";

export type TrafficAccountMode = "ensure" | "require-existing";
export type TrafficAccountSource = "env" | "artifact" | "generated";

export type TrafficTransactionReceipt = {
  hash: string;
  blockNumber: number | null;
};

export type TrafficAccountChain = {
  getEthBalance(address: string): Promise<bigint>;
  getTokenBalance(token: string, address: string): Promise<bigint>;
  sendEth(params: { from: string; to: string; value: bigint; gasPrice: bigint }): Promise<TrafficTransactionReceipt>;
  sendToken(params: {
    from: string;
    token: string;
    to: string;
    value: bigint;
    gasPrice: bigint;
  }): Promise<TrafficTransactionReceipt>;
};

export type TrafficAccountConfig = {
  mode: TrafficAccountMode;
  env: Record<string, string | undefined>;
  runtimeKeysPath: string;
  demoTrafficEnvPath: string;
  l2GasPriceWei: bigint;
  ethMinBalanceWei: bigint;
  ethTopUpWei: bigint;
  erc20?: {
    tokenAddress: string;
    minBalanceWei: bigint;
    topUpWei: bigint;
  };
  generatePrivateKey?: () => string;
  log?: (message: string) => void;
};

export type TrafficAccountResult = {
  address: string;
  source: TrafficAccountSource;
  ethBalanceWei: bigint;
  ethTopUpTx?: TrafficTransactionReceipt;
  erc20BalanceWei?: bigint;
  erc20TopUpTx?: TrafficTransactionReceipt;
};

const ERC20_ABI = [
  "function balanceOf(address) view returns (uint256)",
  "function transfer(address,uint256) returns (bool)",
] as const;

function log(config: TrafficAccountConfig, message: string) {
  config.log?.(`[traffic-account] ${message}`);
}

function die(message: string): never {
  throw new Error(`[traffic-account] ERROR: ${message}`);
}

function readEnvFile(filePath: string): Record<string, string> {
  return fs.existsSync(filePath) ? readDotEnvContents(fs.readFileSync(filePath, "utf8")) : {};
}

function requirePrivateKey(name: string, value: string | undefined): string {
  if (!value) {
    die(`${name} missing`);
  }
  if (!/^0x[a-fA-F0-9]{64}$/.test(value)) {
    die(`${name} is malformed`);
  }
  return value;
}

function requireAddress(name: string, value: string): string {
  if (!isAddress(value)) {
    die(`${name} is invalid`);
  }
  return value;
}

export function readRuntimeL2DeployerPrivateKey(runtimeKeysPath: string): string {
  if (!fs.existsSync(runtimeKeysPath)) {
    die(`${runtimeKeysPath} missing; boot the stack first`);
  }
  const runtimeKeys = readEnvFile(runtimeKeysPath);
  return requirePrivateKey("L2_DEPLOYER_PRIVATE_KEY", runtimeKeys.L2_DEPLOYER_PRIVATE_KEY);
}

async function withTrafficAccountLock<T>(demoTrafficEnvPath: string, run: () => Promise<T>): Promise<T> {
  const lockDir = `${demoTrafficEnvPath}.lock`;
  for (let attempt = 0; attempt < 50; attempt += 1) {
    try {
      fs.mkdirSync(lockDir);
      try {
        return await run();
      } finally {
        fs.rmSync(lockDir, { force: true, recursive: true });
      }
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EEXIST") {
        throw error;
      }
      await sleep(100);
    }
  }
  die(`timed out waiting for ${path.basename(lockDir)}`);
}

function writeTrafficEnvAtomic(demoTrafficEnvPath: string, privateKey: string) {
  fs.mkdirSync(path.dirname(demoTrafficEnvPath), { recursive: true });
  const tmpPath = `${demoTrafficEnvPath}.${process.pid}.tmp`;
  fs.writeFileSync(tmpPath, `L2_TRAFFIC_PRIVATE_KEY=${privateKey}\n`, { mode: 0o600 });
  fs.renameSync(tmpPath, demoTrafficEnvPath);
  fs.chmodSync(demoTrafficEnvPath, 0o600);
}

async function resolveTrafficPrivateKey(
  config: TrafficAccountConfig,
): Promise<{ privateKey: string; source: TrafficAccountSource }> {
  const envKeyNames =
    config.mode === "require-existing"
      ? ["L2_WITHDRAW_PRIVATE_KEY", "L2_TRAFFIC_PRIVATE_KEY"]
      : ["L2_TRAFFIC_PRIVATE_KEY"];

  for (const name of envKeyNames) {
    if (config.env[name]) {
      return { privateKey: requirePrivateKey(name, config.env[name]), source: "env" };
    }
  }

  const readArtifactKey = () => {
    const artifact = readEnvFile(config.demoTrafficEnvPath);
    return artifact.L2_TRAFFIC_PRIVATE_KEY
      ? {
          privateKey: requirePrivateKey("L2_TRAFFIC_PRIVATE_KEY", artifact.L2_TRAFFIC_PRIVATE_KEY),
          source: "artifact" as const,
        }
      : undefined;
  };

  const existing = readArtifactKey();
  if (existing) {
    return existing;
  }

  if (config.mode === "require-existing") {
    die("no disposable traffic account found; run the L1-to-L2 ERC20 smoke or a traffic script first");
  }

  return withTrafficAccountLock(config.demoTrafficEnvPath, async () => {
    const afterLock = readArtifactKey();
    if (afterLock) {
      return afterLock;
    }
    const privateKey = requirePrivateKey(
      "generated L2_TRAFFIC_PRIVATE_KEY",
      config.generatePrivateKey ? config.generatePrivateKey() : Wallet.createRandom().privateKey,
    );
    writeTrafficEnvAtomic(config.demoTrafficEnvPath, privateKey);
    return { privateKey, source: "generated" as const };
  });
}

export async function ensureTrafficAccount(
  config: TrafficAccountConfig,
  chain: TrafficAccountChain,
): Promise<TrafficAccountResult> {
  const l2DeployerPrivateKey = readRuntimeL2DeployerPrivateKey(config.runtimeKeysPath);
  const l2DeployerAddress = new Wallet(l2DeployerPrivateKey).address;
  const { privateKey, source } = await resolveTrafficPrivateKey(config);
  const address = new Wallet(privateKey).address;

  log(config, `${source === "generated" ? "created" : "using"} disposable account ${address} source=${source}`);
  log(config, `l2GasPriceWei=${config.l2GasPriceWei}`);

  let ethBalanceWei = await chain.getEthBalance(address);
  let ethTopUpTx: TrafficTransactionReceipt | undefined;
  if (ethBalanceWei < config.ethMinBalanceWei) {
    const deployerBalance = await chain.getEthBalance(l2DeployerAddress);
    if (deployerBalance < config.ethTopUpWei) {
      die(
        `L2 deployer ${l2DeployerAddress} has ${deployerBalance} wei; cannot top up traffic account with ${config.ethTopUpWei} wei`,
      );
    }
    log(config, `funding ETH to ${address}: value=${config.ethTopUpWei} wei`);
    ethTopUpTx = await chain.sendEth({
      from: l2DeployerAddress,
      to: address,
      value: config.ethTopUpWei,
      gasPrice: config.l2GasPriceWei,
    });
    log(config, `ETH top-up confirmed block=${ethTopUpTx.blockNumber ?? "unknown"} tx=${ethTopUpTx.hash}`);
    ethBalanceWei = await chain.getEthBalance(address);
  }

  let erc20BalanceWei: bigint | undefined;
  let erc20TopUpTx: TrafficTransactionReceipt | undefined;
  if (config.erc20) {
    const tokenAddress = requireAddress("TRAFFIC_ERC20_ADDRESS", config.erc20.tokenAddress);
    erc20BalanceWei = await chain.getTokenBalance(tokenAddress, address);
    if (erc20BalanceWei < config.erc20.minBalanceWei) {
      log(config, `funding ERC20 to ${address}: token=${tokenAddress} value=${config.erc20.topUpWei} wei`);
      erc20TopUpTx = await chain.sendToken({
        from: l2DeployerAddress,
        token: tokenAddress,
        to: address,
        value: config.erc20.topUpWei,
        gasPrice: config.l2GasPriceWei,
      });
      log(config, `ERC20 top-up confirmed block=${erc20TopUpTx.blockNumber ?? "unknown"} tx=${erc20TopUpTx.hash}`);
      erc20BalanceWei = await chain.getTokenBalance(tokenAddress, address);
    }
  }

  return {
    address,
    source,
    ethBalanceWei,
    ethTopUpTx,
    erc20BalanceWei,
    erc20TopUpTx,
  };
}

export function formatTrafficAccountOutput(result: TrafficAccountResult): string {
  const lines = [
    `TRAFFIC_ACCOUNT_ADDRESS=${result.address}`,
    `TRAFFIC_ACCOUNT_SOURCE=${result.source}`,
    `TRAFFIC_ACCOUNT_ETH_BALANCE_WEI=${result.ethBalanceWei.toString()}`,
  ];
  if (result.ethTopUpTx) {
    lines.push(`TRAFFIC_ACCOUNT_ETH_TOP_UP_TX=${result.ethTopUpTx.hash}`);
  }
  if (result.erc20BalanceWei !== undefined) {
    lines.push(`TRAFFIC_ACCOUNT_ERC20_BALANCE_WEI=${result.erc20BalanceWei.toString()}`);
  }
  if (result.erc20TopUpTx) {
    lines.push(`TRAFFIC_ACCOUNT_ERC20_TOP_UP_TX=${result.erc20TopUpTx.hash}`);
  }
  return `${lines.join("\n")}\n`;
}

class EthersTrafficChain implements TrafficAccountChain {
  private readonly readProvider: JsonRpcProvider;
  private readonly sendProvider: JsonRpcProvider;
  private readonly deployer: Wallet;

  constructor(params: { readRpcUrl: string; sendRpcUrl: string; l2DeployerPrivateKey: string }) {
    this.readProvider = new JsonRpcProvider(params.readRpcUrl);
    this.sendProvider = new JsonRpcProvider(params.sendRpcUrl);
    this.deployer = new Wallet(params.l2DeployerPrivateKey, this.sendProvider);
  }

  async getEthBalance(address: string): Promise<bigint> {
    return this.readProvider.getBalance(address);
  }

  async getTokenBalance(token: string, address: string): Promise<bigint> {
    const contract = new Contract(token, ERC20_ABI, this.readProvider);
    return (await contract.balanceOf(address)) as bigint;
  }

  async sendEth(params: {
    from: string;
    to: string;
    value: bigint;
    gasPrice: bigint;
  }): Promise<TrafficTransactionReceipt> {
    if (params.from.toLowerCase() !== this.deployer.address.toLowerCase()) {
      die(`cannot fund from ${params.from}; expected generated L2 deployer ${this.deployer.address}`);
    }
    const tx = await this.deployer.sendTransaction({
      to: params.to,
      value: params.value,
      gasLimit: 21_000n,
      gasPrice: params.gasPrice,
      type: 0,
    });
    const receipt = await tx.wait();
    if (!receipt) {
      die(`missing receipt for ETH top-up tx ${tx.hash}`);
    }
    return { hash: tx.hash, blockNumber: receipt.blockNumber };
  }

  async sendToken(params: {
    from: string;
    token: string;
    to: string;
    value: bigint;
    gasPrice: bigint;
  }): Promise<TrafficTransactionReceipt> {
    if (params.from.toLowerCase() !== this.deployer.address.toLowerCase()) {
      die(`cannot fund from ${params.from}; expected generated L2 deployer ${this.deployer.address}`);
    }
    const contract = new Contract(params.token, ERC20_ABI, this.deployer);
    const tx = await contract.transfer(params.to, params.value, {
      gasPrice: params.gasPrice,
      type: 0,
    });
    const receipt = await tx.wait();
    if (!receipt) {
      die(`missing receipt for ERC20 top-up tx ${tx.hash}`);
    }
    return { hash: tx.hash, blockNumber: receipt.blockNumber };
  }

  destroy() {
    this.readProvider.destroy();
    this.sendProvider.destroy();
  }
}

function buildConfigFromEnv(mode: TrafficAccountMode, env: Record<string, string | undefined>): TrafficAccountConfig {
  const erc20 = envValue("TRAFFIC_ERC20_ADDRESS", env)
    ? {
        tokenAddress: envValue("TRAFFIC_ERC20_ADDRESS", env),
        minBalanceWei: parseDecimalWei(
          "L2_TRAFFIC_ERC20_MIN_BALANCE_WEI",
          envValue("L2_TRAFFIC_ERC20_MIN_BALANCE_WEI", env, "100"),
        ),
        topUpWei: parseDecimalWei("L2_TRAFFIC_ERC20_TOP_UP_WEI", envValue("L2_TRAFFIC_ERC20_TOP_UP_WEI", env, "10000")),
      }
    : undefined;

  return {
    mode,
    env,
    runtimeKeysPath: envValue("RUNTIME_KEYS_ENV", env, "/accounts/runtime-keys.env"),
    demoTrafficEnvPath: envValue("DEMO_TRAFFIC_ENV", env, "/accounts/demo-traffic.env"),
    l2GasPriceWei: parseDecimalWei(
      "L2_GAS_PRICE_WEI",
      envValue("L2_GAS_PRICE_WEI", env, LOCAL_L2_POLICY_DEFAULTS.L2_GAS_PRICE_WEI),
    ),
    ethMinBalanceWei: parseDecimalWei(
      "L2_TRAFFIC_ETH_MIN_BALANCE_WEI",
      envValue("L2_TRAFFIC_ETH_MIN_BALANCE_WEI", env, "100000000000000000"),
    ),
    ethTopUpWei: parseDecimalWei(
      "L2_TRAFFIC_ETH_TOP_UP_WEI",
      envValue("L2_TRAFFIC_ETH_TOP_UP_WEI", env, "1000000000000000000"),
    ),
    erc20,
    log: (message) => console.error(message),
  };
}

async function main() {
  const mode = process.argv[2] as TrafficAccountMode | undefined;
  if (mode !== "ensure" && mode !== "require-existing") {
    die("usage: traffic-account.ts <ensure|require-existing>");
  }

  const config = buildConfigFromEnv(mode, process.env);
  const l2DeployerPrivateKey = readRuntimeL2DeployerPrivateKey(config.runtimeKeysPath);
  const chain = new EthersTrafficChain({
    readRpcUrl: envValue(
      "L2_READ_RPC_URL",
      process.env,
      envValue("L2_RPC_URL", process.env, "http://l2-node-besu:8545"),
    ),
    sendRpcUrl: envValue("L2_SEND_RPC_URL", process.env, envValue("L2_RPC_URL", process.env, "http://sequencer:8545")),
    l2DeployerPrivateKey,
  });

  try {
    const result = await ensureTrafficAccount(config, chain);
    process.stdout.write(formatTrafficAccountOutput(result));
  } finally {
    chain.destroy();
  }
}

if (require.main === module) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : error);
    process.exit(1);
  });
}
