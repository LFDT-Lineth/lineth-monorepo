import * as fs from "node:fs";
import * as path from "node:path";

import { encryptKeystoreJson, HDNodeWallet, isAddress, JsonRpcProvider, Wallet } from "ethers";

import { resolveL1DeployerConfig } from "./deployer-wallet";
import { computeBootPrecomputedAddresses } from "./quickstart-invariants";
import { runL1PolicyCheck, sanitizeExternalError } from "./sepolia-policy";

type RuntimeWallet = HDNodeWallet | Wallet;

type RuntimeRole = {
  envName: string;
  keyFile: string;
  label: string;
};

type PrecomputedAddressPlan = {
  _meta: {
    generatedAt: string;
    l1Mode: string;
    l1RpcUrl: string;
    l1ChainId: string;
    l1DeployerStartNonce: number;
    l1AccountSetupBlockNumber: number;
    l1PostmanListenerStartBlock: number;
    l2DeployerStartNonce: number;
    deployScriptVersion: string;
    runtimeKeyModel: string;
  };
  deployers: {
    l1: string;
    l2: string;
  };
  signers: Record<string, string>;
  l1: {
    LineaRollupV8: string;
  };
  l2: {
    L2MessageService: string;
  };
};

type BuildPrecomputedAddressPlanInput = {
  existing?: unknown;
  l1Mode: string;
  l1ChainId: string;
  latestL1Nonce: number;
  l1AccountSetupBlockNumber: number;
  l1PostmanListenerStartBlock: number;
  l1Deployer: RuntimeWallet;
  l2Deployer: RuntimeWallet;
  wallets: Record<string, RuntimeWallet>;
};

const DEFAULT_RUNTIME_PASSWORD = "linea-local-dev";
const ACCOUNTS_DIR = process.env.LINETH_ACCOUNTS_DIR ?? process.env.LINETH_SHARED_DIR ?? "/accounts";
const OUT_JSON = path.join(ACCOUNTS_DIR, "addresses-precomputed.json");
const OUT_RUNTIME_KEYS_ENV = path.join(ACCOUNTS_DIR, "runtime-keys.env");
const OUT_KEYSTORES_DIR = path.join(ACCOUNTS_DIR, "runtime-keystores");
const OUT_KEYSTORE_PASSWORD_FILE = path.join(OUT_KEYSTORES_DIR, "password.txt");
const OUT_WEB3SIGNER_KEYS_DIR = path.join(ACCOUNTS_DIR, "web3signer-keys");

const runtimeRoles: RuntimeRole[] = [
  {
    envName: "L1_BLOB_SUBMITTER_PRIVATE_KEY",
    keyFile: "l1-blob-submitter.json",
    label: "L1 blob/data-submission signer",
  },
  {
    envName: "L1_FINALIZATION_SUBMITTER_PRIVATE_KEY",
    keyFile: "l1-finalization-submitter.json",
    label: "L1 aggregation/finalization signer",
  },
  {
    envName: "L1_POSTMAN_PRIVATE_KEY",
    keyFile: "l1-postman.json",
    label: "L1 postman signer",
  },
  {
    envName: "L2_DEPLOYER_PRIVATE_KEY",
    keyFile: "l2-deployer.json",
    label: "L2 deployer",
  },
  {
    envName: "L2_MESSAGE_ANCHORING_PRIVATE_KEY",
    keyFile: "l2-message-anchoring.json",
    label: "L2 message-anchoring signer",
  },
  {
    envName: "L2_POSTMAN_PRIVATE_KEY",
    keyFile: "l2-postman.json",
    label: "L2 postman signer",
  },
];

function log(message: string) {
  process.stdout.write(`[account-setup] ${sanitizeExternalError(message)}\n`);
}

function die(message: string): never {
  throw new Error(`[account-setup] ERROR: ${message}`);
}

function envNumber(name: string, fallback: number): number {
  const raw = process.env[name] ?? fallback.toString();
  if (!/^[0-9]+$/.test(raw)) {
    die(`${name} must be an integer value`);
  }
  return Number(raw);
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

// Host artifacts are bind-mounted into containers that do not necessarily share
// the host user/group. These files are gitignored dev/demo material and must be
// container-readable for Web3Signer, deploy, traffic, and smoke-test helpers.
const CONTAINER_READABLE_FILE_MODE = 0o644;

function loadKeystorePassword(): string {
  const passwordFile = process.env.LINETH_KEYSTORE_PASSWORD_FILE;
  if (passwordFile) {
    const password = fs.readFileSync(passwordFile, "utf8").replace(/\r?\n$/, "");
    if (!password) {
      die("LINETH_KEYSTORE_PASSWORD_FILE must not contain an empty password because Web3Signer rejects it");
    }
    return password;
  }
  return process.env.LINETH_KEYSTORE_PASSWORD || DEFAULT_RUNTIME_PASSWORD;
}

async function loadOrCreateRuntimeWallet(role: RuntimeRole, password: string): Promise<RuntimeWallet> {
  const file = path.join(OUT_KEYSTORES_DIR, role.keyFile);
  if (fs.existsSync(file)) {
    const json = fs.readFileSync(file, "utf8");
    const wallet = await Wallet.fromEncryptedJson(json, password);
    fs.chmodSync(file, CONTAINER_READABLE_FILE_MODE);
    log(`Reused encrypted keystore for ${role.label}`);
    return wallet;
  }

  const wallet = Wallet.createRandom();
  const encrypted = await encryptKeystoreJson({ address: wallet.address, privateKey: wallet.privateKey }, password, {
    scrypt: {
      N: envNumber("LINETH_KEYSTORE_SCRYPT_N", 1 << 12),
      r: envNumber("LINETH_KEYSTORE_SCRYPT_R", 8),
      p: envNumber("LINETH_KEYSTORE_SCRYPT_P", 1),
    },
  });
  writeFileMode(file, `${encrypted}\n`, CONTAINER_READABLE_FILE_MODE);
  log(`Wrote encrypted keystore for ${role.label}: ${file}`);
  return wallet;
}

function publicKeyWithoutPrefix(wallet: RuntimeWallet): string {
  const publicKey = wallet.signingKey.publicKey;
  if (!/^0x04[0-9a-fA-F]{128}$/.test(publicKey)) {
    die(`derived public key malformed for ${wallet.address}`);
  }
  return `0x${publicKey.slice(4)}`;
}

function assertAddress(address: string, label: string) {
  if (!isAddress(address)) {
    die(`${label} is not a valid address: ${address}`);
  }
}

function objectValue(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    die(`${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function stringField(object: Record<string, unknown>, field: string, label: string): string {
  const value = object[field];
  if (typeof value !== "string" || !value) {
    die(`${label}.${field} is missing or invalid; run ./scripts/reset.sh to regenerate quickstart artifacts`);
  }
  return value;
}

function numberField(object: Record<string, unknown>, field: string, label: string): number {
  const value = object[field];
  if (typeof value !== "number" || !Number.isInteger(value) || value < 0) {
    die(`${label}.${field} is missing or invalid; run ./scripts/reset.sh to regenerate quickstart artifacts`);
  }
  return value;
}

function verifyEqual(actual: string | number, expected: string | number, label: string) {
  if (actual !== expected) {
    die(`${label} changed from ${actual} to ${expected}; run ./scripts/reset.sh to create a new address plan`);
  }
}

function verifyAddressEqual(actual: string, expected: string, label: string) {
  assertAddress(actual, label);
  if (actual.toLowerCase() !== expected.toLowerCase()) {
    die(`${label} changed from ${actual} to ${expected}; run ./scripts/reset.sh to create a new address plan`);
  }
}

function runtimeSignerAddresses(wallets: Record<string, RuntimeWallet>, l1Deployer: RuntimeWallet, l2Deployer: RuntimeWallet) {
  return {
    l1Pubkey: publicKeyWithoutPrefix(l1Deployer),
    l1BlobSubmitterAddress: wallets.L1_BLOB_SUBMITTER_PRIVATE_KEY.address,
    l1BlobSubmitterPubkey: publicKeyWithoutPrefix(wallets.L1_BLOB_SUBMITTER_PRIVATE_KEY),
    l1FinalizationSubmitterAddress: wallets.L1_FINALIZATION_SUBMITTER_PRIVATE_KEY.address,
    l1FinalizationSubmitterPubkey: publicKeyWithoutPrefix(wallets.L1_FINALIZATION_SUBMITTER_PRIVATE_KEY),
    l1PostmanAddress: wallets.L1_POSTMAN_PRIVATE_KEY.address,
    l1PostmanPubkey: publicKeyWithoutPrefix(wallets.L1_POSTMAN_PRIVATE_KEY),
    l2DeployerAddress: l2Deployer.address,
    l2DeployerPubkey: publicKeyWithoutPrefix(l2Deployer),
    l2MessageAnchoringAddress: wallets.L2_MESSAGE_ANCHORING_PRIVATE_KEY.address,
    l2MessageAnchoringPubkey: publicKeyWithoutPrefix(wallets.L2_MESSAGE_ANCHORING_PRIVATE_KEY),
    l2PostmanAddress: wallets.L2_POSTMAN_PRIVATE_KEY.address,
    l2PostmanPubkey: publicKeyWithoutPrefix(wallets.L2_POSTMAN_PRIVATE_KEY),
  };
}

export function buildPrecomputedAddressPlan(input: BuildPrecomputedAddressPlanInput): {
  plan: PrecomputedAddressPlan;
  reused: boolean;
} {
  const signerAddresses = runtimeSignerAddresses(input.wallets, input.l1Deployer, input.l2Deployer);

  if (input.existing !== undefined) {
    const existing = objectValue(input.existing, "addresses-precomputed.json") as unknown as PrecomputedAddressPlan;
    const meta = objectValue(existing._meta, "addresses-precomputed.json._meta");
    const deployers = objectValue(existing.deployers, "addresses-precomputed.json.deployers");
    const signers = objectValue(existing.signers, "addresses-precomputed.json.signers");
    const l1 = objectValue(existing.l1, "addresses-precomputed.json.l1");
    const l2 = objectValue(existing.l2, "addresses-precomputed.json.l2");
    const storedNonce = numberField(meta, "l1DeployerStartNonce", "addresses-precomputed.json._meta");
    const expected = computeBootPrecomputedAddresses({
      l1DeployerAddress: input.l1Deployer.address,
      l1DeployerStartNonce: storedNonce,
      l2DeployerAddress: input.l2Deployer.address,
    });

    verifyEqual(stringField(meta, "l1Mode", "addresses-precomputed.json._meta"), input.l1Mode, "L1 mode");
    verifyEqual(stringField(meta, "l1ChainId", "addresses-precomputed.json._meta"), input.l1ChainId, "L1 chain ID");
    verifyEqual(numberField(meta, "l2DeployerStartNonce", "addresses-precomputed.json._meta"), 0, "L2 deployer start nonce");
    verifyAddressEqual(stringField(deployers, "l1", "addresses-precomputed.json.deployers"), input.l1Deployer.address, "L1 deployer");
    verifyAddressEqual(stringField(deployers, "l2", "addresses-precomputed.json.deployers"), input.l2Deployer.address, "L2 deployer");
    for (const [key, value] of Object.entries(signerAddresses)) {
      if (key.endsWith("Address")) {
        verifyAddressEqual(stringField(signers, key, "addresses-precomputed.json.signers"), value, key);
      } else {
        verifyEqual(stringField(signers, key, "addresses-precomputed.json.signers"), value, key);
      }
    }
    verifyAddressEqual(stringField(l1, "LineaRollupV8", "addresses-precomputed.json.l1"), expected.l1LineaRollup, "L1 LineaRollupV8");
    verifyAddressEqual(
      stringField(l2, "L2MessageService", "addresses-precomputed.json.l2"),
      expected.l2MessageService,
      "L2 MessageService",
    );

    return {
      plan: existing,
      reused: true,
    };
  }

  const computed = computeBootPrecomputedAddresses({
    l1DeployerAddress: input.l1Deployer.address,
    l1DeployerStartNonce: input.latestL1Nonce,
    l2DeployerAddress: input.l2Deployer.address,
  });

  return {
    reused: false,
    plan: {
      _meta: {
        generatedAt: new Date().toISOString(),
        l1Mode: input.l1Mode,
        l1RpcUrl: "<redacted>",
        l1ChainId: input.l1ChainId,
        l1DeployerStartNonce: input.latestL1Nonce,
        l1AccountSetupBlockNumber: input.l1AccountSetupBlockNumber,
        l1PostmanListenerStartBlock: input.l1PostmanListenerStartBlock,
        l2DeployerStartNonce: 0,
        deployScriptVersion: "v1-ethers-runtime-keystores",
        runtimeKeyModel: "ethers-v6-encrypted-json",
      },
      deployers: {
        l1: input.l1Deployer.address,
        l2: input.l2Deployer.address,
      },
      signers: signerAddresses,
      l1: {
        LineaRollupV8: computed.l1LineaRollup,
      },
      l2: {
        L2MessageService: computed.l2MessageService,
      },
    },
  };
}

function writeRuntimeKeysEnv(wallets: Record<string, RuntimeWallet>) {
  const lines = [
    "# Generated by account-setup.ts from encrypted ethers runtime keystores.",
    "# Host artifact compatibility file. Do not commit.",
    ...runtimeRoles.map((role) => `${role.envName}='${wallets[role.envName].privateKey}'`),
    "",
  ];
  writeFileMode(OUT_RUNTIME_KEYS_ENV, lines.join("\n"), CONTAINER_READABLE_FILE_MODE);
  log(`Wrote ${OUT_RUNTIME_KEYS_ENV}`);
}

function writeKeystorePasswordFile(password: string) {
  writeFileMode(OUT_KEYSTORE_PASSWORD_FILE, `${password}\n`, CONTAINER_READABLE_FILE_MODE);
  log(`Wrote ${OUT_KEYSTORE_PASSWORD_FILE}`);
}

function writeWeb3SignerKey(fileName: string, label: string, keyFile: string) {
  const file = path.join(OUT_WEB3SIGNER_KEYS_DIR, `${fileName}.yaml`);
  writeFileMode(
    file,
    `# ============================================================\n` +
      `# DEV ONLY — generated at boot by account-setup.ts.\n` +
      `# Stored in host artifacts for retry-safe restarts.\n` +
      `# Do NOT commit these generated files.\n` +
      `# ============================================================\n` +
      `type: "file-keystore"\n` +
      `keyType: "SECP256K1"\n` +
      `# ${label}\n` +
      `keystoreFile: "${path.join(OUT_KEYSTORES_DIR, keyFile)}"\n` +
      `keystorePasswordFile: "${OUT_KEYSTORE_PASSWORD_FILE}"\n`,
    CONTAINER_READABLE_FILE_MODE,
  );
  log(`Wrote ${file}`);
}

async function main() {
  ensureDir(ACCOUNTS_DIR);
  ensureDir(OUT_KEYSTORES_DIR);
  ensureDir(OUT_WEB3SIGNER_KEYS_DIR);

  const l1DeployerConfig = await resolveL1DeployerConfig(process.env, "container");
  const provider = new JsonRpcProvider(l1DeployerConfig.rpcUrl);
  const l1Deployer = new Wallet(l1DeployerConfig.privateKey, provider);

  const password = loadKeystorePassword();
  if (password === DEFAULT_RUNTIME_PASSWORD) {
    log("Using default local runtime keystore password");
  } else {
    log("Using runtime keystore password from environment");
  }

  const wallets: Record<string, RuntimeWallet> = {};
  for (const role of runtimeRoles) {
    wallets[role.envName] = await loadOrCreateRuntimeWallet(role, password);
  }

  const policyReport = await runL1PolicyCheck({
    provider,
    deployerAddress: l1Deployer.address,
    env: process.env,
  });
  for (const warning of policyReport.warnings) {
    log(`warn: ${warning}`);
  }
  log(`L1 deployer: ${policyReport.deployerAddress}`);
  log(`L1 deployer source: ${l1DeployerConfig.source}`);
  log(`L1 deployer balance: ${policyReport.balanceWei} wei`);
  log(`L1 mode: ${policyReport.mode}`);
  log(`L1 deployer required minimum: ${policyReport.minimumBalanceWei} wei`);
  log(`L1 deploy gas price: ${policyReport.config.l1DeployGasPriceWei} wei`);
  log(`L1 safe listener start block for postman: ${policyReport.l1PostmanListenerStartBlock}`);

  const {
    chainId,
    latestNonce: nonce,
    l1AccountSetupBlockNumber,
    l1PostmanListenerStartBlock,
  } = policyReport;

  const l2Deployer = wallets.L2_DEPLOYER_PRIVATE_KEY;
  writeRuntimeKeysEnv(wallets);
  writeKeystorePasswordFile(password);
  writeWeb3SignerKey(
    "anchoring-signer",
    "L2 message-anchoring signer (generated runtime key, L2 only)",
    "l2-message-anchoring.json",
  );
  writeWeb3SignerKey(
    "data-submission-signer",
    "L1 blob/data-submission signer (generated runtime key)",
    "l1-blob-submitter.json",
  );
  writeWeb3SignerKey(
    "finalization-signer",
    "L1 aggregation/finalization signer (generated runtime key)",
    "l1-finalization-submitter.json",
  );
  writeWeb3SignerKey("l1-postman-signer", "L1 postman signer (generated runtime key)", "l1-postman.json");
  writeWeb3SignerKey("l2-postman-signer", "L2 postman signer (generated runtime key)", "l2-postman.json");

  const existingAddresses = fs.existsSync(OUT_JSON) ? JSON.parse(fs.readFileSync(OUT_JSON, "utf8")) : undefined;
  const { plan: addresses, reused } = buildPrecomputedAddressPlan({
    existing: existingAddresses,
    l1Mode: policyReport.mode,
    l1ChainId: chainId.toString(),
    latestL1Nonce: nonce,
    l1AccountSetupBlockNumber,
    l1PostmanListenerStartBlock,
    l1Deployer,
    l2Deployer,
    wallets,
  });

  writeFileMode(OUT_JSON, `${JSON.stringify(addresses, null, 2)}\n`, CONTAINER_READABLE_FILE_MODE);
  log(`${reused ? "Reused" : "Wrote"} ${OUT_JSON}`);
  log(`Pre-computed L1 LineaRollupV8 (proxy): ${addresses.l1.LineaRollupV8}`);
  log(`Pre-computed L2 MessageService: ${addresses.l2.L2MessageService}`);
  provider.destroy();
  log("Done.");
}

if (require.main === module) {
  main().catch((error) => {
    process.stderr.write(`${sanitizeExternalError(error)}\n`);
    process.exit(1);
  });
}
