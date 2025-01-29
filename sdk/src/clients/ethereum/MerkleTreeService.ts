import { Block, TransactionReceipt, TransactionRequest, TransactionResponse } from "ethers";
import { SparseMerkleTreeFactory } from "../../utils/merkleTree/MerkleTreeFactory";
import {
  ILineaRollupLogClient,
  FinalizationMessagingInfo,
  IMerkleTreeService,
  Proof,
} from "../../core/clients/ethereum";
import { IL2MessageServiceLogClient } from "../../core/clients/linea";
import {
  L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE,
  L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE,
  ZERO_HASH,
} from "../../core/constants";
import { LineaRollup, LineaRollup__factory } from "../../contracts/typechain";
import { IProvider } from "../../core/clients/IProvider";
import { BrowserProvider, Provider } from "../providers";
import { makeBaseError } from "../../core/errors/utils";

export class MerkleTreeService implements IMerkleTreeService {
  private readonly contract: LineaRollup;

  /**
   * Initializes a new instance of the `MerkleTreeService`.
   *
   * @param {IProvider} provider - The provider for interacting with the blockchain.
   * @param {string} contractAddress - The address of the Linea Rollup contract.
   * @param {ILineaRollupLogClient} lineaRollupLogClient - An instance of a class implementing the `ILineaRollupLogClient` interface for fetching events from ethereum.
   * @param {IL2MessageServiceLogClient} l2MessageServiceLogClient - An instance of a class implementing the `IL2MessageServiceLogClient` interface for fetching events from linea.
   * @param {number} l2MessageTreeDepth - The depth of the L2 message tree.
   */
  constructor(
    private readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      Provider | BrowserProvider
    >,
    private readonly contractAddress: string,
    private readonly lineaRollupLogClient: ILineaRollupLogClient,
    private readonly l2MessageServiceLogClient: IL2MessageServiceLogClient,
    private readonly l2MessageTreeDepth: number,
  ) {
    this.contract = LineaRollup__factory.connect(contractAddress, this.provider);
  }

  /**
   * Retrieves the message proof for claiming the message on L1.
   * @param {string} messageHash - The message hash.
   * @returns {Promise<Proof>} The merkle root, the merkle proof and the message leaf index.
   */
  public async getMessageProof(messageHash: string): Promise<Proof> {
    const [messageEvent] = await this.l2MessageServiceLogClient.getMessageSentEventsByMessageHash({ messageHash });

    if (!messageEvent) {
      throw makeBaseError(`Message hash does not exist on L2. Message hash: ${messageHash}`);
    }

    const [l2MessagingBlockAnchoredEvent] = await this.lineaRollupLogClient.getL2MessagingBlockAnchoredEvents({
      filters: { l2Block: BigInt(messageEvent.blockNumber) },
    });

    if (!l2MessagingBlockAnchoredEvent) {
      throw makeBaseError(`L2 block number ${messageEvent.blockNumber} has not been finalized on L1.`);
    }

    const finalizationInfo = await this.getFinalizationMessagingInfo(l2MessagingBlockAnchoredEvent.transactionHash);

    const l2MessageHashesInBlockRange = await this.getL2MessageHashesInBlockRange(
      finalizationInfo.l2MessagingBlocksRange.startingBlock,
      finalizationInfo.l2MessagingBlocksRange.endBlock,
    );

    const l2messages = this.getMessageSiblings(messageHash, l2MessageHashesInBlockRange, finalizationInfo.treeDepth);

    const merkleTreeFactory = new SparseMerkleTreeFactory(this.l2MessageTreeDepth);
    const tree = merkleTreeFactory.createAndAddLeaves(l2messages);

    if (!finalizationInfo.l2MerkleRoots.includes(tree.getRoot())) {
      throw makeBaseError("Merkle tree build failed.");
    }

    return tree.getProof(l2messages.indexOf(messageHash));
  }

  /**
   * Retrieves the finalization messaging info. This function is used in the L1 claiming flow.
   * @param {string} transactionHash - The finalization transaction hash.
   * @returns {Promise<FinalizationMessagingInfo>} The finalization messaging info: l2MessagingBlocksRange, l2MerkleRoots, treeDepth.
   */
  public async getFinalizationMessagingInfo(transactionHash: string): Promise<FinalizationMessagingInfo> {
    const receipt = await this.provider.getTransactionReceipt(transactionHash);

    if (!receipt || receipt.logs.length === 0) {
      throw makeBaseError(`Transaction does not exist or no logs found in this transaction: ${transactionHash}.`);
    }

    let treeDepth = 0;
    const l2MerkleRoots: string[] = [];
    const blocksNumber: number[] = [];

    const filteredLogs = receipt.logs.filter((log) => log.address === this.contractAddress);

    for (const log of filteredLogs) {
      const parsedLog = this.contract.interface.parseLog(log);

      if (log.topics[0] === L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE) {
        treeDepth = parseInt(parsedLog?.args.treeDepth);
        l2MerkleRoots.push(parsedLog?.args.l2MerkleRoot);
      } else if (log.topics[0] === L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE) {
        blocksNumber.push(Number(parsedLog?.args.l2Block));
      }
    }

    if (l2MerkleRoots.length === 0) {
      throw makeBaseError(`No L2MerkleRootAdded events found in this transaction.`);
    }

    if (blocksNumber.length === 0) {
      throw makeBaseError(`No L2MessagingBlocksAnchored events found in this transaction.`);
    }

    return {
      l2MessagingBlocksRange: {
        startingBlock: Math.min(...blocksNumber),
        endBlock: Math.max(...blocksNumber),
      },
      l2MerkleRoots,
      treeDepth,
    };
  }

  /**
   * Retrieves L2 message hashes in a specific L2 block range.
   * @param {number} fromBlock - The starting block number.
   * @param {number} toBlock - The ending block number.
   * @returns {Promise<string[]>} A list of all L2 message hashes in the specified block range.
   */
  public async getL2MessageHashesInBlockRange(fromBlock: number, toBlock: number): Promise<string[]> {
    const events = await this.l2MessageServiceLogClient.getMessageSentEventsByBlockRange(fromBlock, toBlock);

    if (events.length === 0) {
      throw makeBaseError(`No MessageSent events found in this block range on L2.`);
    }

    return events.map((event) => event.messageHash);
  }

  /**
   * Retrieves message siblings for building the merkle tree. This merkle tree will be used to generate the proof to be able to claim the message on L1.
   * @param {string} messageHash - The message hash.
   * @param {string[]} messageHashes - The list of all L2 message hashes finalized in the same finalization transaction on L1.
   * @param {number} treeDepth - The merkle tree depth.
   * @returns {string[]} The message siblings.
   */
  public getMessageSiblings(messageHash: string, messageHashes: string[], treeDepth: number): string[] {
    const numberOfMessagesInTrees = 2 ** treeDepth;
    const messageHashesLength = messageHashes.length;

    const messageHashIndex = messageHashes.indexOf(messageHash);

    if (messageHashIndex === -1) {
      throw makeBaseError("Message hash not found in messages.");
    }

    const start = Math.floor(messageHashIndex / numberOfMessagesInTrees) * numberOfMessagesInTrees;
    const end = Math.min(messageHashesLength, start + numberOfMessagesInTrees);

    const siblings = messageHashes.slice(start, end);

    const remainder = siblings.length % numberOfMessagesInTrees;
    if (remainder !== 0) {
      siblings.push(...Array(numberOfMessagesInTrees - remainder).fill(ZERO_HASH));
    }

    return siblings;
  }
}
