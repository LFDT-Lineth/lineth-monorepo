import * as dotenv from "dotenv";
import {
  createPublicClient,
  createWalletClient,
  encodeFunctionData,
  getAddress,
  http,
  isHex,
  parseAbi,
  zeroAddress,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { requireEnv, checkDelegation, getAccountInfo, createAuthorization, estimateGasFees } from "./utils.js";

// Self-sponsored EIP-7702 transaction: the same wallet signs the authorization AND sends the transaction.
// Prerequisite - Deploy a contract with NON-VIEW initialize() function, e.g. TestEIP7702Delegation
// Use this contract for TARGET_ADDRESS env
//
// TARGET_ADDRESS is optional. If omitted (or set to 0x0), the authorization is set to the zero
// address, which removes any existing EIP-7702 delegation from the signer's EOA per the spec.

// RPC_URL=<> DEPLOYER_PRIVATE_KEY=<> [TARGET_ADDRESS=<>] pnpm exec hardhat run scripts/testEIP7702/sendSelfSponsoredType4Tx.ts

dotenv.config();

const SHORT_ZERO_ADDRESS = "0x0";

async function main() {
  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
  const targetAddressEnv = process.env.TARGET_ADDRESS?.trim();
  const targetAddress =
    targetAddressEnv == null || targetAddressEnv === SHORT_ZERO_ADDRESS ? zeroAddress : getAddress(targetAddressEnv);

  const normalizedPrivateKey = privateKey.startsWith("0x") ? privateKey : `0x${privateKey}`;
  if (!isHex(normalizedPrivateKey)) {
    throw new Error("DEPLOYER_PRIVATE_KEY must be a hex string");
  }
  const signer = privateKeyToAccount(normalizedPrivateKey);
  const provider = createPublicClient({ transport: http(rpcUrl) });
  const walletClient = createWalletClient({ account: signer, transport: http(rpcUrl) });

  const signerInfo = await getAccountInfo(provider, signer.address);
  console.log("Signer info:", signerInfo);

  const delegationStatus = await checkDelegation(provider, signer.address);
  console.log("EOA delegation status:", delegationStatus);

  const isRevocation = targetAddress === zeroAddress;

  console.log("\n=== SELF-SPONSORED EIP-7702 TRANSACTION ===");
  console.log("Same wallet signs authorization and sends the transaction.");
  if (isRevocation) {
    console.log("TARGET_ADDRESS not set or set to 0x0 - sending revocation (zero address) to remove delegation.");
  }

  // +1 nonce offset: sender nonce is incremented before authorization processing
  const authorization = await createAuthorization(signer, provider, targetAddress, 1);

  let hash;
  if (isRevocation) {
    // No calldata to execute when revoking - the authorization list alone carries the intent.
    // Use the node's EIP-1559 fee estimate directly; linea_estimateGas rejects empty-calldata self-sends.
    const feeData = await provider.estimateFeesPerGas();
    hash = await walletClient.sendTransaction({
      chain: null,
      type: "eip7702",
      authorizationList: [authorization],
      to: signer.address,
      data: "0x",
      gas: 500000n,
      value: 0n,
      maxFeePerGas: feeData.maxFeePerGas,
      maxPriorityFeePerGas: feeData.maxPriorityFeePerGas,
    });
  } else {
    const ABI = parseAbi(["function initialize() external"]);

    const calldata = encodeFunctionData({ abi: ABI, functionName: "initialize" });
    const delegationFees = await estimateGasFees(provider, signer.address, signer.address, calldata);

    hash = await walletClient.sendTransaction({
      chain: null,
      type: "eip7702",
      authorizationList: [authorization],
      to: signer.address,
      data: calldata,
      gas: 500000n,
      value: 0n,
      ...(delegationFees.maxFeePerGas != null && { maxFeePerGas: delegationFees.maxFeePerGas }),
      ...(delegationFees.maxPriorityFeePerGas != null && { maxPriorityFeePerGas: delegationFees.maxPriorityFeePerGas }),
    });
  }

  console.log("Self-sponsored transaction sent:", hash);

  const receipt = await provider.waitForTransactionReceipt({ hash });
  console.log("Receipt:", receipt);
}

main().catch((error) => {
  console.error("Error:", error);
  process.exit(1);
});
