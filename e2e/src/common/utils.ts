import * as fs from "fs";
import assert from "assert";
import {
  AbstractSigner,
  BaseContract,
  BlockTag,
  TransactionReceipt,
  TransactionRequest,
  Wallet,
  ethers,
  toBeHex,
} from "ethers";
import path from "path";
import { exec } from "child_process";
import { L2MessageServiceV1 as L2MessageService, TokenBridgeV1_1 as TokenBridge, LineaRollupV6 } from "../typechain";
import { PayableOverrides, TypedContractEvent, TypedDeferredTopicFilter, TypedEventLog } from "../typechain/common";
import { MessageEvent, SendMessageArgs } from "./types";
import { createTestLogger } from "../config/logger";
import { randomUUID, randomInt } from "crypto";
import { config } from "../config/tests-config";

const logger = createTestLogger();

export function etherToWei(amount: string): bigint {
  return ethers.parseEther(amount.toString());
}

export const wait = (timeout: number) => new Promise((resolve) => setTimeout(resolve, timeout));

export function increaseDate(currentDate: Date, seconds: number): Date {
  const newDate = new Date(currentDate.getTime());
  newDate.setSeconds(newDate.getSeconds() + seconds);
  return newDate;
}

export const subtractSecondsToDate = (date: Date, seconds: number): Date => {
  const dateCopy = new Date(date);
  dateCopy.setSeconds(date.getSeconds() - seconds);
  return dateCopy;
};

export function getWallet(privateKey: string, provider: ethers.JsonRpcProvider) {
  return new ethers.Wallet(privateKey, provider);
}

export function encodeFunctionCall(contractInterface: ethers.Interface, functionName: string, args: unknown[]) {
  return contractInterface.encodeFunctionData(functionName, args);
}

export const generateKeccak256 = (types: string[], values: unknown[], packed?: boolean) =>
  ethers.keccak256(encodeData(types, values, packed));

export const encodeData = (types: string[], values: unknown[], packed?: boolean) => {
  if (packed) {
    return ethers.solidityPacked(types, values);
  }
  return ethers.AbiCoder.defaultAbiCoder().encode(types, values);
};

export async function isSendBundleMethodNotFound(rpcEndpoint: URL, targetBlockNumber = "0xffff") {
  const lineaSendBundleClient = new LineaBundleClient(rpcEndpoint);
  try {
    await lineaSendBundleClient.lineaSendBundle([], generateRandomUUIDv4(), targetBlockNumber);
  } catch (err) {
    if (err instanceof Error) {
      if (err.message === "Method not found") {
        return true;
      }
    }
  }
  return false;
}

export function generateRandomInt(max = 1000): number {
  return randomInt(max);
}

export function generateRandomUUIDv4(): string {
  return randomUUID();
}

export class AwaitUntilTimeoutError extends Error {
  constructor(public readonly timeoutMs: number) {
    super(`awaitUntil timed out after ${timeoutMs}ms`);
    this.name = "AwaitUntilTimeoutError";
  }
}

export async function awaitUntil<T>(
  callback: () => Promise<T>,
  stopRetry: (value: T) => boolean,
  pollingIntervalMs = 500,
  timeoutMs = 2 * 60 * 1000,
): Promise<T> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    const result = await callback();

    if (stopRetry(result)) {
      return result;
    }

    await wait(pollingIntervalMs);
  }

  throw new AwaitUntilTimeoutError(timeoutMs);
}

export async function pollForBlockNumber(
  provider: ethers.JsonRpcProvider,
  expectedBlockNumber: number,
  pollingIntervalMs: number = 500,
  timeoutMs: number = 2 * 60 * 1000,
): Promise<boolean> {
  try {
    await awaitUntil(
      async () => await provider.getBlockNumber(),
      (a: number) => a >= expectedBlockNumber,
      pollingIntervalMs,
      timeoutMs,
    );
    return true;
  } catch (error) {
    if (error instanceof AwaitUntilTimeoutError) {
      logger.error(`Timeout waiting for block number ${expectedBlockNumber} after ${error.timeoutMs}ms`);
      return false;
    }
    throw error;
  }
}

export class RollupGetZkEVMBlockNumberClient {
  private endpoint: URL;
  private request = {
    method: "post",
    body: JSON.stringify({
      jsonrpc: "2.0",
      method: "rollup_getZkEVMBlockNumber",
      params: [],
      id: generateRandomInt(),
    }),
  };

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async rollupGetZkEVMBlockNumber(): Promise<number> {
    const response = await fetch(this.endpoint, this.request);
    const data = await response.json();
    assert("result" in data);
    return Number.parseInt(data.result);
  }
}

export class GetEthLogsClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async getLogs(
    address: string,
    topics: Array<null | string | Array<string>>,
    fromBlock: BlockTag,
    toBlock: BlockTag,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ): Promise<any> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "eth_getLogs",
        params: [
          {
            topics: [...topics],
            fromBlock,
            toBlock,
            address,
          },
        ],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    return await response.json();
  }
}

export class LineaEstimateGasClient {
  private endpoint: URL;
  private BASE_FEE_MULTIPLIER = 1.35;
  private PRIORITY_FEE_MULTIPLIER = 1.05;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async lineaEstimateGas(
    from: string,
    to?: string,
    data: string = "0x",
    value: string = "0x0",
  ): Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint; gasLimit: bigint }> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_estimateGas",
        params: [
          {
            from,
            to,
            data,
            value,
          },
        ],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();
    assert("result" in responseJson);

    const baseFeePerGas = this.getValueFromMultiplier(
      BigInt(responseJson.result.baseFeePerGas),
      this.BASE_FEE_MULTIPLIER,
    );
    const maxPriorityFeePerGas = this.getValueFromMultiplier(
      BigInt(responseJson.result.priorityFeePerGas),
      this.PRIORITY_FEE_MULTIPLIER,
    );

    return {
      maxFeePerGas: baseFeePerGas + maxPriorityFeePerGas,
      maxPriorityFeePerGas,
      gasLimit: BigInt(responseJson.result.gasLimit),
    };
  }

  private getValueFromMultiplier(value: bigint, multiplier: number): bigint {
    return (value * BigInt(multiplier * 100)) / 100n;
  }
}

export class LineaBundleClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async lineaSendBundle(
    txs: string[],
    replacementUUID: string,
    blockNumber: string,
  ): Promise<{ bundleHash: string }> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_sendBundle",
        params: [
          {
            txs,
            replacementUUID,
            blockNumber,
          },
        ],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();
    if (responseJson.error?.code === -32601 && responseJson.error?.message === "Method not found") {
      throw Error("Method not found");
    }
    assert("result" in responseJson);
    return {
      bundleHash: responseJson.result.bundleHash,
    };
  }

  public async lineaCancelBundle(replacementUUID: string): Promise<boolean> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_cancelBundle",
        params: [replacementUUID],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();
    if (responseJson.error?.code === -32601 && responseJson.error?.message === "Method not found") {
      throw Error("Method not found");
    }
    assert("result" in responseJson);
    return responseJson.result;
  }
}

export class LineaShomeiClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async rollupGetZkEVMStateMerkleProofV0(
    startBlockNumber: number,
    endBlockNumber: number,
    zkStateManagerVersion: string,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ): Promise<any> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "rollup_getZkEVMStateMerkleProofV0",
        params: [
          {
            startBlockNumber,
            endBlockNumber,
            zkStateManagerVersion,
          },
        ],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();
    assert("result" in responseJson);
    return responseJson;
  }
}

export class LineaShomeiFrontendClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async lineaGetProof(
    address: string,
    storageKeys: string[] = [],
    blockParameter: string = "latest",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ): Promise<any> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_getProof",
        params: [address, storageKeys, blockParameter],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    return await response.json();
  }
}

export class TransactionExclusionClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  public async getTransactionExclusionStatusV1(txHash: string): Promise<any> {
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_getTransactionExclusionStatusV1",
        params: [txHash],
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    return await response.json();
  }

  public async saveRejectedTransactionV1(
    txRejectionStage: string,
    timestamp: string, // ISO-8601
    blockNumber: number | null,
    transactionRLP: string,
    reasonMessage: string,
    overflows: { module: string; count: number; limit: number }[],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
  ): Promise<any> {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let params: any = {
      txRejectionStage,
      timestamp,
      transactionRLP,
      reasonMessage,
      overflows,
    };
    if (blockNumber != null) {
      params = {
        ...params,
        blockNumber,
      };
    }
    const request = {
      method: "post",
      body: JSON.stringify({
        jsonrpc: "2.0",
        method: "linea_saveRejectedTransactionV1",
        params: params,
        id: generateRandomInt(),
      }),
    };
    const response = await fetch(this.endpoint, request);
    return await response.json();
  }
}

export async function getRawTransactionHex(txRequest: TransactionRequest, signer: Wallet): Promise<string> {
  const rawTransaction = await signer.populateTransaction(txRequest);
  return await signer.signTransaction(rawTransaction);
}

export async function getTransactionHash(txRequest: TransactionRequest, signer: Wallet): Promise<string> {
  const signature = await getRawTransactionHex(txRequest, signer);
  return ethers.keccak256(signature);
}

export async function getBlockByNumberOrBlockTag(
  rpcUrl: URL,
  blockTag: BlockTag,
  prefetchTxs: boolean = false,
): Promise<ethers.Block | null> {
  const provider = new ethers.JsonRpcProvider(rpcUrl.href);
  try {
    const blockNumber = await provider.getBlock(blockTag, prefetchTxs);
    return blockNumber;
  } catch {
    return null;
  }
}

export async function getEvents<
  TContract extends LineaRollupV6 | L2MessageService | TokenBridge,
  TEvent extends TypedContractEvent,
>(
  contract: TContract,
  eventFilter: TypedDeferredTopicFilter<TEvent>,
  fromBlock?: BlockTag,
  toBlock?: BlockTag,
  criteria?: (events: TypedEventLog<TEvent>[]) => Promise<TypedEventLog<TEvent>[]>,
): Promise<Array<TypedEventLog<TEvent>>> {
  const events = await contract.queryFilter(
    eventFilter,
    fromBlock as string | number | undefined,
    toBlock as string | number | undefined,
  );

  if (criteria) {
    return await criteria(events);
  }

  return events;
}

export async function waitForEvents<
  TContract extends LineaRollupV6 | L2MessageService | TokenBridge,
  TEvent extends TypedContractEvent,
>(
  contract: TContract,
  eventFilter: TypedDeferredTopicFilter<TEvent>,
  pollingIntervalMs: number = 500,
  fromBlock?: BlockTag,
  toBlock?: BlockTag,
  criteria?: (events: TypedEventLog<TEvent>[]) => Promise<TypedEventLog<TEvent>[]>,
): Promise<TypedEventLog<TEvent>[]> {
  try {
    return await awaitUntil(
      async () => await getEvents(contract, eventFilter, fromBlock, toBlock, criteria),
      (a: TypedEventLog<TEvent>[]) => a.length > 0,
      pollingIntervalMs,
    );
  } catch (error) {
    if (error instanceof AwaitUntilTimeoutError) {
      logger.error(`Timeout waiting for events after ${error.timeoutMs}ms`);
      throw new Error(
        `Timeout waiting for events after ${error.timeoutMs}ms. contract=${await contract.getAddress()} eventName=${eventFilter.fragment.name} topics=${JSON.stringify(await eventFilter.getTopicFilter())} fromBlock=${fromBlock ?? 0} toBlock=${toBlock ?? "latest"}`,
      );
    }

    logger.error(`Error waiting for events. error=${error}`);
    throw error;
  }
}

export function getFiles(directory: string, fileRegex: RegExp[]): string[] {
  const files = fs.readdirSync(directory, { withFileTypes: true });
  const filteredFiles = files.filter((file) => fileRegex.map((regex) => regex.test(file.name)).includes(true));
  return filteredFiles.map((file) => fs.readFileSync(path.join(directory, file.name), "utf-8"));
}

export async function sendTransactionsToGenerateTrafficWithInterval(
  signer: AbstractSigner,
  pollingInterval: number = 1_000,
) {
  const lineaEstimateGasClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);

  const transaction: TransactionRequest = {
    to: await signer.getAddress(),
    value: etherToWei("0.000001"),
  };

  let timeoutId: NodeJS.Timeout | null = null;
  let isRunning = true;

  const sendTransaction = async () => {
    if (!isRunning) return;

    try {
      const { maxPriorityFeePerGas, maxFeePerGas } = await lineaEstimateGasClient.lineaEstimateGas(
        await signer.getAddress(),
        await signer.getAddress(),
        undefined,
        toBeHex(transaction.value!),
      );
      const tx = await signer.sendTransaction({ ...transaction, maxPriorityFeePerGas, maxFeePerGas });
      await tx.wait();
      logger.debug(
        `Transaction sent successfully. hash=${tx.hash} maxPriorityFeePerGas=${tx.maxPriorityFeePerGas} maxFeePerGas=${tx.maxFeePerGas}`,
      );
    } catch (error) {
      logger.error(`Error sending transaction. error=${JSON.stringify(error)}`);
    } finally {
      if (isRunning) {
        timeoutId = setTimeout(sendTransaction, pollingInterval);
      }
    }
  };

  const stop = () => {
    isRunning = false;
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    logger.debug("Stopped generating traffic on L2");
  };

  sendTransaction();

  return stop;
}

export function getMessageSentEventFromLogs<T extends BaseContract>(
  contract: T,
  receipts: TransactionReceipt[],
): MessageEvent[] {
  return receipts
    .flatMap((receipt) => receipt.logs)
    .filter((log) => log.topics[0] === "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c")
    .map((log) => {
      const logDescription = contract.interface.parseLog(log);
      if (!logDescription) {
        throw new Error("Invalid log description");
      }
      const { args } = logDescription;
      return {
        from: args._from,
        to: args._to,
        fee: args._fee,
        value: args._value,
        messageNumber: args._nonce,
        calldata: args._calldata,
        messageHash: args._messageHash,
        blockNumber: log.blockNumber,
      };
    });
}

export const sendMessage = async <T extends LineaRollupV6 | L2MessageService>(
  signer: AbstractSigner,
  contract: T,
  args: SendMessageArgs,
  overrides?: PayableOverrides,
): Promise<TransactionReceipt> => {
  const tx = await signer.sendTransaction({
    to: await contract.getAddress(),
    value: overrides?.value || 0n,
    data: contract.interface.encodeFunctionData("sendMessage", [args.to, args.fee, args.calldata]),
    ...overrides,
  });

  const receipt = await tx.wait();

  if (!receipt) {
    throw new Error("Transaction receipt is undefined");
  }
  return receipt;
};

export async function execDockerCommand(command: string, containerName: string): Promise<string> {
  const dockerCommand = `docker ${command} ${containerName}`;
  logger.debug(`Executing ${dockerCommand}...`);
  return new Promise((resolve, reject) => {
    exec(dockerCommand, (error, stdout, stderr) => {
      if (error) {
        logger.error(`Error executing (${dockerCommand}). error=${stderr}`);
        reject(error);
      }
      logger.debug(`Execution success (${dockerCommand}). output=${stdout}`);
      resolve(stdout);
    });
  });
}

export async function getDockerImageTag(containerName: string, imageRepoName: string): Promise<string> {
  const inspectJsonOutput = JSON.parse(await execDockerCommand("inspect", containerName));
  return inspectJsonOutput[0]["Config"]["Image"].replace(imageRepoName + ":", "");
}

export function generateRoleAssignments(
  roles: string[],
  defaultAddress: string,
  overrides: { role: string; addresses: string[] }[],
): { role: string; addressWithRole: string }[] {
  const roleAssignments: { role: string; addressWithRole: string }[] = [];

  const overridesMap = new Map<string, string[]>();
  for (const override of overrides) {
    overridesMap.set(override.role, override.addresses);
  }

  const allRolesSet = new Set<string>(roles);
  for (const override of overrides) {
    allRolesSet.add(override.role);
  }

  for (const role of allRolesSet) {
    if (overridesMap.has(role)) {
      const addresses = overridesMap.get(role);

      if (addresses && addresses.length > 0) {
        for (const addressWithRole of addresses) {
          roleAssignments.push({ role, addressWithRole });
        }
      }
    } else {
      roleAssignments.push({ role, addressWithRole: defaultAddress });
    }
  }

  return roleAssignments;
}

export function convertStringToPaddedHexBytes(strVal: string, paddedSize: number): string {
  if (strVal.length > paddedSize) {
    throw "Length is longer than padded size!";
  }

  const strBytes = ethers.toUtf8Bytes(strVal);
  const bytes = ethers.zeroPadBytes(strBytes, paddedSize);
  const bytes8Hex = ethers.hexlify(bytes);

  return bytes8Hex;
}
