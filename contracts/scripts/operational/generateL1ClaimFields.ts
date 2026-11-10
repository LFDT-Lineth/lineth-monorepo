/**
 * generateL1ClaimFields.ts
 *
 * Generates the complete `ClaimMessageWithProofParams` struct required to call
 * `claimMessageWithProof()` on the L1 LinethRollup contract, for a given L2->L1
 * message hash.
 *
 * This is meant to be used as a standalone script, not as part of a Hardhat task or other build process.
 * Additionally, it does not use the SDK as a dependency to avoid circular dependencies.
 *
 * Usage (env):
 *   MESSAGE_HASH=0x... \
 *   L1_RPC_URL=https://... \
 *   L2_RPC_URL=https://... \
 *   LINETH_ROLLUP_ADDRESS=0x... \
 *   L2_MESSAGE_SERVICE_ADDRESS=0x... \
 *   pnpm exec tsx scripts/operational/generateL1ClaimFields.ts
 *
 * Usage (flags + env fallback):
 *   pnpm exec tsx scripts/operational/generateL1ClaimFields.ts \
 *     --message-hash 0x... \
 *     --l1-rpc-url https://... \
 *     --l2-rpc-url https://... \
 *     --lineth-rollup-address 0x... \
 *     --l2-message-service-address 0x... \
 *     --out ./claim-params.json \
 *     --pretty
 *
 * Optional search bounds:
 *   --l1-from-block <number> --l1-to-block <number>
 *   --l2-from-block <number> --l2-to-block <number>
 */

import { writeFileSync } from "node:fs";
import {
  type Address,
  type BlockNumber,
  type BlockTag,
  type Hex,
  type PublicClient,
  type TransactionReceipt,
  createPublicClient,
  encodePacked,
  getAddress,
  http,
  isHex,
  keccak256,
  parseAbi,
  parseEventLogs,
  zeroAddress,
} from "viem";

import { encodeSendMessage } from "../../common/helpers/encoding.js";
import { getRequiredEnvVar } from "../../common/helpers/environment.js";
import { getCliOrEnvValue } from "../../common/helpers/environmentHelper.js";

const MESSAGE_SENT_ABI = parseAbi([
  "event MessageSent(address indexed _from, address indexed _to, uint256 _fee, uint256 _value, uint256 _nonce, bytes _calldata, bytes32 indexed _messageHash)",
]);
const L2_BLOCK_ANCHORED_ABI = parseAbi(["event L2MessagingBlockAnchored(uint256 indexed l2Block)"]);
const L2_MERKLE_ROOT_ADDED_ABI = parseAbi([
  "event L2MerkleRootAdded(bytes32 indexed l2MerkleRoot, uint256 indexed treeDepth)",
]);
const FINALIZATION_ABI = [...L2_MERKLE_ROOT_ADDED_ABI, ...L2_BLOCK_ANCHORED_ABI];
const ZERO_HASH: Hex = "0x0000000000000000000000000000000000000000000000000000000000000000";
const TOTAL_STEPS = 6;

type BlockBound = number | "earliest" | "latest";

interface ScriptConfig {
  messageHash: Hex;
  l1RpcUrl: string;
  l2RpcUrl: string;
  linethRollupAddress: Address;
  l2MessageServiceAddress: Address;
  feeRecipient: Address;
  outputPath: string | undefined;
  pretty: boolean;
  l1FromBlock: BlockBound;
  l1ToBlock: BlockBound;
  l2FromBlock: BlockBound;
  l2ToBlock: BlockBound;
}

interface MessageSentData {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: Hex;
  l2BlockNumber: number;
}

interface FinalizationInfo {
  l2MerkleRoots: Hex[];
  treeDepth: number;
  l2BlockRange: { start: number; end: number };
}

interface ClaimMessageWithProofParams {
  proof: Hex[];
  messageNumber: string;
  leafIndex: number;
  from: Address;
  to: Address;
  fee: string;
  value: string;
  feeRecipient: Address;
  merkleRoot: Hex;
  data: Hex;
}

function getCliValueFromEqualsSyntax(argName: string): string | undefined {
  const prefix = `--${argName}=`;
  const match = process.argv.find((arg) => arg.startsWith(prefix));
  if (!match) {
    return undefined;
  }
  const value = match.slice(prefix.length);
  return value.length > 0 ? value : undefined;
}

function getOptionalCliOrEnvValue(argName: string, envName: string): string | undefined {
  const equalsValue = getCliValueFromEqualsSyntax(argName);
  if (equalsValue) {
    return equalsValue;
  }
  return getCliOrEnvValue(`--${argName}`, envName);
}

function getRequiredCliOrEnvValue(argName: string, envName: string): string {
  const optionalValue = getOptionalCliOrEnvValue(argName, envName);
  if (optionalValue) {
    return optionalValue;
  }
  return getRequiredEnvVar(envName);
}

function hasCliFlag(argName: string): boolean {
  return process.argv.includes(`--${argName}`);
}

function parseBlockBound(rawValue: string | undefined, label: string): BlockBound {
  if (!rawValue || rawValue.length === 0) {
    return label.endsWith("from") ? "earliest" : "latest";
  }
  if (rawValue === "earliest" || rawValue === "latest") {
    return rawValue;
  }
  if (!/^\d+$/.test(rawValue)) {
    throw new Error(`Invalid ${label} block value "${rawValue}". Use "earliest", "latest", or an integer.`);
  }
  return Number(rawValue);
}

function toBlockParameter(bound: BlockBound): BlockNumber | BlockTag {
  return typeof bound === "number" ? BigInt(bound) : bound;
}

function normalizeHex(value: string): string {
  return value.toLowerCase();
}

function findHashIndex(hashes: string[], targetHash: string): number {
  const normalizedTarget = normalizeHex(targetHash);
  return hashes.findIndex((hash) => normalizeHex(hash) === normalizedTarget);
}

function stepLabel(step: number): string {
  return `[${step}/${TOTAL_STEPS}]`;
}

function info(step: number, message: string): void {
  console.log(`${stepLabel(step)} ${message}`);
}

function loadConfig(): ScriptConfig {
  const l1FromRaw = getOptionalCliOrEnvValue("l1-from-block", "L1_FROM_BLOCK");
  const l1ToRaw = getOptionalCliOrEnvValue("l1-to-block", "L1_TO_BLOCK");
  const l2FromRaw = getOptionalCliOrEnvValue("l2-from-block", "L2_FROM_BLOCK");
  const l2ToRaw = getOptionalCliOrEnvValue("l2-to-block", "L2_TO_BLOCK");

  const messageHash = getRequiredCliOrEnvValue("message-hash", "MESSAGE_HASH");
  if (!isHex(messageHash)) {
    throw new Error(`Invalid message hash "${messageHash}"`);
  }

  return {
    messageHash,
    l1RpcUrl: getRequiredCliOrEnvValue("l1-rpc-url", "L1_RPC_URL"),
    l2RpcUrl: getRequiredCliOrEnvValue("l2-rpc-url", "L2_RPC_URL"),
    linethRollupAddress: getAddress(getRequiredCliOrEnvValue("lineth-rollup-address", "LINETH_ROLLUP_ADDRESS")),
    l2MessageServiceAddress: getAddress(
      getRequiredCliOrEnvValue("l2-message-service-address", "L2_MESSAGE_SERVICE_ADDRESS"),
    ),
    feeRecipient: getAddress(getOptionalCliOrEnvValue("fee-recipient", "FEE_RECIPIENT") ?? zeroAddress),
    outputPath: getCliValueFromEqualsSyntax("out") ?? getCliOrEnvValue("--out", "CLAIM_PARAMS_OUT"),
    pretty: hasCliFlag("pretty"),
    l1FromBlock: parseBlockBound(l1FromRaw, "l1-from"),
    l1ToBlock: parseBlockBound(l1ToRaw, "l1-to"),
    l2FromBlock: parseBlockBound(l2FromRaw, "l2-from"),
    l2ToBlock: parseBlockBound(l2ToRaw, "l2-to"),
  };
}

function smtHash(left: Hex, right: Hex): Hex {
  return keccak256(encodePacked(["bytes32", "bytes32"], [left, right]));
}

class SparseMerkleTree {
  private depth: number;
  private nodeMap = new Map<bigint, Hex>();
  private zeroHashes: Hex[];

  constructor(depth: number) {
    if (depth <= 1) {
      throw new Error("Merkle tree depth must be greater than 1");
    }
    this.depth = depth;
    this.zeroHashes = [ZERO_HASH];
    for (let i = 1; i <= depth; i++) {
      this.zeroHashes[i] = smtHash(this.zeroHashes[i - 1], this.zeroHashes[i - 1]);
    }
    this.nodeMap.set(1n, this.zeroHashes[depth]);
  }

  public getRoot(): Hex {
    const root = this.nodeMap.get(1n);
    if (root === undefined) {
      throw new Error("Merkle tree root is not set");
    }
    return root;
  }

  public addLeaf(idx: number, leafHash: Hex): void {
    const nodeIdx = this.leafNodeIndex(idx);
    this.nodeMap.set(nodeIdx, leafHash);
    this.rehashPath(nodeIdx);
  }

  public getProof(idx: number): { proof: Hex[]; root: Hex; leafIndex: number } {
    const leafIdx = this.leafNodeIndex(idx);
    const leafHash = this.nodeMap.get(leafIdx) ?? this.zeroHashes[0];
    if (leafHash === this.zeroHashes[0]) {
      throw new Error("Leaf does not exist");
    }

    const proof: Hex[] = [];
    let nodeIdx = leafIdx;
    for (let level = this.depth; level > 0; level--) {
      const height = this.depth - level;
      const siblingIdx = this.siblingIndex(nodeIdx);
      proof.push(this.nodeMap.get(siblingIdx) ?? this.zeroHashes[height]);
      nodeIdx = this.parentIndex(nodeIdx);
    }

    return { proof, root: this.getRoot(), leafIndex: idx };
  }

  private leafNodeIndex(idx: number): bigint {
    if (idx < 0 || idx >= 1 << this.depth) {
      throw new Error("Leaf index is out of range");
    }
    return (1n << BigInt(this.depth)) + BigInt(idx);
  }

  private parentIndex(nodeIdx: bigint): bigint {
    return nodeIdx >> 1n;
  }

  private siblingIndex(nodeIdx: bigint): bigint {
    return (nodeIdx & 1n) === 1n ? nodeIdx - 1n : nodeIdx + 1n;
  }

  private rehashPath(startIdx: bigint): void {
    let nodeIdx = startIdx;
    for (let level = this.depth; level > 0; level--) {
      const height = this.depth - level;
      const siblingIdx = this.siblingIndex(nodeIdx);
      const leftIdx = (nodeIdx & 1n) === 1n ? siblingIdx : nodeIdx;
      const rightIdx = (nodeIdx & 1n) === 1n ? nodeIdx : siblingIdx;
      const fallback = this.zeroHashes[height];
      const leftHash = this.nodeMap.get(leftIdx) ?? fallback;
      const rightHash = this.nodeMap.get(rightIdx) ?? fallback;
      nodeIdx = this.parentIndex(nodeIdx);
      this.nodeMap.set(nodeIdx, smtHash(leftHash, rightHash));
    }
  }
}

function getMessageSiblings(targetMessageHash: Hex, messageHashes: Hex[], treeDepth: number): Hex[] {
  const treeCapacity = 2 ** treeDepth;
  const messageHashIndex = findHashIndex(messageHashes, targetMessageHash);
  if (messageHashIndex === -1) {
    throw new Error(`Message hash ${targetMessageHash} not found in the finalization message set`);
  }

  const chunkStart = Math.floor(messageHashIndex / treeCapacity) * treeCapacity;
  const chunkEnd = Math.min(messageHashes.length, chunkStart + treeCapacity);
  const siblings = messageHashes.slice(chunkStart, chunkEnd);

  const remainder = siblings.length % treeCapacity;
  if (remainder !== 0) {
    siblings.push(...Array<Hex>(treeCapacity - remainder).fill(ZERO_HASH));
  }

  return siblings;
}

function parseFinalizationReceipt(receipt: TransactionReceipt, linethRollupAddress: Address): FinalizationInfo {
  const l2MerkleRoots: Hex[] = [];
  const anchoredBlocks: number[] = [];
  let treeDepth = 0;

  // Unrelated events from the same contract address are skipped by parseEventLogs.
  const parsedLogs = parseEventLogs({
    abi: FINALIZATION_ABI,
    logs: receipt.logs.filter((log) => normalizeHex(log.address) === normalizeHex(linethRollupAddress)),
  });
  for (const parsed of parsedLogs) {
    if (parsed.eventName === "L2MerkleRootAdded") {
      l2MerkleRoots.push(parsed.args.l2MerkleRoot);
      treeDepth = Number(parsed.args.treeDepth);
    } else if (parsed.eventName === "L2MessagingBlockAnchored") {
      anchoredBlocks.push(Number(parsed.args.l2Block));
    }
  }

  if (l2MerkleRoots.length === 0) {
    throw new Error("No L2MerkleRootAdded events found in finalization receipt");
  }
  if (anchoredBlocks.length === 0) {
    throw new Error("No L2MessagingBlockAnchored events found in finalization receipt");
  }

  return {
    l2MerkleRoots,
    treeDepth,
    l2BlockRange: {
      start: Math.min(...anchoredBlocks),
      end: Math.max(...anchoredBlocks),
    },
  };
}

async function findMessageSentEvent(
  l2Client: PublicClient,
  l2MessageServiceAddress: Address,
  targetMessageHash: Hex,
  fromBlock: BlockBound,
  toBlock: BlockBound,
): Promise<MessageSentData> {
  const messageSentEvents = await l2Client.getContractEvents({
    address: l2MessageServiceAddress,
    abi: MESSAGE_SENT_ABI,
    eventName: "MessageSent",
    args: { _messageHash: targetMessageHash },
    fromBlock: toBlockParameter(fromBlock),
    toBlock: toBlockParameter(toBlock),
  });

  if (messageSentEvents.length === 0) {
    throw new Error(
      `No MessageSent event found for message hash ${targetMessageHash} in range [${fromBlock}, ${toBlock}]`,
    );
  }

  const firstEvent = messageSentEvents[0];
  const { _from, _to, _fee, _value, _nonce, _calldata } = firstEvent.args;
  if (
    _from === undefined ||
    _to === undefined ||
    _fee === undefined ||
    _value === undefined ||
    _nonce === undefined ||
    _calldata === undefined
  ) {
    throw new Error("Failed to parse MessageSent event");
  }

  return {
    from: _from,
    to: _to,
    fee: _fee,
    value: _value,
    nonce: _nonce,
    calldata: _calldata,
    l2BlockNumber: Number(firstEvent.blockNumber),
  };
}

function verifyMessageHash(message: MessageSentData, targetMessageHash: Hex): void {
  const encodedMessage = encodeSendMessage(
    message.from,
    message.to,
    message.fee,
    message.value,
    message.nonce,
    message.calldata,
  );
  const computedHash = keccak256(encodedMessage);
  if (normalizeHex(computedHash) !== normalizeHex(targetMessageHash)) {
    throw new Error(`Computed hash ${computedHash} does not match requested hash ${targetMessageHash}`);
  }
}

async function findFinalizationTxHash(
  l1Client: PublicClient,
  linethRollupAddress: Address,
  l2BlockNumber: number,
  fromBlock: BlockBound,
  toBlock: BlockBound,
): Promise<Hex> {
  const anchoredEvents = await l1Client.getContractEvents({
    address: linethRollupAddress,
    abi: L2_BLOCK_ANCHORED_ABI,
    eventName: "L2MessagingBlockAnchored",
    args: { l2Block: BigInt(l2BlockNumber) },
    fromBlock: toBlockParameter(fromBlock),
    toBlock: toBlockParameter(toBlock),
  });

  if (anchoredEvents.length === 0) {
    throw new Error(
      `No L2MessagingBlockAnchored event found for L2 block ${l2BlockNumber}. ` +
        `Searched range [${fromBlock}, ${toBlock}]. The block may not be finalized yet.`,
    );
  }
  const finalizationTxHash = anchoredEvents[0].transactionHash;
  if (finalizationTxHash === null) {
    throw new Error(`L2MessagingBlockAnchored event for L2 block ${l2BlockNumber} is still pending.`);
  }
  return finalizationTxHash;
}

async function collectMessageHashesInRange(
  l2Client: PublicClient,
  l2MessageServiceAddress: Address,
  startBlock: number,
  endBlock: number,
): Promise<Hex[]> {
  const allEvents = await l2Client.getContractEvents({
    address: l2MessageServiceAddress,
    abi: MESSAGE_SENT_ABI,
    eventName: "MessageSent",
    fromBlock: BigInt(startBlock),
    toBlock: BigInt(endBlock),
  });
  const hashes = allEvents.map((event) => {
    const messageHash = event.args._messageHash;
    if (messageHash === undefined) {
      throw new Error("Failed to parse MessageSent event");
    }
    return messageHash;
  });

  if (hashes.length === 0) {
    throw new Error(`No MessageSent events found in L2 block range [${startBlock}, ${endBlock}]`);
  }

  return hashes;
}

function buildClaimParams(
  message: MessageSentData,
  proof: { proof: Hex[]; root: Hex; leafIndex: number },
  feeRecipient: Address,
): ClaimMessageWithProofParams {
  return {
    proof: proof.proof,
    messageNumber: message.nonce.toString(),
    leafIndex: proof.leafIndex,
    from: message.from,
    to: message.to,
    fee: message.fee.toString(),
    value: message.value.toString(),
    feeRecipient,
    merkleRoot: proof.root,
    data: message.calldata,
  };
}

function writeOutput(claimParams: ClaimMessageWithProofParams, outputPath?: string): void {
  const output = JSON.stringify(claimParams, null, 2);
  if (outputPath) {
    writeFileSync(outputPath, `${output}\n`, "utf8");
    console.log(`Wrote claim params to ${outputPath}`);
    return;
  }
  console.log(output);
}

async function main() {
  const config = loadConfig();
  const l1Client = createPublicClient({ transport: http(config.l1RpcUrl) });
  const l2Client = createPublicClient({ transport: http(config.l2RpcUrl) });

  info(1, `Searching L2 MessageSent event for hash ${config.messageHash}...`);
  const message = await findMessageSentEvent(
    l2Client,
    config.l2MessageServiceAddress,
    config.messageHash,
    config.l2FromBlock,
    config.l2ToBlock,
  );
  verifyMessageHash(message, config.messageHash);
  info(1, `Found message in L2 block ${message.l2BlockNumber}`);
  if (config.pretty) {
    console.log(
      `    from=${message.from} to=${message.to} nonce=${message.nonce.toString()} fee=${message.fee.toString()} value=${message.value.toString()}`,
    );
  }

  info(2, `Searching L1 finalization for L2 block ${message.l2BlockNumber}...`);
  const finalizationTxHash = await findFinalizationTxHash(
    l1Client,
    config.linethRollupAddress,
    message.l2BlockNumber,
    config.l1FromBlock,
    config.l1ToBlock,
  );
  info(2, `Found finalization tx ${finalizationTxHash}`);

  info(3, "Parsing finalization receipt...");
  const finalizationReceipt = await l1Client.getTransactionReceipt({ hash: finalizationTxHash });
  if (!finalizationReceipt) {
    throw new Error(`Could not fetch receipt for finalization tx ${finalizationTxHash}`);
  }
  const finalization = parseFinalizationReceipt(finalizationReceipt, config.linethRollupAddress);
  info(
    3,
    `Finalization range ${finalization.l2BlockRange.start}-${finalization.l2BlockRange.end}, tree depth ${finalization.treeDepth}, roots ${finalization.l2MerkleRoots.length}`,
  );

  info(4, "Collecting MessageSent hashes in the finalization L2 range...");
  const allMessageHashes = await collectMessageHashesInRange(
    l2Client,
    config.l2MessageServiceAddress,
    finalization.l2BlockRange.start,
    finalization.l2BlockRange.end,
  );
  info(4, `Collected ${allMessageHashes.length} message hash(es)`);

  info(5, "Building sparse Merkle tree and generating proof...");
  const siblings = getMessageSiblings(config.messageHash, allMessageHashes, finalization.treeDepth);
  const tree = new SparseMerkleTree(finalization.treeDepth);
  for (const [index, leaf] of siblings.entries()) {
    if (leaf !== ZERO_HASH) {
      tree.addLeaf(index, leaf);
    }
  }
  const computedRoot = tree.getRoot();
  const matchingRoot = finalization.l2MerkleRoots.find((root) => normalizeHex(root) === normalizeHex(computedRoot));
  if (!matchingRoot) {
    throw new Error(
      `Computed merkle root ${computedRoot} is not part of finalization roots: ${finalization.l2MerkleRoots.join(", ")}`,
    );
  }
  const localIndex = findHashIndex(siblings, config.messageHash);
  if (localIndex === -1) {
    throw new Error(`Message hash ${config.messageHash} not found in siblings set for computed proof chunk`);
  }
  const proofData = tree.getProof(localIndex);
  info(5, `Generated proof with leafIndex=${proofData.leafIndex}`);

  info(6, "Building ClaimMessageWithProofParams...");
  const claimParams = buildClaimParams(message, proofData, config.feeRecipient);
  if (config.pretty) {
    console.log("=== Verification Summary ===");
    console.log(`messageHash:     ${config.messageHash}`);
    console.log(`finalizationTx:  ${finalizationTxHash}`);
    console.log(`l2BlockRange:    [${finalization.l2BlockRange.start}, ${finalization.l2BlockRange.end}]`);
    console.log(`merkleRoot:      ${claimParams.merkleRoot}`);
    console.log(`leafIndex:       ${claimParams.leafIndex}`);
  }

  console.log("\n=== ClaimMessageWithProofParams ===");
  writeOutput(claimParams, config.outputPath);
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
