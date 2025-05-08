// Manual script to run on devnet & Sepolia deployments to test various scenarios

/** USAGE

npx ts-node postman/scripts/manualTestSendMessages.ts \
  --rpc-url=<INSERT_STRING> \
  --priv-key=<INSERT_STRING> \
  --deployment-env=<devnet or sepolia>

 */

/**
 * Test Scenarios
 * --------------
 *
 * (Below MAX_POSTMAN_SPONSOR_GAS_LIMIT or 250_000 gas used):
 *
 * 1. Fee = 0
 * Should be auto-claimed on L2
 *
 * 2. Fee = Underpriced
 * Should be auto-claimed on L2
 *
 * 3. Fee = correctly priced
 * Should be auto-claimed on L2
 *
 * (Above MAX_POSTMAN_SPONSOR_GAS_LIMIT or 250_000 gas used):
 *
 * 4. Fee = 0
 * Should not be auto-claimed on L2, and log 'Found message with zero fee'
 *
 * 5. Fee = Underpriced
 * Should not be auto-claimed on L2, and log 'Fee underpriced found in this message'
 *
 * 6. Fee = correctly priced
 * Should be auto-claimed on L2
 */

import { config } from "dotenv";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";
import { sanitizePrivKey } from "./cli";
import { Wallet, JsonRpcProvider, ContractTransactionResponse } from "ethers";
import { LineaRollup, LineaRollup__factory } from "@consensys/linea-sdk";
import { SendMessageArgs } from "./types";
import { encodeSendMessage } from "./helpers";

// CONFIG INPUT
config();
const argv = yargs(hideBin(process.argv))
  .option("rpc-url", {
    describe: "Sepolia URL",
    type: "string",
    demandOption: true,
  })
  .option("priv-key", {
    describe: "Signer private key on Sepolia",
    type: "string",
    demandOption: true,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("deployment-env", {
    describe: "Deployment environment",
    type: "string",
    choices: ["devnet", "sepolia"],
    demandOption: true,
  })
  .parseSync();

// TYPES
type TestScenario = {
  logString: string;
  fee: bigint;
  calldata: string;
};

type DeploymentEnv = "devnet" | "sepolia";

// VARIABLES
const ZERO_FEE = 0n;
const UNDERPRICED_FEE = 1n;
const CORRECTLY_PRICED_FEE = 10000000000000000n; // 0.01 ETH, conservatively high estimate
const NO_CALLDATA = "0x";
const SPAM_CALLDATA = "0xdead".padEnd(40964, "dead");

const L1_MESSAGE_SERVICE_ADDRESS: Record<DeploymentEnv, string> = {
  devnet: "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48",
  sepolia: "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5",
};

const testScenarios: TestScenario[] = [
  // Below 250,000 gas used
  {
    logString: "Case 1: <250_000 gas, 0 fee => Should be auto-claimed on L2",
    fee: ZERO_FEE,
    calldata: NO_CALLDATA,
  },
  {
    logString: "Case 2: <250_000 gas, underpriced fee => Should be auto-claimed on L2",
    fee: UNDERPRICED_FEE,
    calldata: NO_CALLDATA,
  },
  {
    logString: "Case 3: <250_000 gas, correct fee => Should be auto-claimed on L2",
    fee: CORRECTLY_PRICED_FEE,
    calldata: NO_CALLDATA,
  },

  // Above 250,000 gas used
  {
    logString:
      "Case 4: >250_000 gas, 0 fee => Should NOT be auto-claimed, and should find Postman log 'Found message with zero fee'",
    fee: ZERO_FEE,
    calldata: SPAM_CALLDATA,
  },
  {
    logString:
      "Case 5: >250_000 gas, underpriced fee => Should NOT be auto-claimed, and should find Postman log 'Fee underpriced found in this message'",
    fee: UNDERPRICED_FEE,
    calldata: SPAM_CALLDATA,
  },
  {
    logString: "Case 6: >250_000 gas, correct fee => Should be auto-claimed on L2",
    fee: CORRECTLY_PRICED_FEE,
    calldata: SPAM_CALLDATA,
  },
];

// LOCAL FUNCTIONS
const sendMessage = async (
  deploymentEnv: DeploymentEnv,
  sender: Wallet,
  args: SendMessageArgs,
  messageNonce: bigint,
  senderNonce: number,
  logString: string,
): Promise<ContractTransactionResponse> => {
  const lineaRollup = LineaRollup__factory.connect(L1_MESSAGE_SERVICE_ADDRESS[deploymentEnv], sender) as LineaRollup;
  const tx = await lineaRollup.sendMessage(args.to, args.fee, args.calldata, { value: args.fee, nonce: senderNonce });
  const messageHash = encodeSendMessage(
    sender.address,
    args.to,
    BigInt(args.fee.toString()),
    0n,
    BigInt(messageNonce),
    String(args.calldata),
  );

  console.log(`Sent message with messageHash=${messageHash}. ${logString}`);
  return tx;
};

const sendMessages = async (deploymentEnv: DeploymentEnv, sender: Wallet, testScenarios: TestScenario[]) => {
  const nextSenderNonce = await sender.getNonce();
  const nextMessageCounter = await getMessageCounter(deploymentEnv, sender);

  // If we send concurrently, we have high risk of sending nonces out of order
  for (let i = 0; i < testScenarios.length; i++) {
    const s = testScenarios[i];
    const functionArgs: SendMessageArgs = {
      to: sender.address,
      fee: s.fee,
      calldata: s.calldata,
    };
    await sendMessage(
      deploymentEnv,
      sender,
      functionArgs,
      nextMessageCounter + BigInt(i),
      nextSenderNonce + i,
      s.logString,
    );
  }
};

const getMessageCounter = async (deploymentEnv: DeploymentEnv, signer: Wallet) => {
  const lineaRollup = LineaRollup__factory.connect(L1_MESSAGE_SERVICE_ADDRESS[deploymentEnv], signer) as LineaRollup;
  return lineaRollup.nextMessageNumber();
};

// MAIN SCRIPT

const main = async (args: typeof argv) => {
  const l1Provider = new JsonRpcProvider(args.rpcUrl);
  const l1Signer = new Wallet(args.privKey, l1Provider);
  await sendMessages(args.deploymentEnv as DeploymentEnv, l1Signer, testScenarios);
};

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
