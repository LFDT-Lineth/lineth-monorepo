import * as dotenv from "dotenv";
import { createPublicClient, createWalletClient, http, isHex } from "viem";
import { privateKeyToAccount } from "viem/accounts";

import { checkDelegation, getAccountInfo, estimateGasFees } from "./utils.js";
import { parseChecksummedAddress } from "../../common/helpers/addressRegistry.js";
import { getRequiredEnvVar } from "../../common/helpers/environment.js";
import { waitForSuccessfulReceipt } from "../../common/helpers/receipt.js";
import { parsePrivateKey } from "../cli.js";

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

// RPC_URL=<> SPONSOR_PRIVATE_KEY=<> AUTHORITY_PRIVATE_KEY=<> TARGET_ADDRESS=<> pnpm exec hardhat run scripts/testEIP7702/sendSponsoredType4Tx.ts

dotenv.config();

async function main() {
  const rpcUrl = getRequiredEnvVar("RPC_URL");
  const targetAddress = parseChecksummedAddress(getRequiredEnvVar("TARGET_ADDRESS"), "TARGET_ADDRESS");
  const calldata = process.env.CALLDATA ?? "0x";
  if (!isHex(calldata)) {
    throw new Error("CALLDATA must be a hex string");
  }

  const provider = createPublicClient({ transport: http(rpcUrl) });
  const sponsor = privateKeyToAccount(parsePrivateKey(getRequiredEnvVar("SPONSOR_PRIVATE_KEY"), "SPONSOR_PRIVATE_KEY"));
  const authority = privateKeyToAccount(
    parsePrivateKey(getRequiredEnvVar("AUTHORITY_PRIVATE_KEY"), "AUTHORITY_PRIVATE_KEY"),
  );
  const sponsorWallet = createWalletClient({ account: sponsor, transport: http(rpcUrl) });
  const toAddress = process.env.TO_ADDRESS
    ? parseChecksummedAddress(process.env.TO_ADDRESS, "TO_ADDRESS")
    : authority.address;

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

  // The authority is not the sender, so its current nonce is signed as is.
  const authorization = await sponsorWallet.signAuthorization({ account: authority, contractAddress: targetAddress });

  const fees = await estimateGasFees(provider, sponsor.address, toAddress, calldata);
  const nonce = await provider.getTransactionCount({ address: sponsor.address });

  const hash = await sponsorWallet.sendTransaction({
    chain: null,
    type: "eip7702",
    authorizationList: [authorization],
    to: toAddress,
    data: calldata,
    nonce,
    gas: 500000n,
    value: 0n,
    ...fees,
  });
  console.log("Sponsored transaction sent:", hash);

  const receipt = await waitForSuccessfulReceipt(provider, hash);
  console.log("Receipt:", receipt);
}

main().catch((error) => {
  console.error("Error:", error);
  process.exit(1);
});
