import * as dotenv from "dotenv";
import { ethers } from "ethers";

import { requireEnv, checkDelegation, getAccountInfo, createAuthorization, estimateGasFees } from "../utils";

// Sponsored EIP-7702 transaction: one wallet (authority) signs the authorization,
// a different wallet (sponsor) sends and pays for the transaction.
//
// Required env vars:
//   RPC_URL              - RPC endpoint
//   SPONSOR_PRIVATE_KEY  - private key of the account that sends and pays for the tx
//   AUTHORITY_PRIVATE_KEY - private key of the account that signs the authorization
//   TARGET_ADDRESS       - contract address to delegate to (set in authorization)
//
// Optional env vars:
//   TO_ADDRESS           - transaction recipient; defaults to authority address
//   CALLDATA             - hex-encoded calldata; defaults to 0x

// RPC_URL=<> SPONSOR_PRIVATE_KEY=<> AUTHORITY_PRIVATE_KEY=<> TARGET_ADDRESS=<> npx hardhat run scripts/testEIP7702/sendSponsoredType4Tx.ts

dotenv.config();

async function main() {
  const rpcUrl = requireEnv("RPC_URL");
  const sponsorPrivateKey = requireEnv("SPONSOR_PRIVATE_KEY");
  const authorityPrivateKey = requireEnv("AUTHORITY_PRIVATE_KEY");
  const targetAddress = requireEnv("TARGET_ADDRESS");
  const calldata = process.env.CALLDATA ?? "0x";

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const sponsor = new ethers.Wallet(sponsorPrivateKey, provider);
  const authority = new ethers.Wallet(authorityPrivateKey, provider);
  const toAddress = process.env.TO_ADDRESS ?? authority.address;

  console.log("Sponsor info:", await getAccountInfo(provider, sponsor.address));
  console.log("Authority info:", await getAccountInfo(provider, authority.address));

  const delegationStatus = await checkDelegation(provider, authority.address);
  console.log("Authority delegation status:", delegationStatus);

  console.log("\n=== SPONSORED EIP-7702 TRANSACTION ===");
  console.log(`Sponsor: ${sponsor.address}`);
  console.log(`Authority: ${authority.address}`);
  console.log(`Delegation target: ${targetAddress}`);
  console.log(`Tx to: ${toAddress}`);
  console.log(`Calldata: ${calldata}`);

  // +0 nonce offset: authority is not the sender, so its nonce is not incremented before auth processing
  const authorization = await createAuthorization(authority, provider, targetAddress, 0);

  const { maxFeePerGas, maxPriorityFeePerGas } = await estimateGasFees(
    provider,
    rpcUrl,
    sponsor.address,
    toAddress,
    calldata,
  );
  const nonce = await provider.getTransactionCount(sponsor.address);

  const tx = await sponsor.sendTransaction({
    type: 4,
    authorizationList: [authorization],
    to: toAddress,
    data: calldata,
    nonce,
    gasLimit: 500000n,
    value: 0n,
    ...(maxFeePerGas != null && { maxFeePerGas }),
    ...(maxPriorityFeePerGas != null && { maxPriorityFeePerGas }),
  });
  console.log("Sponsored transaction sent:", tx.hash);

  const receipt = await tx.wait();
  console.log("Receipt:", receipt);
}

main().catch((error) => {
  console.error("Error:", error);
  process.exit(1);
});
