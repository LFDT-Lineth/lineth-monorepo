import { L2MessageService, L2MessageService__factory } from "@consensys/linea-sdk";
import { config } from "dotenv";
import { ContractTransactionReceipt, Overrides, JsonRpcProvider, Wallet } from "ethers";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import { sanitizeAddress, sanitizePrivKey } from "./cli";
import { SendMessageArgs } from "./types";

config();

const argv = yargs(hideBin(process.argv))
  .option("rpc-url", {
    describe: "Rpc url",
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
    describe: "Smart contract address",
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
    type: "string",
  })
  .option("value", {
    describe: "Value passed to send message function",
    type: "string",
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

const sendMessage = async (
  contract: L2MessageService,
  args: SendMessageArgs,
  overrides: Overrides = {},
): Promise<ContractTransactionReceipt | null> => {
  const tx = await contract.sendMessage(args.to, args.fee, args.calldata, overrides);
  return await tx.wait();
};

const sendMessages = async (
  contract: L2MessageService,
  signer: Wallet,
  numberOfMessages: number,
  args: SendMessageArgs,
  overrides?: Overrides,
) => {
  let nonce = await signer.getNonce();
  const sendMessagePromises: Promise<ContractTransactionReceipt | null>[] = [];

  for (let i = 0; i < numberOfMessages; i++) {
    sendMessagePromises.push(
      sendMessage(contract, args, {
        ...overrides,
        nonce,
      }),
    );
    nonce++;
  }

  await Promise.all(sendMessagePromises);
};

const main = async (args: typeof argv) => {
  const provider = new JsonRpcProvider(args.rpcUrl);
  const signer = new Wallet(args.privKey, provider);

  const functionArgs: SendMessageArgs & Overrides = {
    to: args.to,
    fee: BigInt(args.fee!),
    calldata: args.calldata,
    value: args.value,
  };

  const l2MessageService = L2MessageService__factory.connect(args.contractAddress, signer) as L2MessageService;

  await sendMessages(l2MessageService, signer, args.numberOfMessage, functionArgs, { value: args.value });
};

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
