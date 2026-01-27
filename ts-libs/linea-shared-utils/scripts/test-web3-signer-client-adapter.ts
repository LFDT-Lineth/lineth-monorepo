/* Example usage (ensure certificates are accessible from the repo root):

First run web3signer Docker container:

docker run --rm \
  --platform=linux/amd64 \
  --name web3signer \
  -p 9000:9000 \
  -v "$(pwd)/docker/web3signer/key-files:/key-files" \
  -v "$(pwd)/docker/web3signer/tls-files:/tls-files" \
  consensys/web3signer:25.12.0 \
  --key-store-path=/key-files/ \
  --tls-keystore-file=/tls-files/web3signer-keystore.p12 \
  --tls-keystore-password-file=/tls-files/web3signer-keystore-password.txt \
  --tls-known-clients-file=/tls-files/known-clients.txt \
  --http-host-allowlist='*' \
  eth1 \
  --chain-id=1337

Then run this script

WEB3_SIGNER_URL=https://127.0.0.1:9000 \
WEB3_SIGNER_PUBLIC_KEY=4a788ad6fa008beed58de6418369717d7492f37d173d70e2c26d9737e2c6eeae929452ef8602a19410844db3e200a0e73f5208fd76259a8766b73953fc3e7023 \
WEB3_SIGNER_KEYSTORE_PATH="$(pwd)/docker/config/linea-besu-sequencer/tls-files/sequencer_client_keystore.p12" \
WEB3_SIGNER_KEYSTORE_PASSPHRASE=changeit \
WEB3_SIGNER_TRUST_STORE_PATH="$(pwd)/docker/config/linea-besu-sequencer/tls-files/web3signer_truststore.p12" \
WEB3_SIGNER_TRUST_STORE_PASSPHRASE=changeit \
pnpm --filter @consensys/linea-shared-utils exec tsx scripts/test-web3-signer-client-adapter.ts

// Optional overrides (defaults shown):
//   TX_CHAIN_ID=1337
//   TX_NONCE=0
//   TX_GAS_LIMIT=21000
//   TX_MAX_FEE_PER_GAS=1000000000
//   TX_MAX_PRIORITY_FEE_PER_GAS=100000000
//   TX_VALUE=0
//   TX_TO=0x0000000000000000000000000000000000000000
//   TX_DATA=0x
 */

import { Web3SignerClientAdapter } from "../src/clients/Web3SignerClientAdapter";
import { WinstonLogger } from "../src/logging/WinstonLogger";
import { Address, Hex, TransactionSerializableEIP1559, serializeTransaction } from "viem";

const REQUIRED_ENV_VARS = [
  "WEB3_SIGNER_URL",
  "WEB3_SIGNER_PUBLIC_KEY",
  "WEB3_SIGNER_KEYSTORE_PATH",
  "WEB3_SIGNER_KEYSTORE_PASSPHRASE",
  "WEB3_SIGNER_TRUST_STORE_PATH",
  "WEB3_SIGNER_TRUST_STORE_PASSPHRASE",
];

function readRequiredEnv() {
  const missing = REQUIRED_ENV_VARS.filter((name) => !process.env[name]);
  if (missing.length > 0) {
    throw new Error(`Missing required env vars: ${missing.join(", ")}`);
  }
}

function numberFromEnv(name: string, defaultValue: number): number {
  const raw = process.env[name];
  if (!raw) {
    return defaultValue;
  }
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) {
    throw new Error(`Environment variable ${name} must be a finite number, received: ${raw}`);
  }
  return parsed;
}

function bigintFromEnv(name: string, defaultValue: bigint): bigint {
  const raw = process.env[name];
  if (!raw) {
    return defaultValue;
  }
  try {
    return BigInt(raw);
  } catch {
    throw new Error(`Environment variable ${name} must be a bigint-compatible value, received: ${raw}`);
  }
}

function hexFromEnv(name: string, defaultValue: Hex): Hex {
  const raw = process.env[name];
  if (!raw) {
    return defaultValue;
  }
  if (!raw.startsWith("0x")) {
    throw new Error(`Environment variable ${name} must be a 0x-prefixed hex string, received: ${raw}`);
  }
  return raw as Hex;
}

function addressFromEnv(name: string, defaultValue: Address): Address {
  const raw = process.env[name];
  if (!raw) {
    return defaultValue;
  }
  if (!raw.startsWith("0x") || raw.length !== 42) {
    throw new Error(`Environment variable ${name} must be a 20-byte 0x-prefixed address, received: ${raw}`);
  }
  return raw as Address;
}

async function main() {
  try {
    readRequiredEnv();
  } catch (err) {
    console.error((err as Error).message);
    process.exitCode = 1;
    return;
  }

  const web3SignerUrl = process.env.WEB3_SIGNER_URL as string;
  const web3SignerPublicKey = process.env.WEB3_SIGNER_PUBLIC_KEY as Hex;
  const web3SignerKeystorePath = process.env.WEB3_SIGNER_KEYSTORE_PATH as string;
  const web3SignerKeystorePassphrase = process.env.WEB3_SIGNER_KEYSTORE_PASSPHRASE as string;
  const web3SignerTrustedStorePath = process.env.WEB3_SIGNER_TRUST_STORE_PATH as string;
  const web3SignerTrustedStorePassphrase = process.env.WEB3_SIGNER_TRUST_STORE_PASSPHRASE as string;

  const chainId = numberFromEnv("TX_CHAIN_ID", 1337);
  const nonce = numberFromEnv("TX_NONCE", 0);
  const gasLimit = bigintFromEnv("TX_GAS_LIMIT", BigInt(21_000));
  const maxFeePerGas = bigintFromEnv("TX_MAX_FEE_PER_GAS", BigInt(1_000_000_000));
  const maxPriorityFeePerGas = bigintFromEnv("TX_MAX_PRIORITY_FEE_PER_GAS", BigInt(100_000_000));
  const value = bigintFromEnv("TX_VALUE", BigInt(0));
  const to = addressFromEnv("TX_TO", "0x0000000000000000000000000000000000000000");
  const data = hexFromEnv("TX_DATA", "0x");

  const logger = new WinstonLogger("Web3SignerClientAdapter.integration");
  const signer = new Web3SignerClientAdapter(
    logger,
    web3SignerUrl,
    web3SignerPublicKey,
    web3SignerKeystorePath,
    web3SignerKeystorePassphrase,
    web3SignerTrustedStorePath,
    web3SignerTrustedStorePassphrase,
  );

  console.log("Derived address from public key:", signer.getAddress());

  const tx: TransactionSerializableEIP1559 = {
    type: "eip1559",
    chainId,
    nonce,
    gas: gasLimit,
    to,
    value,
    maxFeePerGas,
    maxPriorityFeePerGas,
    data,
  };

  console.log("Prepared transaction payload:", tx);
  console.log("Unsigned serialized transaction:", serializeTransaction(tx));

  try {
    const signature = await signer.sign(tx);
    console.log("Web3Signer signature:", signature);
  } catch (err) {
    console.error("Web3SignerClientAdapter integration script failed:", err);
    process.exitCode = 1;
  }
}

main();
