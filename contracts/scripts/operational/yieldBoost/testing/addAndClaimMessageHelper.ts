import { type Address, type Hex, bytesToBigInt, getAddress, isHex, keccak256, zeroAddress } from "viem";

import { encodeSendMessage, randomBytes32 } from "../../../../common/helpers/encoding.js";
import { getTaskCliOrEnvValue } from "../../../../common/helpers/environmentHelper.js";
import { getUiWalletClient } from "../../../../hardhat-plugins/signer-ui/session.js";

import type { ContractReturnType, PublicClient } from "@nomicfoundation/hardhat-viem/types";
import type { NetworkConnection } from "hardhat/types/network";

export interface ClaimParams {
  proof: Hex[];
  messageNumber: bigint;
  leafIndex: number;
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  feeRecipient: Address;
  merkleRoot: Hex;
  data: Hex;
  yieldProvider?: Address;
}

export interface PrepareMessageResult {
  claimParams: ClaimParams;
  linethRollup: ContractReturnType<"TestLinethRollup">;
  publicClient: PublicClient;
}

/**
 * Prepares and adds a message merkle root to LinethRollup.
 * Handles parameter resolution, validation, message number generation,
 * proof creation, merkle root calculation, and adding the root to the contract.
 *
 * @param taskArgs - Task arguments from Hardhat
 * @param connection - Hardhat network connection
 * @param requireYieldProvider - Whether yieldProvider is required (default: false)
 * @returns Claim parameters and LinethRollup contract instance
 */
export async function prepareAndAddMessageMerkleRoot(
  taskArgs: Record<string, unknown>,
  connection: NetworkConnection,
  requireYieldProvider: boolean = false,
): Promise<PrepareMessageResult> {
  const { viem } = connection;
  const wallet = await getUiWalletClient(connection);
  const publicClient = await viem.getPublicClient();
  const signerAddress = wallet.account.address;

  // --- Resolve inputs from CLI or ENV (with sensible fallbacks to deployments) ---
  const linethRollupAddress = getTaskCliOrEnvValue(taskArgs, "linethRollupAddress", "LINETH_ROLLUP_ADDRESS");
  const fromAddress = getTaskCliOrEnvValue(taskArgs, "from", "FROM_ADDRESS") || signerAddress;
  const toAddress = getTaskCliOrEnvValue(taskArgs, "to", "TO_ADDRESS");
  const valueRaw = getTaskCliOrEnvValue(taskArgs, "value", "VALUE");
  const data = getTaskCliOrEnvValue(taskArgs, "data", "DATA") || "0x";
  const yieldProvider = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");

  // Validate required params
  const missing: string[] = [];
  if (!linethRollupAddress) missing.push("linethRollupAddress / LINETH_ROLLUP_ADDRESS");
  if (!toAddress) missing.push("to / TO_ADDRESS");
  if (!valueRaw) missing.push("value / VALUE");
  // Note: data has a default value of "0x" (empty calldata), so validation is not needed
  if (requireYieldProvider && !yieldProvider) {
    missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
  }
  if (missing.length || !linethRollupAddress || !toAddress) {
    throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
  }
  if (!isHex(data)) {
    throw new Error(`Invalid data "${data}": expected a 0x-prefixed hex string`);
  }
  const from = getAddress(fromAddress);
  const to = getAddress(toAddress);

  // --- Parse numeric params ---
  const value = valueRaw ? BigInt(valueRaw) : 0n;

  // Fixed values
  const fee = 0n;
  const leafIndex = 0;
  // Generate random proof array first (depth determined by proof length)
  const proof = Array.from({ length: 32 }, () => randomBytes32());
  const proofDepth = proof.length;

  const linethRollup = await viem.getContractAt("TestLinethRollup", getAddress(linethRollupAddress), {
    client: { public: publicClient, wallet },
  });
  // Generate random messageNumber and check if it's already claimed
  let messageNumber: bigint;
  let attempts = 0;
  const maxAttempts = 100; // Prevent infinite loop
  do {
    messageNumber = bytesToBigInt(crypto.getRandomValues(new Uint8Array(32)));
    attempts++;
    if (attempts > maxAttempts) {
      throw new Error(
        `Failed to find unclaimed message number after ${maxAttempts} attempts. This is highly unlikely.`,
      );
    }
    const isClaimed = await linethRollup.read.isMessageClaimed([messageNumber]);
    if (!isClaimed) {
      break;
    }
    console.log(`  Message number ${messageNumber.toString()} already claimed, generating new one...`);
    // eslint-disable-next-line no-constant-condition
  } while (true);

  // Log params
  console.log("Parameters:");
  console.log("  linethRollupAddress:", linethRollupAddress);
  console.log("  messageNumber:", messageNumber.toString(), "(auto-generated, verified unclaimed)");
  console.log("  from:", fromAddress);
  console.log("  to:", toAddress);
  console.log("  value:", value.toString());
  console.log("  fee:", fee.toString(), "(default: 0)");
  console.log("  data:", data);
  if (yieldProvider) {
    console.log("  yieldProvider:", yieldProvider);
  }
  console.log("  leafIndex:", leafIndex.toString(), "(default: 0)");
  console.log("  proofDepth:", proofDepth, "(from generated proof)");

  // Encode message bytes
  const expectedBytes = encodeSendMessage(from, to, fee, value, messageNumber, data);
  const messageHash = keccak256(expectedBytes);
  console.log("  messageHash:", messageHash);

  // Get generatedRoot
  const generatedRoot = await linethRollup.read.generateMerkleRoot([messageHash, proof, leafIndex]);
  console.log("generatedRoot=", generatedRoot);

  // Add merkle root to LinethRollup
  console.log("\nAdding merkle root to LinethRollup...");
  console.log("  NOTE: Signer must have DEFAULT_ADMIN_ROLE");
  {
    const hash = await linethRollup.write.addL2MerkleRoots([[generatedRoot], BigInt(proofDepth)]);
    console.log("  Transaction hash:", hash);
    const receipt = await publicClient.waitForTransactionReceipt({ hash });
    console.log("  Transaction confirmed in block:", receipt.blockNumber.toString());
  }

  // Build claim parameters object
  const claimParams: ClaimParams = {
    proof,
    messageNumber,
    leafIndex,
    from,
    to,
    fee,
    value,
    feeRecipient: zeroAddress,
    merkleRoot: generatedRoot,
    data,
  };

  if (yieldProvider) {
    claimParams.yieldProvider = getAddress(yieldProvider);
  }

  // Output claim parameters
  console.log("\n=== Claim Parameters ===");
  console.log("Claim parameters:");
  const jsonParams: Record<string, unknown> = {
    proof: proof,
    messageNumber: messageNumber.toString(),
    leafIndex: leafIndex.toString(),
    from: fromAddress,
    to: toAddress,
    fee: fee.toString(),
    value: value.toString(),
    feeRecipient: zeroAddress,
    merkleRoot: generatedRoot,
    data: data,
  };
  if (yieldProvider) {
    jsonParams.yieldProvider = yieldProvider;
  }
  console.log(JSON.stringify(jsonParams, null, 2));

  return {
    claimParams,
    linethRollup,
    publicClient,
  };
}
