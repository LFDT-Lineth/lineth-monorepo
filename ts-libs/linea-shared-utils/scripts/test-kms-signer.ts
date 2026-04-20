import { createPublicClient, http, keccak256, parseSignature, recoverAddress, serializeTransaction } from "viem";

import { AwsKmsSignerClientAdapter } from "../src/clients/AwsKmsSignerClientAdapter";
import { WinstonLogger } from "../src/logging/WinstonLogger";

const REQUIRED_ENV_VARS = ["KMS_KEY_ID", "RPC_URL"];

function readRequiredEnv() {
  const missing = REQUIRED_ENV_VARS.filter((name) => !process.env[name]);
  if (missing.length > 0) {
    throw new Error(`Missing required env vars: ${missing.join(", ")}`);
  }
}

async function main() {
  try {
    readRequiredEnv();
  } catch (err) {
    console.error((err as Error).message);
    process.exitCode = 1;
    return;
  }

  const KMS_KEY_ID = process.env.KMS_KEY_ID!;
  const REGION = process.env.AWS_REGION ?? "us-east-2";
  const RPC_URL = process.env.RPC_URL;
  const DRY_RUN = process.env.DRY_RUN === "true";

  const logger = new WinstonLogger("kms-signer-test");

  console.log(`\n=== AWS KMS Signer Integration Test ===`);
  console.log(`Region  : ${REGION}`);
  console.log(`Key ID  : ${KMS_KEY_ID}`);
  console.log(`Dry run : ${DRY_RUN}\n`);

  // 1. Init — fetches public key from KMS and derives Ethereum address
  console.log("1. Initializing adapter (fetching public key from KMS)...");
  const signer = await AwsKmsSignerClientAdapter.create(logger, KMS_KEY_ID, { region: REGION });
  const address = signer.getAddress();
  console.log(`   Derived address: ${address}\n`);

  const client = !DRY_RUN ? createPublicClient({ transport: http(RPC_URL) }) : undefined;

  // 2. Build transaction
  const chainId = client ? await client.getChainId() : 59144;
  const nonce = client ? await client.getTransactionCount({ address }) : 0;

  const tx = {
    type: "eip1559" as const,
    chainId,
    nonce,
    gas: 21_000n,
    maxFeePerGas: 1_000_000_000n,
    maxPriorityFeePerGas: 100_000_000n,
    to: address,
    value: 0n,
    data: "0x" as const,
  };

  console.log("2. Signing EIP-1559 transaction via KMS...");
  console.log(`   chainId=${chainId} nonce=${nonce} to=${tx.to}`);
  const signatureHex = await signer.sign(tx);
  console.log(`   Signature: ${signatureHex.slice(0, 40)}...${signatureHex.slice(-20)}\n`);

  // 3. Recover signer from (txHash, signature) and verify
  console.log("3. Verifying signature (recovering signer address)...");
  const txHash = keccak256(serializeTransaction(tx));
  const signature = parseSignature(signatureHex);
  const recovered = await recoverAddress({ hash: txHash, signature });

  console.log(`   Recovered address: ${recovered}`);
  const match = recovered.toLowerCase() === address.toLowerCase();
  console.log(`   Match: ${match ? "YES" : "NO"}\n`);

  if (!match) {
    console.error("SIGNATURE VERIFICATION FAILED");
    process.exit(1);
  }

  // 4. Send transaction (only when not dry-run)
  if (!DRY_RUN && client) {
    console.log("4. Broadcasting signed transaction...");
    const serializedSignedTx = serializeTransaction(tx, signature);
    const txHashSent = await client.sendRawTransaction({ serializedTransaction: serializedSignedTx });
    console.log(`   Tx hash: ${txHashSent}`);

    console.log("   Waiting for receipt...");
    const receipt = await client.waitForTransactionReceipt({ hash: txHashSent });
    console.log(`   Status : ${receipt.status}`);
    console.log(`   Block  : ${receipt.blockNumber}`);
    console.log(`   Gas    : ${receipt.gasUsed}\n`);

    if (receipt.status !== "success") {
      console.error("TRANSACTION FAILED");
      process.exit(1);
    }
  } else {
    console.log("4. Skipping broadcast (dry run)\n");
  }

  console.log("=== All checks passed ===\n");
}

main().catch((err) => {
  console.error("FAILED:", err);
  process.exit(1);
});
