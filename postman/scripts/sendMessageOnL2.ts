import { config } from "dotenv";
import { Address, Hex, SendTransactionParameters, WalletClient, encodeFunctionData } from "viem";
import { getTransactionCount } from "viem/actions";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import { sanitizeAddress, sanitizePrivKey } from "./cli";
import { createChainContext } from "./helpers";

type SendMessageArgs = {
  to: Address;
  fee: bigint;
  calldata: Hex;
};

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

const argv = yargs(hideBin(process.argv))
  .option("rpc-url", {
    describe: "L2 RPC URL",
    type: "string",
    demandOption: true,
  })
  .option("priv-key", {
    describe: "Signer private key",
    type: "string",
    demandOption: true,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("contract-address", {
    describe: "L2MessageService contract address",
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
    describe: "Fee passed to send message function (in wei)",
    type: "string",
    demandOption: true,
  })
  .option("value", {
    describe: "Value (ETH in wei) sent with the transaction",
    type: "string",
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
  .parseSync();

async function sendMessages(
  client: WalletClient,
  contractAddress: Address,
  args: SendMessageArgs,
  count: number,
  value: bigint,
) {
  let nonce = await getTransactionCount(client, { address: client.account!.address, blockTag: "latest" });

  const txData = encodeFunctionData({
    abi: SEND_MESSAGE_ABI,
    functionName: "sendMessage",
    args: [args.to, args.fee, args.calldata],
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

  await Promise.all(promises);
}

async function main(args: typeof argv) {
  const { walletClient } = await createChainContext(args.rpcUrl, args.privKey as Hex);

  const messageArgs: SendMessageArgs = {
    to: args.to as Address,
    fee: BigInt(args.fee),
    calldata: args.calldata as Hex,
  };

  await sendMessages(
    walletClient,
    args.contractAddress as Address,
    messageArgs,
    args.numberOfMessage,
    BigInt(args.value),
  );
}

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
