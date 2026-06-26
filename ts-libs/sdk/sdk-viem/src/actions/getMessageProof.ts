import { getContractsAddressesByChainId, MessageProof, SparseMerkleTree } from "@lfdt-lineth/sdk-core";
import {
  Abi,
  AbiDecodingZeroDataError,
  Account,
  Address,
  BaseError,
  BlockNumber,
  BlockTag,
  Chain,
  ChainNotFoundError,
  ChainNotFoundErrorType,
  Client,
  ClientChainNotConfiguredError,
  ClientChainNotConfiguredErrorType,
  ContractEventName,
  ContractFunctionZeroDataError,
  encodePacked,
  GetContractEventsErrorType,
  GetContractEventsParameters,
  GetTransactionReceiptErrorType,
  Hex,
  keccak256,
  parseEventLogs,
  ParseEventLogsErrorType,
  Transport,
  zeroHash,
} from "viem";
import { getBlockNumber, getContractEvents, getTransactionReceipt, readContract } from "viem/actions";

import {
  getMessageSentEvents,
  GetMessageSentEventsErrorType,
  GetMessageSentEventsReturnType,
} from "./getMessageSentEvents";
import {
  CURRENT_L2_BLOCK_NUMBER_ABI,
  L2_MERKLE_ROOT_ADDED_EVENT_ABI,
  L2_MESSAGING_BLOCK_ANCHORED_EVENT_ABI,
} from "../abis";
import {
  EventNotFoundInFinalizationDataError,
  EventNotFoundInFinalizationDataErrorType,
  L2BlockNotFinalizedError,
  L2BlockNotFinalizedErrorType,
  MerkleRootNotFoundInFinalizationDataError,
  MerkleRootNotFoundInFinalizationDataErrorType,
  MessageNotFoundError,
  MessageNotFoundErrorType,
  MessagesNotFoundInBlockRangeError,
  MessagesNotFoundInBlockRangeErrorType,
} from "../errors/bridge";

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
  lineaRollupAddress?: Address | undefined;
  // Defaults to the message service address for the L2 chain
  l2MessageServiceAddress?: Address | undefined;
};

export type GetMessageProofReturnType = MessageProof;

export type GetMessageProofErrorType =
  | GetMessageSentEventsErrorType
  | GetContractEventsErrorType
  | ParseEventLogsErrorType
  | GetTransactionReceiptErrorType
  | L2BlockNotFinalizedErrorType
  | MessagesNotFoundInBlockRangeErrorType
  | MerkleRootNotFoundInFinalizationDataErrorType
  | EventNotFoundInFinalizationDataErrorType
  | MessageNotFoundErrorType
  | ChainNotFoundErrorType
  | ClientChainNotConfiguredErrorType;

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
 * import { getMessageProof } from '@lfdt-lineth/sdk-viem'
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

  if (!client.chain) {
    throw new ChainNotFoundError();
  }

  if (!l2Client.chain) {
    throw new ClientChainNotConfiguredError();
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
    throw new MessageNotFoundError({ hash: messageHash });
  }

  const lineaRollupAddress =
    parameters.lineaRollupAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  const l2MessagingBlockAnchoredEvent = await findL2MessagingBlockAnchoredEvent(client, {
    lineaRollupAddress,
    l2BlockNumber: messageSentEvent.blockNumber!,
  });

  if (!l2MessagingBlockAnchoredEvent) {
    throw new L2BlockNotFinalizedError({ blockNumber: messageSentEvent.blockNumber! });
  }

  const finalizationInfo = await getFinalizationMessagingInfo(client, {
    transactionHash: l2MessagingBlockAnchoredEvent.transactionHash,
    lineaRollupAddress,
  });

  const l2MessageHashesInBlockRange = (
    await getMessageSentEventsInChunks(l2Client, {
      address: l2MessageServiceAddress,
      fromBlock: finalizationInfo.l2MessagingBlocksRange.startingBlock,
      toBlock: finalizationInfo.l2MessagingBlocksRange.endBlock,
    })
  ).map((event) => event.messageHash);

  if (l2MessageHashesInBlockRange.length === 0) {
    throw new MessagesNotFoundInBlockRangeError({
      startBlock: finalizationInfo.l2MessagingBlocksRange.startingBlock,
      endBlock: finalizationInfo.l2MessagingBlocksRange.endBlock,
    });
  }

  const l2messages = getMessageSiblings(messageHash, l2MessageHashesInBlockRange, finalizationInfo.treeDepth);

  const tree = new SparseMerkleTree(finalizationInfo.treeDepth, (left: Hex, right: Hex) =>
    keccak256(encodePacked(["bytes32", "bytes32"], [left, right])),
  );

  for (const [index, leaf] of l2messages.entries()) {
    tree.addLeaf(index, leaf);
  }

  if (!finalizationInfo.l2MerkleRoots.includes(tree.getRoot())) {
    throw new MerkleRootNotFoundInFinalizationDataError({
      merkleRoot: tree.getRoot(),
      startBlock: finalizationInfo.l2MessagingBlocksRange.startingBlock,
      endBlock: finalizationInfo.l2MessagingBlocksRange.endBlock,
    });
  }

  return tree.getProof(l2messages.indexOf(messageHash));
}

// Conservative upper bound on the `eth_getLogs` block span accepted by rate-limited providers
// (e.g. Infura rejects spans > 10,000 blocks). The fallback narrows the finalization to a window
// no wider than this so it can be swept in a single allowed query.
const MAX_GET_LOGS_BLOCK_RANGE = 10_000n;

/**
 * Detects the specific viem error raised when a contract `eth_call` returns no data (`0x`),
 * which is what happens when the address has no code at the queried block (i.e. the contract
 * was not yet deployed at that height). Deliberately narrow so that transient network/RPC
 * failures and non-archive state errors are NOT mistaken for "pre-deployment".
 *
 * @param {unknown} error - The error thrown by a `readContract` call.
 * @returns {boolean} `true` if the call returned empty data because no code exists at that block.
 */
function isNoContractDataError(error: unknown): boolean {
  return (
    error instanceof BaseError &&
    error.walk((e) => e instanceof ContractFunctionZeroDataError || e instanceof AbiDecodingZeroDataError) !== null
  );
}

/**
 * Locates a `<= MAX_GET_LOGS_BLOCK_RANGE`-wide settlement-chain block window guaranteed to contain
 * the finalization in which `l2BlockNumber` was anchored, via a binary search over the
 * monotonically-increasing `currentL2BlockNumber()` view.
 *
 * `currentL2BlockNumber()` holds the *endBlock of the latest finalized range*, not every L2 block
 * number, so its value jumps (e.g. 100 -> 250 -> 400). The search therefore targets the first L1
 * block where `currentL2BlockNumber() >= l2BlockNumber`, i.e. the finalization range that *covers*
 * the target block (the target may sit in the middle of a range, with no exact-equal value). That
 * is exactly the finalization that anchors the target block's `L2MessagingBlockAnchored` event.
 *
 * `f(0)` reads as `0` (pre-deployment: no code => `0x`) and `f(head) >= target` (checked up front),
 * so the boundary lies in `(0, head]` and no deployment block need be supplied. The bisection stops
 * once the bracket is `<= MAX_GET_LOGS_BLOCK_RANGE` wide rather than pinpointing the exact block:
 * the residual window can then be swept by a single allowed `getLogs` call, saving the deepest
 * `~log2(MAX_GET_LOGS_BLOCK_RANGE)` probes. Costs `~log2(head / MAX_GET_LOGS_BLOCK_RANGE)`
 * `eth_call`s and requires an archive-capable endpoint for the historical reads.
 *
 * @param {Client} client - The settlement-chain (L1 or, for Validium, Linea) client.
 * @param {Object} args - The lookup arguments.
 * @param {Address} args.lineaRollupAddress - The LineaRollup/Validium contract address.
 * @param {bigint} args.l2BlockNumber - The L2 block number to locate the finalization for.
 * @returns {Promise<{ fromBlock: bigint; toBlock: bigint } | null>} The block window containing the
 * finalization, or `null` if the L2 block is not finalized yet.
 */
async function findL1FinalizationRange<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  args: { lineaRollupAddress: Address; l2BlockNumber: bigint },
): Promise<{ fromBlock: bigint; toBlock: bigint } | null> {
  const finalizedL2BlockAt = async (blockNumber: bigint): Promise<bigint> => {
    try {
      return await readContract(client, {
        address: args.lineaRollupAddress,
        abi: CURRENT_L2_BLOCK_NUMBER_ABI,
        functionName: "currentL2BlockNumber",
        blockNumber,
      });
    } catch (error) {
      // Only treat "no contract code at this height" (i.e. the call returned `0x`, which is what
      // happens before the contract is deployed) as "nothing finalized". Transient network/RPC
      // failures and non-archive state errors MUST propagate: swallowing them would let a failed
      // probe read as `0`, silently steering the search to a wrong finalization block (or a
      // spurious "not finalized") instead of failing loudly.
      if (isNoContractDataError(error)) {
        return 0n;
      }
      throw error;
    }
  };

  const head = await getBlockNumber(client);
  if ((await finalizedL2BlockAt(head)) < args.l2BlockNumber) {
    return null;
  }

  // Binary search (0, head] for the first block whose finalized state covers the target L2 block,
  // keeping the invariant finalizedAt(lo) < target <= finalizedAt(hi). Stop once the bracket fits a
  // single getLogs window: the boundary block then lies in (lo, hi].
  let lo = 0n;
  let hi = head;
  while (hi - lo > MAX_GET_LOGS_BLOCK_RANGE) {
    const mid = lo + (hi - lo) / 2n;
    if ((await finalizedL2BlockAt(mid)) >= args.l2BlockNumber) {
      hi = mid;
    } else {
      lo = mid;
    }
  }

  // The boundary is in (lo, hi]; `fromBlock = lo + 1` keeps the span at `hi - lo` (<= the limit).
  return { fromBlock: lo + 1n, toBlock: hi };
}

/**
 * Resolves the `L2MessagingBlockAnchored` event for `l2BlockNumber` on the settlement chain.
 *
 * Fast path: a single full-range (`earliest`..`latest`) query, ideal for providers that allow
 * large block ranges. The full-range attempt is treated purely as an optimization: on *any*
 * failure (a provider's `eth_getLogs` range-cap rejection — whose code/message varies across
 * providers — or otherwise) it falls back to narrowing the finalization to a
 * `<= MAX_GET_LOGS_BLOCK_RANGE` window via {@link findL1FinalizationRange} and querying that single
 * window. The fallback is itself a correct lookup, so if the block is genuinely unreachable it
 * surfaces the real error rather than relying on fragile error classification.
 *
 * @param {Client} client - The settlement-chain client.
 * @param {Object} args - The lookup arguments.
 * @param {Address} args.lineaRollupAddress - The LineaRollup/Validium contract address.
 * @param {bigint} args.l2BlockNumber - The L2 block number whose anchoring event is sought.
 * @returns The anchored event, or `undefined` if the L2 block is not finalized yet.
 */
async function findL2MessagingBlockAnchoredEvent<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  args: { lineaRollupAddress: Address; l2BlockNumber: bigint },
) {
  try {
    const [event] = await getContractEvents(client, {
      address: args.lineaRollupAddress,
      abi: L2_MESSAGING_BLOCK_ANCHORED_EVENT_ABI,
      eventName: "L2MessagingBlockAnchored",
      args: { l2Block: args.l2BlockNumber },
      fromBlock: "earliest",
      toBlock: "latest",
    });
    return event;
  } catch {
    // The full-range query is an optimization for providers that allow large `eth_getLogs` spans.
    // Any failure falls back to the bounded path below; provider range-cap rejections differ too
    // much by code/message to classify reliably, and a genuinely fatal error will resurface there.
  }

  const finalizationRange = await findL1FinalizationRange(client, args);
  if (finalizationRange === null) {
    return undefined;
  }

  const [event] = await getContractEvents(client, {
    address: args.lineaRollupAddress,
    abi: L2_MESSAGING_BLOCK_ANCHORED_EVENT_ABI,
    eventName: "L2MessagingBlockAnchored",
    args: { l2Block: args.l2BlockNumber },
    fromBlock: finalizationRange.fromBlock,
    toBlock: finalizationRange.toBlock,
  });
  return event;
}

/**
 * Fetches `MessageSent` events over `[fromBlock, toBlock]`, splitting the span into
 * `<= MAX_GET_LOGS_BLOCK_RANGE` windows so it never trips a rate-limited provider's `eth_getLogs`
 * range cap. A finalization's L2 messaging range can exceed that cap, and the results feed an
 * order-sensitive Merkle tree, so windows are queried in ascending order and concatenated to
 * preserve the on-chain (block, logIndex) ordering. Disjoint, contiguous windows mean no duplicates.
 *
 * @param {Client} client - The L2 client to query `MessageSent` events on.
 * @param {Object} args - The query arguments.
 * @param {Address} args.address - The L2 message service address.
 * @param {bigint} args.fromBlock - The inclusive start of the L2 block range.
 * @param {bigint} args.toBlock - The inclusive end of the L2 block range.
 * @returns {Promise<GetMessageSentEventsReturnType>} The ordered `MessageSent` events in the range.
 */
async function getMessageSentEventsInChunks<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  args: { address: Address; fromBlock: bigint; toBlock: bigint },
): Promise<GetMessageSentEventsReturnType> {
  const events: GetMessageSentEventsReturnType = [];
  for (let start = args.fromBlock; start <= args.toBlock; start += MAX_GET_LOGS_BLOCK_RANGE) {
    // Inclusive window of at most MAX_GET_LOGS_BLOCK_RANGE blocks, clamped to the requested end.
    const windowEnd = start + MAX_GET_LOGS_BLOCK_RANGE - 1n;
    const end = windowEnd < args.toBlock ? windowEnd : args.toBlock;
    events.push(...(await getMessageSentEvents(client, { address: args.address, fromBlock: start, toBlock: end })));
  }
  return events;
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
    abi: [...L2_MERKLE_ROOT_ADDED_EVENT_ABI, ...L2_MESSAGING_BLOCK_ANCHORED_EVENT_ABI],
    eventName: ["L2MerkleRootAdded", "L2MessagingBlockAnchored"],
    logs: filteredLogs,
  });

  for (const log of parsedLogs) {
    if (log.eventName === "L2MerkleRootAdded") {
      treeDepth = parseInt(log.args.treeDepth.toString());
      l2MerkleRoots.push(log.args.l2MerkleRoot);
    } else {
      // parseEventLogs is scoped to the two events above, so any non-merkle log is L2MessagingBlockAnchored.
      blocksNumber.push(parseInt(log.args.l2Block.toString()));
    }
  }

  if (l2MerkleRoots.length === 0) {
    throw new EventNotFoundInFinalizationDataError({
      transactionHash: parameters.transactionHash,
      eventName: "L2MerkleRootAdded",
    });
  }

  if (blocksNumber.length === 0) {
    throw new EventNotFoundInFinalizationDataError({
      transactionHash: parameters.transactionHash,
      eventName: "L2MessagingBlockAnchored",
    });
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

// Exported for unit testing only; not re-exported from the package entrypoint (`src/index.ts`).
export function getMessageSiblings(messageHash: Hex, messageHashes: Hex[], treeDepth: number): Hex[] {
  const numberOfMessagesInTrees = 2 ** treeDepth;
  const messageHashesLength = messageHashes.length;

  const messageHashIndex = messageHashes.indexOf(messageHash);

  if (messageHashIndex === -1) {
    throw new MessageNotFoundError({ hash: messageHash });
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
