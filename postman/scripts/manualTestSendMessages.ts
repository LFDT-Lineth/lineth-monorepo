/**
 * Manual script to run on devnet & Sepolia deployments to test various scenarios.
 *
 * USAGE:
 *   npx ts-node postman/scripts/manualTestSendMessages.ts \
 *     --rpc-url=<INSERT_STRING> \
 *     --priv-key=<INSERT_STRING> \
 *     --deployment-env=<devnet or sepolia>
 *
 * Test Scenarios
 * --------------
 *
 * Below MAX_POSTMAN_SPONSOR_GAS_LIMIT (250,000 gas):
 *   1. Fee = 0             → auto-claimed on L2
 *   2. Fee = underpriced   → auto-claimed on L2
 *   3. Fee = correct       → auto-claimed on L2
 *
 * Above MAX_POSTMAN_SPONSOR_GAS_LIMIT (250,000 gas):
 *   4. Fee = 0             → NOT auto-claimed; log "Found message with zero fee"
 *   5. Fee = underpriced   → NOT auto-claimed; log "Fee underpriced found in this message"
 *   6. Fee = correct       → auto-claimed on L2
 */

import { config } from "dotenv";
import { Address, Hex, PublicClient, SendTransactionParameters, WalletClient, encodeFunctionData } from "viem";
import { getTransactionCount, readContract } from "viem/actions";
import yargs from "yargs";
import { hideBin } from "yargs/helpers";

import { sanitizePrivKey } from "./cli";
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

const argv = yargs(hideBin(process.argv))
  .option("rpc-url", {
    describe: "L1 RPC URL",
    type: "string",
    demandOption: true,
  })
  .option("priv-key", {
    describe: "Signer private key on L1",
    type: "string",
    demandOption: true,
    coerce: sanitizePrivKey("priv-key"),
  })
  .option("deployment-env", {
    describe: "Deployment environment",
    type: "string",
    choices: ["devnet", "sepolia"] as const,
    demandOption: true,
  })
  .parseSync();

type DeploymentEnv = "devnet" | "sepolia";

type TestScenario = {
  description: string;
  fee: bigint;
  calldata: Hex;
};

const ZERO_FEE = 0n;
const UNDERPRICED_FEE = 1n;
const CORRECTLY_PRICED_FEE = 10_000_000_000_000_000n; // 0.01 ETH
const NO_CALLDATA: Hex = "0x";
const SPAM_CALLDATA = "0xdead".padEnd(40964, "dead") as Hex;

const L1_MESSAGE_SERVICE_ADDRESS: Record<DeploymentEnv, Address> = {
  devnet: "0x2A5CDCfc38856e2590E9Bd32F54Fa348e5De5f48",
  sepolia: "0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5",
};

const TEST_SCENARIOS: TestScenario[] = [
  {
    description: "Case 1: <250k gas, 0 fee → should be auto-claimed on L2",
    fee: ZERO_FEE,
    calldata: NO_CALLDATA,
  },
  {
    description: "Case 2: <250k gas, underpriced fee → should be auto-claimed on L2",
    fee: UNDERPRICED_FEE,
    calldata: NO_CALLDATA,
  },
  {
    description: "Case 3: <250k gas, correct fee → should be auto-claimed on L2",
    fee: CORRECTLY_PRICED_FEE,
    calldata: NO_CALLDATA,
  },
  {
    description: "Case 4: >250k gas, 0 fee → should NOT be auto-claimed (log: 'Found message with zero fee')",
    fee: ZERO_FEE,
    calldata: SPAM_CALLDATA,
  },
  {
    description:
      "Case 5: >250k gas, underpriced fee → should NOT be auto-claimed (log: 'Fee underpriced found in this message')",
    fee: UNDERPRICED_FEE,
    calldata: SPAM_CALLDATA,
  },
  {
    description: "Case 6: >250k gas, correct fee → should be auto-claimed on L2",
    fee: CORRECTLY_PRICED_FEE,
    calldata: SPAM_CALLDATA,
  },
];

async function getNextMessageNumber(client: PublicClient, contractAddress: Address): Promise<bigint> {
  return readContract(client, {
    abi: NEXT_MESSAGE_NUMBER_ABI,
    functionName: "nextMessageNumber",
    address: contractAddress,
  });
}

async function runTestScenarios(
  walletClient: WalletClient,
  publicClient: PublicClient,
  contractAddress: Address,
  scenarios: TestScenario[],
) {
  let senderNonce = await getTransactionCount(walletClient, {
    address: walletClient.account!.address,
    blockTag: "latest",
  });
  let messageNonce = await getNextMessageNumber(publicClient, contractAddress);
  const senderAddress = walletClient.account!.address;

  for (const scenario of scenarios) {
    const txData = encodeFunctionData({
      abi: SEND_MESSAGE_ABI,
      functionName: "sendMessage",
      args: [senderAddress, scenario.fee, scenario.calldata],
    });

    await walletClient.sendTransaction({
      account: walletClient.account!,
      to: contractAddress,
      data: txData,
      value: scenario.fee,
      nonce: senderNonce,
    } as SendTransactionParameters);

    const messageHash = encodeSendMessage(
      senderAddress,
      senderAddress,
      scenario.fee,
      0n,
      messageNonce,
      scenario.calldata,
    );

    console.log(`Sent message with messageHash=${messageHash}. ${scenario.description}`);
    senderNonce++;
    messageNonce++;
  }
}

async function main(args: typeof argv) {
  const deploymentEnv = args.deploymentEnv as DeploymentEnv;
  const contractAddress = L1_MESSAGE_SERVICE_ADDRESS[deploymentEnv];

  const { walletClient, publicClient } = await createChainContext(args.rpcUrl, args.privKey as Hex);

  await runTestScenarios(walletClient, publicClient, contractAddress, TEST_SCENARIOS);
}

main(argv)
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
