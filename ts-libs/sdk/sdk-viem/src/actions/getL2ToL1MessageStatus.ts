import { getContractsAddressesByChainId, OnChainMessageStatus } from "@consensys/linea-sdk-core";
import {
  Abi,
  Account,
  Address,
  BlockNumber,
  BlockTag,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  ClientChainNotConfiguredError,
  ClientChainNotConfiguredErrorType,
  ContractEventName,
  GetContractEventsErrorType,
  GetContractEventsParameters,
  Hex,
  ReadContractErrorType,
  Transport,
} from "viem";
import { getContractEvents, readContract } from "viem/actions";

import { getMessageSentEvents, GetMessageSentEventsErrorType } from "./getMessageSentEvents";
import { MessageNotFoundError, MessageNotFoundErrorType } from "../errors/bridge";

export type GetL2ToL1MessageStatusParameters<
  chain extends Chain | undefined,
  account extends Account | undefined,
  abi extends Abi | readonly unknown[] = Abi,
  eventName extends ContractEventName<abi> | undefined = ContractEventName<abi> | undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
> = {
  l2Client: Client<Transport, chain, account>;
  messageHash: Hex;
  l2LogsBlockRange?: Pick<
    GetContractEventsParameters<abi, eventName, strict, fromBlock, toBlock>,
    "fromBlock" | "toBlock"
  >;
  // Defaults to the message service address for the L1 chain
  lineaRollupAddress?: Address;
  // Defaults to the message service address for the L2 chain
  l2MessageServiceAddress?: Address;
};

export type GetL2ToL1MessageStatusReturnType = OnChainMessageStatus;

export type GetL2ToL1MessageStatusErrorType =
  | GetMessageSentEventsErrorType
  | GetContractEventsErrorType
  | ReadContractErrorType
  | MessageNotFoundErrorType
  | ChainNotFoundErrorType
  | ClientChainNotConfiguredErrorType;

/**
 * Returns the status of an L2 to L1 message on Linea.
 *
 * @returns The status of the L2 to L1 message.  {@link GetL2ToL1MessageStatusReturnType}
 * @param client - Client to use
 * @param args - {@link GetL2ToL1MessageStatusParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { mainnet, linea } from 'viem/chains'
 * import { getL2ToL1MessageStatus } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: mainnet,
 *   transport: http(),
 * });
 *
 * const l2Client = createPublicClient({
 *  chain: linea,
 *  transport: http(),
 * });
 *
 * const messageStatus = await getL2ToL1MessageStatus(client, {
 *   l2Client,
 *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getL2ToL1MessageStatus<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainL2 extends Chain | undefined,
  accountL2 extends Account | undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetL2ToL1MessageStatusParameters<chainL2, accountL2>,
): Promise<GetL2ToL1MessageStatusReturnType> {
  const { l2Client, messageHash, l2LogsBlockRange } = parameters;

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  if (!l2Client.chain) {
    throw new ClientChainNotConfiguredError();
  }

  const l2MessageServiceAddress =
    parameters.l2MessageServiceAddress ?? getContractsAddressesByChainId(l2Client.chain.id).messageService;

  const [messageSentEvent] = await getMessageSentEvents(l2Client, {
    args: { _messageHash: messageHash },
    address: l2MessageServiceAddress,
    fromBlock: l2LogsBlockRange?.fromBlock,
    toBlock: l2LogsBlockRange?.toBlock,
  });

  if (!messageSentEvent) {
    throw new MessageNotFoundError({ hash: messageHash });
  }

  const lineaRollupAddress =
    parameters.lineaRollupAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  const [[l2MessagingBlockAnchoredEvent], isMessageClaimed] = await Promise.all([
    getContractEvents(client, {
      address: lineaRollupAddress,
      abi: [
        {
          anonymous: false,
          inputs: [{ indexed: true, internalType: "uint256", name: "l2Block", type: "uint256" }],
          name: "L2MessagingBlockAnchored",
          type: "event",
        },
      ] as const,
      eventName: "L2MessagingBlockAnchored",
      args: {
        l2Block: messageSentEvent.blockNumber,
      },
      fromBlock: "earliest",
      toBlock: "latest",
    }),
    readContract(client, {
      address: lineaRollupAddress,
      abi: [
        {
          inputs: [{ internalType: "uint256", name: "_messageNumber", type: "uint256" }],
          name: "isMessageClaimed",
          outputs: [{ internalType: "bool", name: "isClaimed", type: "bool" }],
          stateMutability: "view",
          type: "function",
        },
      ],
      functionName: "isMessageClaimed",
      args: [messageSentEvent.messageNonce],
    }),
  ]);

  if (isMessageClaimed) {
    return OnChainMessageStatus.CLAIMED;
  }

  if (l2MessagingBlockAnchoredEvent) {
    return OnChainMessageStatus.CLAIMABLE;
  }

  return OnChainMessageStatus.UNKNOWN;
}
