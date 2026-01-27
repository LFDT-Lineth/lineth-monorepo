import { L2PublicClient } from "@consensys/linea-sdk-core";
import { Account, Address, BlockTag, Chain, Client, Transport } from "viem";

import {
  getBlockExtraData,
  GetBlockExtraDataParameters,
  GetBlockExtraDataReturnType,
} from "../actions/getBlockExtraData";
import {
  getL1ToL2MessageStatus,
  GetL1ToL2MessageStatusParameters,
  GetL1ToL2MessageStatusReturnType,
} from "../actions/getL1ToL2MessageStatus";
import {
  getMessageByMessageHash,
  GetMessageByMessageHashParameters,
  GetMessageByMessageHashReturnType,
} from "../actions/getMessageByMessageHash";
import {
  getMessagesByTransactionHash,
  GetMessagesByTransactionHashParameters,
  GetMessagesByTransactionHashReturnType,
} from "../actions/getMessagesByTransactionHash";
import {
  getTransactionReceiptByMessageHash,
  GetTransactionReceiptByMessageHashParameters,
  GetTransactionReceiptByMessageHashReturnType,
} from "../actions/getTransactionReceiptByMessageHash";
import { StrictFunctionOnly } from "../types/misc";

export type PublicActionsL2<chain extends Chain | undefined = Chain | undefined> = StrictFunctionOnly<
  L2PublicClient,
  {
    /**
     * Returns fomatted linea block extra data.
     *
     * @param client - Client to use
     * @param args - {@link GetBlockExtraDataParameters}
     * @returns Formatted linea block extra data. {@link GetBlockExtraDataReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { linea } from 'viem/chains'
     * import { publicActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(publicActionsL2());
     *
     * const data = await client.getBlockExtraData({
     *   blockTag: 'latest',
     * });
     */
    getBlockExtraData: <blockTag extends BlockTag = "latest">(
      args: GetBlockExtraDataParameters<blockTag>,
    ) => Promise<GetBlockExtraDataReturnType>;

    /**
     * Returns the status of an L1 to L2 message on Linea.
     *
     * @param client - Client to use
     * @param args - {@link GetL1ToL2MessageStatusParameters}
     * @returns The status of the L1 to L2 message. {@link GetL1ToL2MessageStatusReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { linea } from 'viem/chains'
     * import { publicActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(publicActionsL2());
     *
     * const data = await client.getL1ToL2MessageStatus({
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getL1ToL2MessageStatus: (args: GetL1ToL2MessageStatusParameters) => Promise<GetL1ToL2MessageStatusReturnType>;

    /**
     * Returns the details of a message by its hash.
     *
     * @param client - Client to use
     * @param args - {@link GetMessageByMessageHashParameters}
     * @returns The details of a message. {@link GetMessageByMessageHashReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { linea } from 'viem/chains'
     * import { publicActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(publicActionsL2());
     *
     * const data = await client.getMessageByMessageHash({
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getMessageByMessageHash: (args: GetMessageByMessageHashParameters) => Promise<GetMessageByMessageHashReturnType>;

    /**
     * Returns the details of messages sent in a transaction by its hash.
     *
     * @param client - Client to use
     * @param args - {@link GetMessagesByTransactionHashParameters}
     * @returns The details of messages sent in the transaction.  {@link GetMessagesByTransactionHashReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { linea } from 'viem/chains'
     * import { publicActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(publicActionsL2());
     *
     * const data = await client.getMessagesByTransactionHash({
     *   transactionHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getMessagesByTransactionHash: (
      args: GetMessagesByTransactionHashParameters,
    ) => Promise<GetMessagesByTransactionHashReturnType>;

    /**
     * Returns the transaction receipt for a message sent by its message hash.
     *
     * @param client - Client to use
     * @param args - {@link GetTransactionReceiptByMessageHashParameters}
     * @returns The transaction receipt of the message. {@link GetTransactionReceiptByMessageHashReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { linea } from 'viem/chains'
     * import { publicActionsL2 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: linea,
     *   transport: http(),
     * }).extend(publicActionsL2());
     *
     * const data = await client.getTransactionReceiptByMessageHash({
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getTransactionReceiptByMessageHash: (
      args: GetTransactionReceiptByMessageHashParameters,
    ) => Promise<GetTransactionReceiptByMessageHashReturnType<chain>>;
  }
>;

export type PublicActionsL2Parameters = {
  l2MessageServiceAddress: Address;
};

export function publicActionsL2(parameters?: PublicActionsL2Parameters) {
  return <
    transport extends Transport = Transport,
    chain extends Chain | undefined = Chain | undefined,
    account extends Account | undefined = Account | undefined,
  >(
    client: Client<transport, chain, account>,
  ): PublicActionsL2<chain> => ({
    getBlockExtraData: (args) => getBlockExtraData(client, args),
    getL1ToL2MessageStatus: (args) =>
      getL1ToL2MessageStatus(client, {
        ...args,
        ...(parameters ? { l2MessageServiceAddress: parameters.l2MessageServiceAddress } : {}),
      }),
    getMessageByMessageHash: (args) =>
      getMessageByMessageHash(client, {
        ...args,
        ...(parameters ? { messageServiceAddress: parameters.l2MessageServiceAddress } : {}),
      }),
    getMessagesByTransactionHash: (args) =>
      getMessagesByTransactionHash(client, {
        ...args,
        ...(parameters ? { messageServiceAddress: parameters.l2MessageServiceAddress } : {}),
      }),
    getTransactionReceiptByMessageHash: (args) =>
      getTransactionReceiptByMessageHash(client, {
        ...args,
        ...(parameters ? { messageServiceAddress: parameters.l2MessageServiceAddress } : {}),
      }),
  });
}
