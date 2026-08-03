import { getContractsAddressesByChainId } from "@lfdt-lineth/sdk-core";
import {
  Account,
  Address,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  GetContractEventsErrorType,
  GetTransactionReceiptErrorType,
  GetTransactionReceiptReturnType,
  Hex,
  Transport,
} from "viem";
import { getContractEvents, getTransactionReceipt } from "viem/actions";

import { MESSAGE_SENT_EVENT_ABI } from "../abis";
import { MessageNotFoundError, MessageNotFoundErrorType } from "../errors/bridge";

export type GetTransactionReceiptByMessageHashParameters = {
  messageHash: Hex;
  // Defaults to the message service address for the chain
  messageServiceAddress?: Address | undefined;
  // Block in which the `MessageSent` event was emitted. When provided, the lookup queries only that
  // block instead of the full `earliest`..`latest` range. This is REQUIRED when the RPC provider does
  // not support large block ranges; otherwise the default full-range query will be rejected.
  messageBlockNumber?: bigint;
};

export type GetTransactionReceiptByMessageHashReturnType<chain extends Chain | undefined> =
  GetTransactionReceiptReturnType<chain>;

export type GetTransactionReceiptByMessageHashErrorType =
  | GetContractEventsErrorType
  | GetTransactionReceiptErrorType
  | MessageNotFoundErrorType
  | ChainNotFoundErrorType;

/**
 * Returns the transaction receipt for a message sent by its message hash.
 *
 * @returns The transaction receipt of the message. {@link GetTransactionReceiptByMessageHashReturnType}
 * @param client - Client to use
 * @param args - {@link GetTransactionReceiptByMessageHashParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getTransactionReceiptByMessageHash } from '@lfdt-lineth/sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const transactionReceipt = await getTransactionReceiptByMessageHash(client, {
 *   transactionHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 *   // Required if the RPC provider does not support large block ranges.
 *   // It restricts the lookup to a single block.
 *   messageBlockNumber: 1_234_567n,
 * });
 */
export async function getTransactionReceiptByMessageHash<
  chain extends Chain | undefined,
  account extends Account | undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetTransactionReceiptByMessageHashParameters,
): Promise<GetTransactionReceiptByMessageHashReturnType<chain>> {
  const { messageHash, messageServiceAddress, messageBlockNumber } = parameters;

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  const [event] = await getContractEvents(client, {
    address: messageServiceAddress ?? getContractsAddressesByChainId(client.chain.id).messageService,
    abi: MESSAGE_SENT_EVENT_ABI,
    eventName: "MessageSent",
    args: {
      _messageHash: messageHash,
    },
    fromBlock: messageBlockNumber ?? "earliest",
    toBlock: messageBlockNumber ?? "latest",
  });

  if (!event) {
    throw new MessageNotFoundError({ hash: messageHash });
  }

  const receipt = await getTransactionReceipt(client, { hash: event.transactionHash });

  return receipt;
}
