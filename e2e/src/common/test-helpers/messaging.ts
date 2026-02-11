import { encodeFunctionCall, etherToWei } from "@consensys/linea-shared-utils";
import { randomBytes } from "crypto";
import { PrivateKeyAccount, toHex, TransactionReceipt, Hash } from "viem";

import { L2RpcEndpoint } from "../../config/clients/l2-client";
import { createTestLogger } from "../../config/logger";
import { DummyContractAbi, L2MessageServiceV1Abi } from "../../generated";
import { MINIMUM_FEE_IN_WEI } from "../constants";
import { estimateLineaGas } from "../utils/gas";

import type { TestContext } from "../../config/setup";

const logger = createTestLogger();

const DEFAULT_TRANSACTION_TIMEOUT_MS = 30_000;
const CALLDATA_BYTE_SIZE = 100;
const DEFAULT_L2_DESTINATION_ADDRESS = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";
const DEFAULT_L1_DESTINATION_ADDRESS = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

export type SendMessageParams = {
  account: PrivateKeyAccount;
  fee?: bigint;
  value?: bigint;
  withCalldata?: boolean;
  timeoutMs?: number;
};

export type SendMessageResult = {
  txHash: Hash;
  receipt: TransactionReceipt;
};

function generateCalldata(withCalldata: boolean): `0x${string}` {
  if (!withCalldata) {
    return "0x";
  }

  return encodeFunctionCall({
    abi: DummyContractAbi,
    functionName: "setPayload",
    args: [toHex(randomBytes(CALLDATA_BYTE_SIZE).toString("hex"))],
  });
}

export async function sendL1ToL2Message(context: TestContext, params: SendMessageParams): Promise<SendMessageResult> {
  const { account, fee = 0n, value = 0n, withCalldata = false, timeoutMs = DEFAULT_TRANSACTION_TIMEOUT_MS } = params;

  const l2PublicClient = context.l2PublicClient();
  const l1WalletClient = context.l1WalletClient({ account });
  const l1PublicClient = context.l1PublicClient();

  const dummyContract = context.l2Contracts.dummyContract(l2PublicClient);
  const lineaRollup = context.l1Contracts.lineaRollup(l1WalletClient);

  const calldata = generateCalldata(withCalldata);
  const destinationAddress = withCalldata ? dummyContract.address : DEFAULT_L2_DESTINATION_ADDRESS;

  const { maxPriorityFeePerGas, maxFeePerGas } = await l1PublicClient.estimateFeesPerGas();

  logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

  const txHash = await lineaRollup.write.sendMessage([destinationAddress, fee, calldata], {
    value,
    maxPriorityFeePerGas,
    maxFeePerGas,
  });

  logger.debug(`sendMessage transaction sent. transactionHash=${txHash}`);

  logger.debug(`Waiting for transaction to be mined... transactionHash=${txHash}`);
  const receipt = await l1PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: timeoutMs });

  if (!receipt) {
    throw new Error(`Transaction receipt not received for hash: ${txHash}`);
  }

  if (receipt.status !== "success") {
    logger.error(`L1 transaction reverted. transactionHash=${txHash} status=${receipt.status}`);
  } else {
    logger.debug(`Transaction mined. transactionHash=${txHash} status=${receipt.status}`);
  }

  return { txHash, receipt };
}

export async function sendL2ToL1Message(context: TestContext, params: SendMessageParams): Promise<SendMessageResult> {
  const {
    account,
    fee = MINIMUM_FEE_IN_WEI,
    value = etherToWei("0.001"),
    withCalldata = false,
    timeoutMs = DEFAULT_TRANSACTION_TIMEOUT_MS,
  } = params;

  const l1PublicClient = context.l1PublicClient();
  const l2WalletClient = context.l2WalletClient({ account });
  const l2PublicClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });

  const dummyContract = context.l1Contracts.dummyContract(l1PublicClient);
  const l2MessageService = context.l2Contracts.l2MessageService(l2WalletClient);

  const calldata = generateCalldata(withCalldata);
  const destinationAddress = withCalldata ? dummyContract.address : DEFAULT_L1_DESTINATION_ADDRESS;

  const { maxPriorityFeePerGas, maxFeePerGas, gasLimit } = await estimateLineaGas(l2PublicClient, {
    account: account,
    to: l2MessageService.address,
    data: encodeFunctionCall({
      abi: L2MessageServiceV1Abi,
      functionName: "sendMessage",
      args: [destinationAddress, fee, calldata],
    }),
    value,
  });

  logger.debug(
    `Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas} gasLimit=${gasLimit}`,
  );

  const txHash = await l2MessageService.write.sendMessage([destinationAddress, fee, calldata], {
    value,
    maxPriorityFeePerGas,
    maxFeePerGas,
    gas: gasLimit,
  });

  logger.debug(`sendMessage transaction sent. transactionHash=${txHash}`);

  logger.debug(`Waiting for transaction to be mined... transactionHash=${txHash}`);
  const receipt = await l2PublicClient.waitForTransactionReceipt({ hash: txHash, timeout: timeoutMs });

  if (!receipt) {
    throw new Error(`Transaction receipt not received for hash: ${txHash}`);
  }

  if (receipt.status !== "success") {
    logger.error(`L2 transaction reverted. transactionHash=${txHash} status=${receipt.status}`);
  } else {
    logger.debug(`Transaction mined. transactionHash=${txHash} status=${receipt.status}`);
  }

  return { txHash, receipt };
}
