import * as fs from "fs";
import assert from "assert";
import { AbstractSigner, BaseContract, BlockTag, TransactionReceipt, TransactionRequest, Wallet, ethers } from "ethers";
import path from "path";
import { exec } from "child_process";
import { L2MessageServiceV1 as L2MessageService, TokenBridgeV1 as TokenBridge, LineaRollupV6 } from "../typechain";
import {
  PayableOverrides,
  TypedContractEvent,
  TypedDeferredTopicFilter,
  TypedEventLog,
  TypedContractMethod,
} from "../typechain/common";
import { MessageEvent, SendMessageArgs } from "./types";
import { createTestLogger } from "../config/logger";

const logger = createTestLogger();

export function etherToWei(amount: string): bigint {
  return ethers.parseEther(amount.toString());
}

export function readJsonFile(filePath: string): unknown {
  const data = fs.readFileSync(filePath, "utf8");
  return JSON.parse(data);
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

export class RollupGetZkEVMBlockNumberClient {
  private endpoint: URL;
  private request = {
    method: "post",
    body: JSON.stringify({
      jsonrpc: "2.0",
      method: "rollup_getZkEVMBlockNumber",
      params: [],
      id: 1,
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

export class LineaEstimateGasClient {
  private endpoint: URL;

  public constructor(endpoint: URL) {
    this.endpoint = endpoint;
  }

  public async lineaEstimateGas(
    from: string,
    to: string,
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
        id: 1,
      }),
    };
    const response = await fetch(this.endpoint, request);
    const responseJson = await response.json();
    assert("result" in responseJson);
    return {
      maxFeePerGas: BigInt(responseJson.result.baseFeePerGas) + BigInt(responseJson.result.priorityFeePerGas),
      maxPriorityFeePerGas: BigInt(responseJson.result.priorityFeePerGas),
      gasLimit: BigInt(responseJson.result.gasLimit),
    };
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
        id: 1,
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
        id: 1,
      }),
    };
    const response = await fetch(this.endpoint, request);
    return await response.json();
  }
}

export async function getTransactionHash(txRequest: TransactionRequest, signer: Wallet): Promise<string> {
  const rawTransaction = await signer.populateTransaction(txRequest);
  const signature = await signer.signTransaction(rawTransaction);
  return ethers.keccak256(signature);
}

export async function getBlockByNumberOrBlockTag(rpcUrl: URL, blockTag: BlockTag): Promise<ethers.Block | null> {
  const provider = new ethers.JsonRpcProvider(rpcUrl.href);
  try {
    const blockNumber = await provider.getBlock(blockTag);
    return blockNumber;
  } catch (error) {
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
  pollingInterval: number = 500,
  fromBlock?: BlockTag,
  toBlock?: BlockTag,
  criteria?: (events: TypedEventLog<TEvent>[]) => Promise<TypedEventLog<TEvent>[]>,
): Promise<TypedEventLog<TEvent>[]> {
  let events = await getEvents(contract, eventFilter, fromBlock, toBlock, criteria);

  while (events.length === 0) {
    events = await getEvents(contract, eventFilter, fromBlock, toBlock, criteria);
    await wait(pollingInterval);
  }

  return events;
}

// Currently only handle simple single return types - uint256 | bytesX | string | bool
export async function pollForContractMethodReturnValue<
  ExpectedReturnType extends bigint | string | boolean,
  R extends [ExpectedReturnType],
>(
  method: TypedContractMethod<[], R, "view">,
  expectedReturnValue: ExpectedReturnType,
  compareFunction: (a: ExpectedReturnType, b: ExpectedReturnType) => boolean = (a, b) => a === b,
  pollingInterval: number = 500,
  timeout: number = 2 * 60 * 1000,
): Promise<boolean> {
  let isExceedTimeOut = false;
  setTimeout(() => {
    isExceedTimeOut = true;
  }, timeout);

  while (!isExceedTimeOut) {
    const returnValue = await method();
    if (compareFunction(returnValue, expectedReturnValue)) return true;
    await wait(pollingInterval);
  }

  return false;
}

// Currently only handle single uint256 return type
export async function pollForContractMethodReturnValueExceedTarget<
  ExpectedReturnType extends bigint,
  R extends [ExpectedReturnType],
>(
  method: TypedContractMethod<[], R, "view">,
  targetReturnValue: ExpectedReturnType,
  pollingInterval: number = 500,
  timeout: number = 2 * 60 * 1000,
): Promise<boolean> {
  return pollForContractMethodReturnValue(method, targetReturnValue, (a, b) => a >= b, pollingInterval, timeout);
}

export function getFiles(directory: string, fileRegex: RegExp[]): string[] {
  const files = fs.readdirSync(directory, { withFileTypes: true });
  const filteredFiles = files.filter((file) => fileRegex.map((regex) => regex.test(file.name)).includes(true));
  return filteredFiles.map((file) => fs.readFileSync(path.join(directory, file.name), "utf-8"));
}

export async function waitForFile(
  directory: string,
  regex: RegExp,
  pollingInterval: number,
  timeout: number,
  criteria?: (fileName: string) => boolean,
): Promise<string> {
  const endTime = Date.now() + timeout;

  while (Date.now() < endTime) {
    try {
      const files = fs.readdirSync(directory);

      for (const file of files) {
        if (regex.test(file) && (!criteria || criteria(file))) {
          const filePath = path.join(directory, file);
          const content = fs.readFileSync(filePath, "utf-8");
          return content;
        }
      }
    } catch (err) {
      throw new Error(`Error reading directory: ${(err as Error).message}`);
    }

    await new Promise((resolve) => setTimeout(resolve, pollingInterval));
  }

  throw new Error("File check timed out");
}

export async function sendTransactionsToGenerateTrafficWithInterval(
  signer: AbstractSigner,
  pollingInterval: number = 1_000,
) {
  const { maxPriorityFeePerGas, maxFeePerGas } = await signer.provider!.getFeeData();
  const transactionRequest = {
    to: await signer.getAddress(),
    value: etherToWei("0.000001"),
    maxPriorityFeePerGas: maxPriorityFeePerGas,
    maxFeePerGas: maxFeePerGas,
  };

  let timeoutId: NodeJS.Timeout | null = null;
  let isRunning = true;

  const sendTransaction = async () => {
    if (!isRunning) return;

    try {
      const tx = await signer.sendTransaction(transactionRequest);
      await tx.wait();
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
    logger.info("Stopped generating traffic on L2");
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
  logger.info(`Executing ${dockerCommand}...`);
  return new Promise((resolve, reject) => {
    exec(dockerCommand, (error, stdout, stderr) => {
      if (error) {
        logger.error(`Error executing (${dockerCommand}). error=${stderr}`);
        reject(error);
      }
      logger.info(`Execution success (${dockerCommand}). output=${stdout}`);
      resolve(stdout);
    });
  });
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
