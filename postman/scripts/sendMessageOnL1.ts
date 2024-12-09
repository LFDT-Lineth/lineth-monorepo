import { BytesLike, ContractTransactionReceipt, Overrides, Wallet, JsonRpcProvider } from "ethers";
import { config } from "dotenv";
import { L2MessageService, L2MessageService__factory, LineaRollup, LineaRollup__factory } from "@consensys/linea-sdk";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import { SendMessageArgs } from "./types";
import { sanitizeAddress, sanitizePrivKey } from "./cli";
import { encodeSendMessage } from "./helpers";

config();

const argv = yargs(hideBin(process.argv))
  .option("l1-rpc-url", {
    describe: "Rpc url",
    type: "string",
    demandOption: true,
  })
  .option("l2-rpc-url", {
    describe: "Rpc url",
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
    describe: "Signer private key on L2 from account with L1_L2_MESSAGE_SETTER_ROLE",
    type: "string",
    demandOption: false,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("l1-contract-address", {
    describe: "Smart contract address",
    type: "string",
    demandOption: true,
    coerce: sanitizeAddress("smc-address"),
  })
  .option("l2-contract-address", {
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
    describe: "Auto anchoring",
    type: "boolean",
    demandOption: false,
    default: false,
  })
  .parseSync();

const sendMessage = async (
  contract: LineaRollup,
  args: SendMessageArgs,
  overrides: Overrides = {},
): Promise<ContractTransactionReceipt | null> => {
  const tx = await contract.sendMessage(args.to, args.fee, args.calldata, overrides);
  return await tx.wait();
};

const sendMessages = async (
  contract: LineaRollup,
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

const getMessageCounter = async (contractAddress: string, signer: Wallet) => {
  const lineaRollup = LineaRollup__factory.connect(contractAddress, signer) as LineaRollup;
  return lineaRollup.nextMessageNumber();
};

const anchorMessageHashesOnL2 = async (
  lineaRollup: LineaRollup,
  l2MessageService: L2MessageService,
  messageHashes: BytesLike[],
  startingMessageNumber: bigint,
) => {
  const finalMessageNumber = startingMessageNumber + BigInt(messageHashes.length - 1);
  const rollingHashes = await lineaRollup.rollingHashes(finalMessageNumber);
  const tx = await l2MessageService.anchorL1L2MessageHashes(
    messageHashes,
    startingMessageNumber,
    finalMessageNumber,
    rollingHashes,
  );
  await tx.wait();
};

const main = async (args: typeof argv) => {
  if (args.autoAnchoring && !args.l2PrivKey) {
    console.error(
      `private key from an L2 account with L1_L2_MESSAGE_SETTER_ROLE must be given if auto-anchoring is true`,
    );
    return;
  }

  const l1Provider = new JsonRpcProvider(args.l1RpcUrl);
  const l2Provider = new JsonRpcProvider(args.l2RpcUrl);
  const l1Signer = new Wallet(args.l1PrivKey, l1Provider);

  const functionArgs: SendMessageArgs & Overrides = {
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    to: args.to,
    fee: BigInt(args.fee.toString()),
    calldata: args.calldata,
  };

  const lineaRollup = LineaRollup__factory.connect(args.l1ContractAddress, l1Signer) as LineaRollup;

  await sendMessages(lineaRollup, l1Signer, args.numberOfMessage, functionArgs, {
    value: BigInt(args.value.toString()),
  });

  // Anchor messages hash on L2
  if (!args.autoAnchoring) return;

  const nextMessageCounter = await getMessageCounter(args.l1ContractAddress, l1Signer);
  const startCounter = nextMessageCounter - BigInt(args.numberOfMessage);

  const messageHashesToAnchor: string[] = [];
  for (let i = startCounter; i < nextMessageCounter; i++) {
    const messageHash = await encodeSendMessage(
      l1Signer.address,
      args.to,
      BigInt(args.fee.toString()),
      BigInt(args.value.toString()) - BigInt(args.fee.toString()),
      BigInt(i),
      args.calldata,
    );
    console.log(messageHash);
    messageHashesToAnchor.push(messageHash);
  }

  const l2Signer = new Wallet(args.l2PrivKey!, l2Provider);
  const l2MessageService = L2MessageService__factory.connect(args.l2ContractAddress, l2Signer) as L2MessageService;
  const startingMessageNumber = startCounter;
  await anchorMessageHashesOnL2(lineaRollup, l2MessageService, messageHashesToAnchor, startingMessageNumber);
};

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
