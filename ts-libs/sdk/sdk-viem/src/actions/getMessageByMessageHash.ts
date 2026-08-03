import { getContractsAddressesByChainId } from "@lfdt-lineth/sdk-core";
import {
  Account,
  Address,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  GetContractEventsErrorType,
  Hex,
  Transport,
} from "viem";
import { getContractEvents } from "viem/actions";

import { MESSAGE_SENT_EVENT_ABI } from "../abis";
import { MessageNotFoundError, MessageNotFoundErrorType } from "../errors/bridge";

export type GetMessageByMessageHashParameters = {
  messageHash: Hex;
  // Defaults to the message service address for the chain
  messageServiceAddress?: Address | undefined;
  // Block in which the `MessageSent` event was emitted. When provided, the lookup queries only that
  // block instead of the full `earliest`..`latest` range. This is REQUIRED when the RPC provider does
  // not support large block ranges; otherwise the default full-range query will be rejected.
  messageBlockNumber?: bigint;
};

export type GetMessageByMessageHashReturnType = {
  from: Hex;
  to: Hex;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: Hex;
  messageHash: Hex;
  transactionHash: Hex;
  blockNumber: bigint;
  logIndex: number;
};

export type GetMessageByMessageHashErrorType =
  | GetContractEventsErrorType
  | MessageNotFoundErrorType
  | ChainNotFoundErrorType;

/**
 * Returns the details of a message by its hash.
 *
 * @returns The details of a message. {@link GetMessageByMessageHashReturnType}
 * @param client - Client to use
 * @param args - {@link GetMessageByMessageHashParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getMessageByMessageHash } from '@lfdt-lineth/sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const message = await getMessageByMessageHash(client, {
 *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 *   // Required if the RPC provider does not support large block ranges.
 *   // It restricts the lookup to a single block.
 *   messageBlockNumber: 1_234_567n,
 * });
 */
export async function getMessageByMessageHash<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  parameters: GetMessageByMessageHashParameters,
): Promise<GetMessageByMessageHashReturnType> {
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

  return {
    from: event.args._from!,
    to: event.args._to!,
    fee: event.args._fee!,
    value: event.args._value!,
    nonce: event.args._nonce!,
    calldata: event.args._calldata!,
    messageHash: event.args._messageHash!,
    transactionHash: event.transactionHash,
    blockNumber: event.blockNumber,
    logIndex: event.logIndex,
  };
}
