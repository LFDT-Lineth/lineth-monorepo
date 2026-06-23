import * as fs from "node:fs";
import * as path from "node:path";

import { encryptKeystoreJson, Wallet } from "ethers";

export type EnvMap = Record<string, string | undefined>;

export type L1Mode = "sepolia" | "local";
export type L1Context = "host" | "container";
export type L1DeployerSource = "local-genesis" | "generated-keystore" | "provided-keystore" | "legacy-raw-key";

export type L1Config = {
  mode: L1Mode;
  rpcUrl: string;
  deployerPrivateKey: string;
};

export type L1DeployerConfig = L1Config & {
  address: string;
  privateKey: string;
  deployerPrivateKey: string;
  source: L1DeployerSource;
  created: boolean;
  keystorePath?: string;
  passwordFilePath?: string;
};

function sanitizeExternalError(error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return message
    .replace(/https?:\/\/[^\s)"']+/g, "<redacted-url>")
    .replace(/0x[a-fA-F0-9]{64}/g, "<redacted-hex>");
}

export type ResolveL1DeployerOptions = {
  stackDir?: string;
  accountsDir?: string;
};

export const L1_MODE_VALUES = ["sepolia", "local"] as const;
export const LOCAL_L1_CHAIN_ID = 31648428n;
export const LOCAL_L1_CONTAINER_RPC_URL = "http://l1-el-node:8545";
export const DEFAULT_LOCAL_L1_HOST_RPC_PORT = "8445";
export const LOCAL_L1_HOST_RPC_URL = `http://localhost:${DEFAULT_LOCAL_L1_HOST_RPC_PORT}`;
export const LOCAL_L1_DEPLOYER_PRIVATE_KEY =
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";

const DEFAULT_DEPLOYER_KEYSTORE_PASSWORD = "linea-local-dev-deployer";
const DEFAULT_DEPLOYER_KEYSTORE_FILE = "l1-deployer.json";
const DEFAULT_DEPLOYER_PASSWORD_FILE = "password.txt";

export function readDotEnvContents(contents: string): Record<string, string> {
  const result: Record<string, string> = {};
  for (const line of contents.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) {
      continue;
    }
    const index = trimmed.indexOf("=");
    if (index === -1) {
      continue;
    }
    const key = trimmed.slice(0, index).trim();
    let value = trimmed.slice(index + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    result[key] = value;
  }
  return result;
}

export function readDotEnvFile(envPath: string): Record<string, string> {
  if (!fs.existsSync(envPath)) {
    throw new Error(`${envPath} is missing; copy .env.example to .env first`);
  }
  return readDotEnvContents(fs.readFileSync(envPath, "utf8"));
}

export function envValue(name: string, env: EnvMap, fallback = ""): string {
  const raw = env[name];
  return raw === undefined || raw === "" ? fallback : raw;
}

export function requiredEnvValue(name: string, env: EnvMap): string {
  const value = envValue(name, env);
  if (!value) {
    throw new Error(`${name} must be set in .env`);
  }
  return value;
}

export function l1Mode(env: EnvMap): L1Mode {
  const raw = envValue("L1_MODE", env, "sepolia");
  if (raw === "sepolia" || raw === "local") {
    return raw;
  }
  throw new Error(`L1_MODE must be one of ${L1_MODE_VALUES.join(", ")} (got '${raw}')`);
}

export function localL1HostRpcUrl(env: EnvMap): string {
  const port = envValue("HOST_PORT_L1_RPC", env, DEFAULT_LOCAL_L1_HOST_RPC_PORT);
  if (!/^[0-9]+$/.test(port)) {
    throw new Error("HOST_PORT_L1_RPC must be an integer port");
  }
  return `http://localhost:${port}`;
}

function normalizeDir(dir: string): string {
  return path.resolve(dir);
}

function defaultStackDir(): string {
  return normalizeDir(process.env.LINETH_STACK_DIR ?? process.cwd());
}

function defaultAccountsDir(stackDir: string): string {
  return normalizeDir(process.env.LINETH_ACCOUNTS_DIR ?? path.join(stackDir, "artifacts", "accounts"));
}

function ensureDir(dir: string) {
  fs.mkdirSync(dir, { recursive: true });
}

function writeFileMode(file: string, contents: string, mode: number) {
  const tmp = `${file}.tmp`;
  fs.writeFileSync(tmp, contents, { mode });
  fs.renameSync(tmp, file);
  fs.chmodSync(file, mode);
}

function pathInside(child: string, parent: string): boolean {
  const relative = path.relative(parent, child);
  return relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
}

function resolveUserPath(rawPath: string, stackDir: string, accountsDir: string): string {
  const resolved = path.isAbsolute(rawPath) ? normalizeDir(rawPath) : normalizeDir(path.join(stackDir, rawPath));
  if (!pathInside(resolved, stackDir) && !pathInside(resolved, accountsDir)) {
    throw new Error(`L1_DEPLOYER_KEYSTORE_PATH must resolve inside ${stackDir} or ${accountsDir}`);
  }
  return resolved;
}

function readPasswordFile(file: string, envName: string): string {
  const password = fs.readFileSync(file, "utf8").replace(/\r?\n$/, "");
  if (!password) {
    throw new Error(`${envName} must not contain an empty password`);
  }
  return password;
}

function resolveProvidedPassword(env: EnvMap, stackDir: string, accountsDir: string): string {
  const direct = envValue("L1_DEPLOYER_KEYSTORE_PASSWORD", env);
  if (direct) {
    return direct;
  }
  const passwordFile = envValue("L1_DEPLOYER_KEYSTORE_PASSWORD_FILE", env);
  if (!passwordFile) {
    throw new Error("L1_DEPLOYER_KEYSTORE_PASSWORD or L1_DEPLOYER_KEYSTORE_PASSWORD_FILE must be set");
  }
  return readPasswordFile(resolveUserPath(passwordFile, stackDir, accountsDir), "L1_DEPLOYER_KEYSTORE_PASSWORD_FILE");
}

function resolveGeneratedPassword(env: EnvMap, stackDir: string, accountsDir: string, passwordFile: string): string {
  const direct = envValue("L1_DEPLOYER_KEYSTORE_PASSWORD", env);
  if (direct) {
    return direct;
  }
  const configuredPasswordFile = envValue("L1_DEPLOYER_KEYSTORE_PASSWORD_FILE", env);
  if (configuredPasswordFile) {
    return readPasswordFile(
      resolveUserPath(configuredPasswordFile, stackDir, accountsDir),
      "L1_DEPLOYER_KEYSTORE_PASSWORD_FILE",
    );
  }
  if (fs.existsSync(passwordFile)) {
    return readPasswordFile(passwordFile, "generated deployer password file");
  }
  return DEFAULT_DEPLOYER_KEYSTORE_PASSWORD;
}

function envNumber(name: string, env: EnvMap, fallback: number): number {
  const raw = envValue(name, env, fallback.toString());
  if (!/^[0-9]+$/.test(raw)) {
    throw new Error(`${name} must be an integer value`);
  }
  return Number(raw);
}

async function decryptWallet(keystorePath: string, password: string): Promise<Wallet> {
  try {
    return (await Wallet.fromEncryptedJson(fs.readFileSync(keystorePath, "utf8"), password)) as Wallet;
  } catch (_error) {
    throw new Error(`Could not decrypt L1 deployer keystore at ${keystorePath}`);
  }
}

function walletFromPrivateKey(privateKey: string, label: string): Wallet {
  try {
    return new Wallet(privateKey);
  } catch (_error) {
    throw new Error(`${label} is not a valid private key`);
  }
}

async function createGeneratedKeystore(
  keystorePath: string,
  passwordFilePath: string,
  password: string,
  env: EnvMap,
): Promise<Wallet> {
  ensureDir(path.dirname(keystorePath));
  const wallet = Wallet.createRandom();
  const encrypted = await encryptKeystoreJson({ address: wallet.address, privateKey: wallet.privateKey }, password, {
    scrypt: {
      N: envNumber("LINETH_DEPLOYER_KEYSTORE_SCRYPT_N", env, 1 << 12),
      r: envNumber("LINETH_DEPLOYER_KEYSTORE_SCRYPT_R", env, 8),
      p: envNumber("LINETH_DEPLOYER_KEYSTORE_SCRYPT_P", env, 1),
    },
  });
  writeFileMode(keystorePath, `${encrypted}\n`, 0o600);
  if (!fs.existsSync(passwordFilePath)) {
    writeFileMode(passwordFilePath, `${password}\n`, 0o600);
  }
  return wallet as Wallet;
}

function buildResolved(params: {
  mode: L1Mode;
  rpcUrl: string;
  wallet: Wallet;
  source: L1DeployerSource;
  created: boolean;
  keystorePath?: string;
  passwordFilePath?: string;
}): L1DeployerConfig {
  return {
    mode: params.mode,
    rpcUrl: params.rpcUrl,
    deployerPrivateKey: params.wallet.privateKey,
    privateKey: params.wallet.privateKey,
    address: params.wallet.address,
    source: params.source,
    created: params.created,
    keystorePath: params.keystorePath,
    passwordFilePath: params.passwordFilePath,
  };
}

export async function resolveL1DeployerConfig(
  env: EnvMap,
  context: L1Context,
  options: ResolveL1DeployerOptions = {},
): Promise<L1DeployerConfig> {
  const mode = l1Mode(env);
  if (mode === "local") {
    const wallet = walletFromPrivateKey(LOCAL_L1_DEPLOYER_PRIVATE_KEY, "local L1 deployer private key");
    return buildResolved({
      mode,
      rpcUrl: context === "host" ? localL1HostRpcUrl(env) : LOCAL_L1_CONTAINER_RPC_URL,
      wallet,
      source: "local-genesis",
      created: false,
    });
  }

  const stackDir = normalizeDir(options.stackDir ?? defaultStackDir());
  const accountsDir = normalizeDir(options.accountsDir ?? defaultAccountsDir(stackDir));
  const rpcUrl = requiredEnvValue("L1_RPC_URL", env);

  const providedKeystorePath = envValue("L1_DEPLOYER_KEYSTORE_PATH", env);
  if (providedKeystorePath) {
    const keystorePath = resolveUserPath(providedKeystorePath, stackDir, accountsDir);
    const wallet = await decryptWallet(keystorePath, resolveProvidedPassword(env, stackDir, accountsDir));
    return buildResolved({
      mode,
      rpcUrl,
      wallet,
      source: "provided-keystore",
      created: false,
      keystorePath,
    });
  }

  const legacyPrivateKey = envValue("L1_DEPLOYER_PRIVATE_KEY", env);
  if (legacyPrivateKey) {
    return buildResolved({
      mode,
      rpcUrl,
      wallet: walletFromPrivateKey(legacyPrivateKey, "L1_DEPLOYER_PRIVATE_KEY"),
      source: "legacy-raw-key",
      created: false,
    });
  }

  const keystoreDir = path.join(accountsDir, "deployer-keystore");
  const keystorePath = path.join(keystoreDir, DEFAULT_DEPLOYER_KEYSTORE_FILE);
  const passwordFilePath = path.join(keystoreDir, DEFAULT_DEPLOYER_PASSWORD_FILE);
  const password = resolveGeneratedPassword(env, stackDir, accountsDir, passwordFilePath);

  if (fs.existsSync(keystorePath)) {
    const wallet = await decryptWallet(keystorePath, password);
    return buildResolved({
      mode,
      rpcUrl,
      wallet,
      source: "generated-keystore",
      created: false,
      keystorePath,
      passwordFilePath,
    });
  }

  const wallet = await createGeneratedKeystore(keystorePath, passwordFilePath, password, env);
  return buildResolved({
    mode,
    rpcUrl,
    wallet,
    source: "generated-keystore",
    created: true,
    keystorePath,
    passwordFilePath,
  });
}

export async function resolveL1Config(env: EnvMap, context: L1Context): Promise<L1Config> {
  const resolved = await resolveL1DeployerConfig(env, context);
  return {
    mode: resolved.mode,
    rpcUrl: resolved.rpcUrl,
    deployerPrivateKey: resolved.deployerPrivateKey,
  };
}

function shellQuote(value: string): string {
  return `'${value.replace(/'/g, "'\\''")}'`;
}

export function emitShellEnv(config: L1DeployerConfig): string {
  return [
    `L1_MODE=${shellQuote(config.mode)}`,
    `L1_RPC_URL=${shellQuote(config.rpcUrl)}`,
    `L1_DEPLOYER_ADDRESS=${shellQuote(config.address)}`,
    `L1_DEPLOYER_SOURCE=${shellQuote(config.source)}`,
    `L1_DEPLOYER_PRIVATE_KEY=${shellQuote(config.privateKey)}`,
    "",
  ].join("\n");
}

async function cli() {
  const [, , command, ...args] = process.argv;
  if (command !== "emit-shell-env") {
    throw new Error("usage: deployer-wallet.ts emit-shell-env --context <host|container>");
  }
  const contextIndex = args.indexOf("--context");
  const context = contextIndex === -1 ? undefined : args[contextIndex + 1];
  if (context !== "host" && context !== "container") {
    throw new Error("usage: deployer-wallet.ts emit-shell-env --context <host|container>");
  }
  const stackDir = defaultStackDir();
  const envPath = path.join(stackDir, ".env");
  const fileEnv = fs.existsSync(envPath) ? readDotEnvFile(envPath) : {};
  const resolved = await resolveL1DeployerConfig({ ...fileEnv, ...process.env }, context, { stackDir });
  process.stdout.write(emitShellEnv(resolved));
}

if (require.main === module) {
  cli().catch((error) => {
    process.stderr.write(`[deployer-wallet] ERROR: ${sanitizeExternalError(error)}\n`);
    process.exit(1);
  });
}
