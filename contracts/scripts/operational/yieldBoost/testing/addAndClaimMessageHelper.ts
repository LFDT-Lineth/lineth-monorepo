import { HardhatRuntimeEnvironment } from "hardhat/types";
import { Contract } from "ethers";
import { getTaskCliOrEnvValue } from "../../../../common/helpers/environmentHelper";
import { encodeSendMessage, randomBytes32 } from "../../../../common/helpers/encoding";

export interface ClaimParams {
  proof: string[];
  messageNumber: bigint;
  leafIndex: bigint;
  from: string;
  to: string;
  fee: string;
  value: bigint;
  feeRecipient: string;
  merkleRoot: string;
  data: string;
  yieldProvider?: string;
}

export interface PrepareMessageResult {
  claimParams: ClaimParams;
  lineaRollup: Contract;
}

/**
 * Prepares and adds a message merkle root to LineaRollup.
 * Handles parameter resolution, validation, message number generation,
 * proof creation, merkle root calculation, and adding the root to the contract.
 *
 * @param taskArgs - Task arguments from Hardhat
 * @param hre - Hardhat runtime environment
 * @param requireYieldProvider - Whether yieldProvider is required (default: false)
 * @returns Claim parameters and LineaRollup contract instance
 */
export async function prepareAndAddMessageMerkleRoot(
  taskArgs: Record<string, unknown>,
  hre: HardhatRuntimeEnvironment,
  requireYieldProvider: boolean = false,
): Promise<PrepareMessageResult> {
  const { ethers, getNamedAccounts } = hre;
  const { deployer } = await getNamedAccounts();
  const signer = await ethers.getSigner(deployer);

  // --- Resolve inputs from CLI or ENV (with sensible fallbacks to deployments) ---
  const lineaRollupAddress = getTaskCliOrEnvValue(taskArgs, "lineaRollupAddress", "LINEA_ROLLUP_ADDRESS");
  const fromAddress = getTaskCliOrEnvValue(taskArgs, "from", "FROM_ADDRESS") || signer.address;
  const toAddress = getTaskCliOrEnvValue(taskArgs, "to", "TO_ADDRESS");
  const valueRaw = getTaskCliOrEnvValue(taskArgs, "value", "VALUE");
  const data = getTaskCliOrEnvValue(taskArgs, "data", "DATA") || "0x";
  const yieldProvider = getTaskCliOrEnvValue(taskArgs, "yieldProvider", "YIELD_PROVIDER_ADDRESS");

  // Validate required params
  const missing: string[] = [];
  if (!lineaRollupAddress) missing.push("lineaRollupAddress / LINEA_ROLLUP_ADDRESS");
  if (!toAddress) missing.push("to / TO_ADDRESS");
  if (!valueRaw) missing.push("value / VALUE");
  // Note: data has a default value of "0x" (empty calldata), so validation is not needed
  if (requireYieldProvider && !yieldProvider) {
    missing.push("yieldProvider / YIELD_PROVIDER_ADDRESS");
  }
  if (missing.length) {
    throw new Error(`Missing required params/envs: ${missing.join(", ")}`);
  }

  // --- Parse numeric params ---
  const value = valueRaw ? BigInt(valueRaw) : 0n;

  // Fixed values
  const fee = 0n;
  const leafIndex = 0n;
  // Generate random proof array first (depth determined by proof length)
  const proof = Array.from({ length: 32 }, () => randomBytes32());
  const proofDepth = proof.length;

  const lineaRollup = await ethers.getContractAt("TestLineaRollup", lineaRollupAddress!, signer);
  // Generate random messageNumber and check if it's already claimed
  let messageNumber: bigint;
  let attempts = 0;
  const maxAttempts = 100; // Prevent infinite loop
  do {
    messageNumber = ethers.toBigInt(ethers.randomBytes(32));
    attempts++;
    if (attempts > maxAttempts) {
      throw new Error(
        `Failed to find unclaimed message number after ${maxAttempts} attempts. This is highly unlikely.`,
      );
    }
    const isClaimed = await lineaRollup.isMessageClaimed(messageNumber);
    if (!isClaimed) {
      break;
    }
    console.log(`  Message number ${messageNumber.toString()} already claimed, generating new one...`);
    // eslint-disable-next-line no-constant-condition
  } while (true);

  // Log params
  console.log("Parameters:");
  console.log("  lineaRollupAddress:", lineaRollupAddress);
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
  const expectedBytes = encodeSendMessage(fromAddress, toAddress!, fee, value, messageNumber, data);
  const messageHash = ethers.keccak256(expectedBytes);
  console.log("  messageHash:", messageHash);

  // Get generatedRoot
  const generatedRoot = await lineaRollup.generateMerkleRoot.staticCall(messageHash, proof, leafIndex);
  console.log("generatedRoot=", generatedRoot);

  // Add merkle root to LineaRollup
  console.log("\nAdding merkle root to LineaRollup...");
  console.log("  NOTE: Signer must have DEFAULT_ADMIN_ROLE");
  {
    const tx = await lineaRollup.addL2MerkleRoots([generatedRoot], proofDepth);
    console.log("  Transaction hash:", tx.hash);
    const receipt = await tx.wait();
    console.log("  Transaction confirmed in block:", receipt?.blockNumber);
  }

  // Build claim parameters object
  const claimParams: ClaimParams = {
    proof,
    messageNumber,
    leafIndex,
    from: fromAddress,
    to: toAddress!,
    fee: fee.toString(),
    value,
    feeRecipient: ethers.ZeroAddress,
    merkleRoot: generatedRoot,
    data,
  };

  if (yieldProvider) {
    claimParams.yieldProvider = yieldProvider;
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
    feeRecipient: ethers.ZeroAddress,
    merkleRoot: generatedRoot,
    data: data,
  };
  if (yieldProvider) {
    jsonParams.yieldProvider = yieldProvider;
  }
  console.log(JSON.stringify(jsonParams, null, 2));

  return {
    claimParams,
    lineaRollup,
  };
}
