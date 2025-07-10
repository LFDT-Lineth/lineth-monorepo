import { Abi, Account, Address, BlockNumber, BlockTag, Chain, Client, ContractEventName, Transport } from "viem";
import { getMessageProof, GetMessageProofParameters, GetMessageProofReturnType } from "../actions/getMessageProof";
import {
  getL2ToL1MessageStatus,
  GetL2ToL1MessageStatusParameters,
  GetL2ToL1MessageStatusReturnType,
} from "../actions/getL2ToL1MessageStatus";
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
import { L1PublicClient } from "@consensys/linea-sdk-core";
import { StrictFunctionOnly } from "../types/misc";

export type PublicActionsL1<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
> = StrictFunctionOnly<
  L1PublicClient,
  {
    /**
     * Returns the proof of a message sent from L2 to L1.
     *
     * @param client - Client to use
     * @param args - {@link GetMessageProofParameters}
     * @returns The proof of a message sent from L2 to L1. {@link GetMessageProofReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { mainnet, linea } from 'viem/chains'
     * import { publicActionsL1 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: mainnet,
     *   transport: http,
     * }).extend(publicActionsL1());
     *
     * const l2Client = createPublicClient({
     *  chain: linea,
     *  transport: http(),
     * });
     *
     * const data = await client.getMessageProof({
     *   l2Client,
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getMessageProof: <
      abi extends Abi | readonly unknown[] = Abi,
      eventName extends ContractEventName<abi> | undefined = ContractEventName<abi> | undefined,
      strict extends boolean | undefined = undefined,
      fromBlock extends BlockNumber | BlockTag | undefined = undefined,
      toBlock extends BlockNumber | BlockTag | undefined = undefined,
    >(
      args: GetMessageProofParameters<chain, account, abi, eventName, strict, fromBlock, toBlock>,
    ) => Promise<GetMessageProofReturnType>;

    /**
     * Returns the status of an L2 to L1 message on Linea.
     *
     * @param client - Client to use
     * @param args - {@link GetL2ToL1MessageStatusParameters}
     * @returns The status of a message sent from L2 to L1. {@link GetL2ToL1MessageStatusReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { mainnet, linea } from 'viem/chains'
     * import { publicActionsL1 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(publicActionsL1());
     *
     * const l2Client = createPublicClient({
     *  chain: linea,
     *  transport: http(),
     * });
     *
     * const status = await client.getL2ToL1MessageStatus({
     *   l2Client,
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getL2ToL1MessageStatus: <
      abi extends Abi | readonly unknown[] = Abi,
      eventName extends ContractEventName<abi> | undefined = ContractEventName<abi> | undefined,
      strict extends boolean | undefined = undefined,
      fromBlock extends BlockNumber | BlockTag | undefined = undefined,
      toBlock extends BlockNumber | BlockTag | undefined = undefined,
    >(
      args: GetL2ToL1MessageStatusParameters<chain, account, abi, eventName, strict, fromBlock, toBlock>,
    ) => Promise<GetL2ToL1MessageStatusReturnType>;

    /**
     * Returns the details of a message by its hash.
     *
     * @param client - Client to use
     * @param args - {@link GetMessageByMessageHashParameters}
     * @returns The details of a message. {@link GetMessageByMessageHashReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { mainnet } from 'viem/chains'
     * import { publicActionsL1 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(publicActionsL1());
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
     * @param parameters - {@link GetMessagesByTransactionHashParameters}
     * @returns The details of messages sent in the transaction.  {@link GetMessagesByTransactionHashReturnType}
     *
     * @example
     * import { createPublicClient, http } from 'viem'
     * import { mainnet } from 'viem/chains'
     * import { publicActionsL1 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(publicActionsL1());
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
     * import { mainnet } from 'viem/chains'
     * import { publicActionsL1 } from '@consensys/linea-sdk-viem'
     *
     * const client = createPublicClient({
     *   chain: mainnet,
     *   transport: http(),
     * }).extend(publicActionsL1());
     *
     * const data = await client.getTransactionReceiptByMessageHash({
     *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
     * });
     */
    getTransactionReceiptByMessageHash: <chain extends Chain | undefined>(
      args: GetTransactionReceiptByMessageHashParameters,
    ) => Promise<GetTransactionReceiptByMessageHashReturnType<chain>>;
  }
>;

export type PublicActionsL1Parameters = {
  lineaRollupAddress: Address;
  l2MessageServiceAddress: Address;
};

export function publicActionsL1(parameters?: PublicActionsL1Parameters) {
  return <
    chain extends Chain | undefined = Chain | undefined,
    account extends Account | undefined = Account | undefined,
  >(
    client: Client<Transport, chain, account>,
  ): PublicActionsL1<chain, account> => ({
    getMessageProof: (args) =>
      getMessageProof(client, {
        ...args,
        ...(parameters
          ? {
              lineaRollupAddress: parameters.lineaRollupAddress,
              l2MessageServiceAddress: parameters.l2MessageServiceAddress,
            }
          : {}),
      }),
    getL2ToL1MessageStatus: (args) =>
      getL2ToL1MessageStatus(client, {
        ...args,
        ...(parameters
          ? {
              lineaRollupAddress: parameters.lineaRollupAddress,
              l2MessageServiceAddress: parameters.l2MessageServiceAddress,
            }
          : {}),
      }),
    getMessageByMessageHash: (args) =>
      getMessageByMessageHash(client, {
        ...args,
        ...(parameters
          ? {
              messageServiceAddress: parameters.lineaRollupAddress,
            }
          : {}),
      }),
    getMessagesByTransactionHash: (args) =>
      getMessagesByTransactionHash(client, {
        ...args,
        ...(parameters
          ? {
              messageServiceAddress: parameters.lineaRollupAddress,
            }
          : {}),
      }),
    getTransactionReceiptByMessageHash: (args) =>
      getTransactionReceiptByMessageHash(client, {
        ...args,
        ...(parameters
          ? {
              messageServiceAddress: parameters.lineaRollupAddress,
            }
          : {}),
      }),
  });
}
