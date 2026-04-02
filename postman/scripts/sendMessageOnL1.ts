import { config } from "dotenv";
import { Address, Hex, PublicClient, SendTransactionParameters, WalletClient, encodeFunctionData } from "viem";
import { getTransactionCount, readContract, waitForTransactionReceipt } from "viem/actions";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import { sanitizeAddress, sanitizePrivKey } from "./cli";
import { createChainContext, encodeSendMessage } from "./helpers";

config();

const SEND_MESSAGE_ABI = [
  {
    inputs: [
      { internalType: "address", name: "_to", type: "address" },
      { internalType: "uint256", name: "_fee", type: "uint256" },
      { internalType: "bytes", name: "_calldata", type: "bytes" },
    ],
    name: "sendMessage",
    outputs: [],
    stateMutability: "payable",
    type: "function",
  },
] as const;

const NEXT_MESSAGE_NUMBER_ABI = [
  {
    inputs: [],
    name: "nextMessageNumber",
    outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

const ROLLING_HASHES_ABI = [
  {
    inputs: [{ internalType: "uint256", name: "messageNumber", type: "uint256" }],
    name: "rollingHashes",
    outputs: [{ internalType: "bytes32", name: "rollingHash", type: "bytes32" }],
    stateMutability: "view",
    type: "function",
  },
] as const;

const ANCHOR_L1_L2_MESSAGE_HASHES_ABI = [
  {
    inputs: [
      { internalType: "bytes32[]", name: "_messageHashes", type: "bytes32[]" },
      { internalType: "uint256", name: "_startingMessageNumber", type: "uint256" },
      { internalType: "uint256", name: "_finalMessageNumber", type: "uint256" },
      { internalType: "bytes32", name: "_finalRollingHash", type: "bytes32" },
    ],
    name: "anchorL1L2MessageHashes",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

const argv = yargs(hideBin(process.argv))
  .option("l1-rpc-url", {
    describe: "L1 RPC URL",
    type: "string",
    demandOption: true,
  })
  .option("l2-rpc-url", {
    describe: "L2 RPC URL",
    type: "string",
    demandOption: true,
  })
  .option("l1-priv-key", {
    describe: "Signer private key on L1",
    type: "string",
    demandOption: true,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("l2-priv-key", {
    describe: "Signer private key on L2 (account with L1_L2_MESSAGE_SETTER_ROLE)",
    type: "string",
    demandOption: false,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("l1-contract-address", {
    describe: "L1 contract address",
    type: "string",
    demandOption: true,
    coerce: sanitizeAddress("smc-address"),
  })
  .option("l2-contract-address", {
    describe: "L2 contract address",
    type: "string",
    demandOption: true,
    coerce: sanitizeAddress("smc-address"),
  })
  .option("to", {
    describe: "Destination address",
    type: "string",
    demandOption: true,
    coerce: sanitizeAddress("to"),
  })
  .option("fee", {
    describe: "Fee passed to send message function",
    type: "number",
    demandOption: true,
  })
  .option("value", {
    describe: "Value passed to send message function",
    type: "number",
    demandOption: true,
  })
  .option("calldata", {
    describe: "Encoded message calldata",
    type: "string",
    demandOption: true,
  })
  .option("number-of-message", {
    describe: "Number of messages to send",
    type: "number",
    demandOption: true,
  })
  .option("auto-anchoring", {
    describe: "Auto-anchor message hashes on L2 after sending",
    type: "boolean",
    demandOption: false,
    default: false,
  })
  .parseSync();

async function sendMessages(
  client: WalletClient,
  contractAddress: Address,
  to: Address,
  fee: bigint,
  calldata: Hex,
  value: bigint,
  count: number,
) {
  let nonce = await getTransactionCount(client, { address: client.account!.address, blockTag: "latest" });
  const txData = encodeFunctionData({
    abi: SEND_MESSAGE_ABI,
    functionName: "sendMessage",
    args: [to, fee, calldata],
  });

  const promises = Array.from({ length: count }, () => {
    const tx = client.sendTransaction({
      account: client.account!,
      to: contractAddress,
      data: txData,
      value,
      nonce,
    } as SendTransactionParameters);
    nonce++;
    return tx;
  });

  const txHashes = await Promise.all(promises);

  const receipts = await Promise.all(txHashes.map((tx) => waitForTransactionReceipt(client, { hash: tx })));

  console.log("receipts", receipts.map((receipt) => [receipt.transactionHash, receipt.status]).join("\n"));
}

async function getNextMessageNumber(client: PublicClient, contractAddress: Address): Promise<bigint> {
  return readContract(client, {
    abi: NEXT_MESSAGE_NUMBER_ABI,
    functionName: "nextMessageNumber",
    address: contractAddress,
  });
}

async function anchorMessageHashesOnL2(
  l2WalletClient: WalletClient,
  l1PublicClient: PublicClient,
  l1ContractAddress: Address,
  l2ContractAddress: Address,
  messageHashes: Hex[],
  startingMessageNumber: bigint,
) {
  const finalMessageNumber = startingMessageNumber + BigInt(messageHashes.length - 1);

  const finalRollingHash = await readContract(l1PublicClient, {
    abi: ROLLING_HASHES_ABI,
    address: l1ContractAddress,
    functionName: "rollingHashes",
    args: [finalMessageNumber],
  });

  const txData = encodeFunctionData({
    abi: ANCHOR_L1_L2_MESSAGE_HASHES_ABI,
    functionName: "anchorL1L2MessageHashes",
    args: [messageHashes, startingMessageNumber, finalMessageNumber, finalRollingHash],
  });

  const txHash = await l2WalletClient.sendTransaction({
    account: l2WalletClient.account!,
    to: l2ContractAddress,
    data: txData,
    value: 0n,
  } as SendTransactionParameters);

  const receipt = await waitForTransactionReceipt(l2WalletClient, { hash: txHash });
  console.log("anchoring receipt", receipt.transactionHash, receipt.status);
}

async function main(args: typeof argv) {
  if (args.autoAnchoring && !args.l2PrivKey) {
    throw new Error("--l2-priv-key is required when --auto-anchoring is enabled (needs L1_L2_MESSAGE_SETTER_ROLE)");
  }

  const fee = BigInt(args.fee);
  const value = BigInt(args.value);
  const to = args.to as Address;
  const calldata = args.calldata as Hex;
  const l1ContractAddress = args.l1ContractAddress as Address;

  const { walletClient: l1WalletClient, publicClient: l1PublicClient } = await createChainContext(
    args.l1RpcUrl,
    args.l1PrivKey as Hex,
  );

  await sendMessages(l1WalletClient, l1ContractAddress, to, fee, calldata, value, args.numberOfMessage);

  if (!args.autoAnchoring) return;

  const { walletClient: l2WalletClient } = await createChainContext(args.l2RpcUrl, args.l2PrivKey as Hex);

  const nextMessageNumber = await getNextMessageNumber(l1PublicClient, l1ContractAddress);
  const startMessageNumber = nextMessageNumber - BigInt(args.numberOfMessage);

  const messageHashes = Array.from({ length: args.numberOfMessage }, (_, i) => {
    const messageNumber = startMessageNumber + BigInt(i);
    return encodeSendMessage(l1WalletClient.account.address, to, fee, value - fee, messageNumber, calldata);
  });

  messageHashes.forEach((hash) => console.log(hash));

  await anchorMessageHashesOnL2(
    l2WalletClient,
    l1PublicClient,
    l1ContractAddress,
    args.l2ContractAddress as Address,
    messageHashes,
    startMessageNumber,
  );
}

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
