import {
  Account,
  Address,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  GetTransactionReceiptErrorType,
  Hex,
  parseEventLogs,
  ParseEventLogsErrorType,
  toEventSelector,
  Transport,
} from "viem";
import { getTransactionReceipt } from "viem/actions";
import { ExtendedMessage, getContractsAddressesByChainId } from "@consensys/linea-sdk-core";

export type GetMessagesByTransactionHashParameters = {
  transactionHash: Hex;
  // Defaults to the message service address for the chain
  messageServiceAddress?: Address;
};

export type GetMessagesByTransactionHashReturnType = ExtendedMessage[];

export type GetMessagesByTransactionHashErrorType =
  | GetTransactionReceiptErrorType
  | ParseEventLogsErrorType
  | ChainNotFoundErrorType;

/**
 * Returns the details of messages sent in a transaction by its hash.
 *
 * @returns The details of messages sent in the transaction.  {@link GetMessagesByTransactionHashReturnType}
 * @param client - Client to use
 * @param args - {@link GetMessagesByTransactionHashParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getMessagesByTransactionHash } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const messages = await getMessagesByTransactionHash(client, {
 *   transactionHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getMessagesByTransactionHash<
  chain extends Chain | undefined,
  account extends Account | undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetMessagesByTransactionHashParameters,
): Promise<GetMessagesByTransactionHashReturnType> {
  const { transactionHash } = parameters;

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  const receipt = await getTransactionReceipt(client, { hash: transactionHash });

  const messageServiceAddress = parameters.messageServiceAddress
    ? parameters.messageServiceAddress.toLowerCase()
    : getContractsAddressesByChainId(client.chain.id).messageService.toLowerCase();

  const logs = receipt.logs.filter(
    (log) =>
      log.address.toLowerCase() === messageServiceAddress &&
      log.topics[0]?.toLowerCase() ===
        toEventSelector("MessageSent(address,address,uint256,uint256,uint256,bytes,bytes32)").toLowerCase(),
  );

  const parsedLogs = parseEventLogs({
    abi: [
      {
        anonymous: false,
        inputs: [
          { indexed: true, internalType: "address", name: "_from", type: "address" },
          { indexed: true, internalType: "address", name: "_to", type: "address" },
          { indexed: false, internalType: "uint256", name: "_fee", type: "uint256" },
          { indexed: false, internalType: "uint256", name: "_value", type: "uint256" },
          { indexed: false, internalType: "uint256", name: "_nonce", type: "uint256" },
          { indexed: false, internalType: "bytes", name: "_calldata", type: "bytes" },
          { indexed: true, internalType: "bytes32", name: "_messageHash", type: "bytes32" },
        ],
        name: "MessageSent",
        type: "event",
      },
    ] as const,
    eventName: "MessageSent",
    logs: logs,
  });

  return parsedLogs.map((log) => ({
    from: log.args._from!,
    to: log.args._to!,
    fee: log.args._fee!,
    value: log.args._value!,
    nonce: log.args._nonce!,
    calldata: log.args._calldata!,
    messageHash: log.args._messageHash!,
    transactionHash: log.transactionHash,
    blockNumber: log.blockNumber,
  }));
}
