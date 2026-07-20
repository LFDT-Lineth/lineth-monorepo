import { ExtendedMessage, getContractsAddressesByChainId } from "@lfdt-lineth/sdk-core";
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

import { MESSAGE_SENT_EVENT_ABI } from "../abis";

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
 * import { getMessagesByTransactionHash } from '@lfdt-lineth/sdk-viem'
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
    abi: MESSAGE_SENT_EVENT_ABI,
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
