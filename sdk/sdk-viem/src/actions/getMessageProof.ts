import {
  Abi,
  Account,
  Address,
  BaseError,
  BlockNumber,
  BlockTag,
  Chain,
  Client,
  ContractEventName,
  encodePacked,
  GetContractEventsParameters,
  Hex,
  keccak256,
  parseEventLogs,
  Transport,
  zeroHash,
} from "viem";
import { getContractsAddressesByChainId, MessageProof, SparseMerkleTree } from "@consensys/linea-sdk-core";
import { getMessageSentEvents } from "./getMessageSentEvents";
import { getContractEvents, getTransactionReceipt } from "viem/actions";

export type GetMessageProofReturnType = MessageProof;

export type GetMessageProofParameters<
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

/**
 * Returns the proof of a message sent from L2 to L1.
 *
 * @returns The proof of a message sent from L2 to L1. {@link GetMessageProofReturnType}
 * @param client - Client to use
 * @param parameters - {@link GetMessageProofParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { mainnet, linea } from 'viem/chains'
 * import { getMessageProof } from '@consensys/linea-sdk-viem'
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
 * const messageProof = await getMessageProof(client, {
 *   l2Client,
 *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getMessageProof<
  chain extends Chain | undefined,
  account extends Account | undefined,
  chainL2 extends Chain | undefined,
  accountL2 extends Account | undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetMessageProofParameters<chainL2, accountL2>,
): Promise<GetMessageProofReturnType> {
  const { l2Client, messageHash } = parameters;

  if (!l2Client.chain) {
    throw new BaseError("L2 client is required to get message proof.");
  }

  if (!client.chain) {
    throw new BaseError("L1 client is required to get message proof.");
  }

  const l2MessageServiceAddress =
    parameters.l2MessageServiceAddress ?? getContractsAddressesByChainId(l2Client.chain.id).messageService;

  const [messageSentEvent] = await getMessageSentEvents(l2Client, {
    address: l2MessageServiceAddress,
    args: { _messageHash: messageHash },
    fromBlock: parameters.l2LogsBlockRange?.fromBlock,
    toBlock: parameters.l2LogsBlockRange?.toBlock,
  });

  if (!messageSentEvent) {
    throw new BaseError(`Message hash does not exist on L2. Message hash: ${messageHash}`);
  }

  const lineaRollupAddress =
    parameters.lineaRollupAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  const [l2MessagingBlockAnchoredEvent] = await getContractEvents(client, {
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
  });

  if (!l2MessagingBlockAnchoredEvent) {
    throw new BaseError(`L2 block number ${messageSentEvent.blockNumber} has not been finalized on L1.`);
  }

  const finalizationInfo = await getFinalizationMessagingInfo(client, {
    transactionHash: l2MessagingBlockAnchoredEvent.transactionHash,
    lineaRollupAddress,
  });

  const l2MessageHashesInBlockRange = (
    await getMessageSentEvents(l2Client, {
      address: l2MessageServiceAddress,
      fromBlock: finalizationInfo.l2MessagingBlocksRange.startingBlock,
      toBlock: finalizationInfo.l2MessagingBlocksRange.endBlock,
    })
  ).map((event) => event.messageHash);

  if (l2MessageHashesInBlockRange.length === 0) {
    throw new BaseError(`No MessageSent events found in this block range on L2.`);
  }

  const l2messages = getMessageSiblings(messageHash, l2MessageHashesInBlockRange, finalizationInfo.treeDepth);

  const tree = new SparseMerkleTree(finalizationInfo.treeDepth, (left: Hex, right: Hex) =>
    keccak256(encodePacked(["bytes32", "bytes32"], [left, right])),
  );

  for (const [index, leaf] of l2messages.entries()) {
    tree.addLeaf(index, leaf);
  }

  if (!finalizationInfo.l2MerkleRoots.includes(tree.getRoot())) {
    throw new BaseError("Merkle tree build failed.");
  }

  return tree.getProof(l2messages.indexOf(messageHash));
}

async function getFinalizationMessagingInfo<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  parameters: {
    lineaRollupAddress: Hex;
    transactionHash: Hex;
  },
) {
  const receipt = await getTransactionReceipt(client, { hash: parameters.transactionHash });

  let treeDepth = 0;
  const l2MerkleRoots: string[] = [];
  const blocksNumber: number[] = [];

  const filteredLogs = receipt.logs.filter(
    (log) => log.address.toLowerCase() === parameters.lineaRollupAddress.toLowerCase(),
  );

  const parsedLogs = parseEventLogs({
    abi: [
      {
        anonymous: false,
        inputs: [
          { indexed: true, internalType: "bytes32", name: "l2MerkleRoot", type: "bytes32" },
          { indexed: true, internalType: "uint256", name: "treeDepth", type: "uint256" },
        ],
        name: "L2MerkleRootAdded",
        type: "event",
      },
      {
        anonymous: false,
        inputs: [{ indexed: true, internalType: "uint256", name: "l2Block", type: "uint256" }],
        name: "L2MessagingBlockAnchored",
        type: "event",
      },
    ] as const,
    eventName: ["L2MerkleRootAdded", "L2MessagingBlockAnchored"],
    logs: filteredLogs,
  });

  for (const log of parsedLogs) {
    if (log.eventName === "L2MerkleRootAdded") {
      treeDepth = parseInt(log.args.treeDepth.toString());
      l2MerkleRoots.push(log.args.l2MerkleRoot);
    } else if (log.eventName === "L2MessagingBlockAnchored") {
      blocksNumber.push(parseInt(log.args.l2Block.toString()));
    }
  }

  if (l2MerkleRoots.length === 0) {
    throw new BaseError(`No L2MerkleRootAdded events found in this transaction.`);
  }

  if (blocksNumber.length === 0) {
    throw new BaseError(`No L2MessagingBlocksAnchored events found in this transaction.`);
  }

  return {
    l2MessagingBlocksRange: {
      startingBlock: BigInt(Math.min(...blocksNumber)),
      endBlock: BigInt(Math.max(...blocksNumber)),
    },
    l2MerkleRoots,
    treeDepth,
  };
}

function getMessageSiblings(messageHash: Hex, messageHashes: Hex[], treeDepth: number): Hex[] {
  const numberOfMessagesInTrees = 2 ** treeDepth;
  const messageHashesLength = messageHashes.length;

  const messageHashIndex = messageHashes.indexOf(messageHash);

  if (messageHashIndex === -1) {
    throw new BaseError("Message hash not found in messages.");
  }

  const start = Math.floor(messageHashIndex / numberOfMessagesInTrees) * numberOfMessagesInTrees;
  const end = Math.min(messageHashesLength, start + numberOfMessagesInTrees);

  const siblings = messageHashes.slice(start, end);

  const remainder = siblings.length % numberOfMessagesInTrees;
  if (remainder !== 0) {
    siblings.push(...Array(numberOfMessagesInTrees - remainder).fill(zeroHash));
  }

  return siblings;
}
