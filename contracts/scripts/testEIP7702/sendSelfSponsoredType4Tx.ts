import { ethers } from "ethers";
import { requireEnv, checkDelegation, getAccountInfo, createAuthorization, estimateGasFees } from "../utils";
import * as dotenv from "dotenv";

// Self-sponsored EIP-7702 transaction: the same wallet signs the authorization AND sends the transaction.
// Prerequisite - Deploy a contract with NON-VIEW initialize() function, e.g. TestEIP7702Delegation
// Use this contract for TARGET_ADDRESS env
//
// TARGET_ADDRESS is optional. If omitted (or set to 0x0), the authorization is set to the zero
// address, which removes any existing EIP-7702 delegation from the signer's EOA per the spec.

// RPC_URL=<> DEPLOYER_PRIVATE_KEY=<> [TARGET_ADDRESS=<>] npx hardhat run scripts/testEIP7702/sendSelfSponsoredType4Tx.ts

dotenv.config();

const SHORT_ZERO_ADDRESS = "0x0";

async function main() {
  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
  const targetAddressEnv = process.env.TARGET_ADDRESS?.trim();
  const targetAddress =
    targetAddressEnv == null || targetAddressEnv === SHORT_ZERO_ADDRESS ? ethers.ZeroAddress : targetAddressEnv;

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const signer = new ethers.Wallet(privateKey, provider);

  const signerInfo = await getAccountInfo(provider, signer.address);
  console.log("Signer info:", signerInfo);

  const delegationStatus = await checkDelegation(provider, signer.address);
  console.log("EOA delegation status:", delegationStatus);

  const isRevocation = targetAddress === ethers.ZeroAddress;

  console.log("\n=== SELF-SPONSORED EIP-7702 TRANSACTION ===");
  console.log("Same wallet signs authorization and sends the transaction.");
  if (isRevocation) {
    console.log("TARGET_ADDRESS not set or set to 0x0 - sending revocation (zero address) to remove delegation.");
  }

  // +1 nonce offset: sender nonce is incremented before authorization processing
  const authorization = await createAuthorization(signer, provider, targetAddress, 1);

  let tx;
  if (isRevocation) {
    // No calldata to execute when revoking - the authorization list alone carries the intent.
    // Use provider.getFeeData() directly; linea_estimateGas rejects empty-calldata self-sends.
    const feeData = await provider.getFeeData();
    tx = await signer.sendTransaction({
      type: 4,
      authorizationList: [authorization],
      to: signer.address,
      data: "0x",
      gasLimit: 500000n,
      value: 0n,
      ...(feeData.maxFeePerGas != null && { maxFeePerGas: feeData.maxFeePerGas }),
      ...(feeData.maxPriorityFeePerGas != null && { maxPriorityFeePerGas: feeData.maxPriorityFeePerGas }),
    });
  } else {
    const ABI = ["function initialize() external"];
    const delegatedContract = new ethers.Contract(signer.address, ABI, signer);

    const calldata = delegatedContract.interface.encodeFunctionData("initialize");
    const delegationFees = await estimateGasFees(provider, rpcUrl, signer.address, signer.address, calldata);

    const txParams = {
      type: 4,
      authorizationList: [authorization],
      gasLimit: 500000n,
      value: 0n,
      ...(delegationFees.maxFeePerGas != null && { maxFeePerGas: delegationFees.maxFeePerGas }),
      ...(delegationFees.maxPriorityFeePerGas != null && { maxPriorityFeePerGas: delegationFees.maxPriorityFeePerGas }),
    };

    tx = await delegatedContract["initialize()"](txParams);
  }

  console.log("Self-sponsored transaction sent:", tx.hash);

  const receipt = await tx.wait();
  console.log("Receipt:", receipt);
}

main().catch((error) => {
  console.error("Error:", error);
  process.exit(1);
});
