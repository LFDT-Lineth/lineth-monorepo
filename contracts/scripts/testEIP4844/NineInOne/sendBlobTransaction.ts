import * as kzg from "c-kzg";
import { LineaRollup__factory, type ILineaRollupBase } from "contracts/typechain-types";
import { Transaction, Wallet, ethers } from "ethers";

import aggregateProof1to305 from "./aggregatedProof-1-305.json";
import submissionDataJson1 from "./blocks-1-46.json";
import submissionDataJson4 from "./blocks-115-155.json";
import submissionDataJson5 from "./blocks-156-175.json";
import submissionDataJson6 from "./blocks-176-206.json";
import submissionDataJson7 from "./blocks-207-228.json";
import submissionDataJson8 from "./blocks-229-265.json";
import submissionDataJson9 from "./blocks-266-305.json";
import submissionDataJson2 from "./blocks-47-81.json";
import submissionDataJson3 from "./blocks-82-114.json";

const chainId = 31648428;

// Each fixture predates the blockhash-centric ABI cutover and only carries `parentStateRootHash` /
// `finalStateRootHash` (no real L2 block hashes). We reuse those hashes as stand-in block hashes below —
// see `FixtureSubmission` in `contracts/test/hardhat/common/helpers/dataGeneration.ts` for the same convention.
const dataItems = [
  submissionDataJson1,
  submissionDataJson2,
  submissionDataJson3,
  submissionDataJson4,
  submissionDataJson5,
  submissionDataJson6,
  submissionDataJson7,
  submissionDataJson8,
  submissionDataJson9,
];

/**
 * A single blob's submission data plus the shnarf chain values computed for it.
 * @dev `submitBlobs` no longer takes a KZG proof or data-evaluation claim: the point-evaluation
 *   precompile-based verification was removed from the contract in favor of relying on the EVM's
 *   native `blobhash()` (the beacon chain already guarantees blob/commitment validity for a landed
 *   EIP-4844 transaction).
 */
type BlobEntry = {
  finalBlockHash: string;
  dataHash: string;
  compressedData: string;
  parentShnarf: string;
  expectedShnarf: string;
};

/**
 * Mirrors the Solidity 3-field _computeShnarf: keccak256(abi.encodePacked(parentShnarf, finalBlockHash, dataHash)).
 */
function computeShnarfV2(parentShnarf: string, finalBlockHash: string, dataHash: string): string {
  return ethers.keccak256(ethers.concat([parentShnarf, finalBlockHash, dataHash]));
}

function computeGenesisShnarf(initialBlockHash: string): string {
  return computeShnarfV2(ethers.ZeroHash, initialBlockHash, ethers.ZeroHash);
}

/**
 * EIP-4844 versioned blob hash from a KZG commitment: 0x01 || sha256(commitment)[1:].
 * This is what `blobhash()` returns on-chain for the blob carrying this commitment.
 */
function computeBlobVersionedHash(commitment: string): string {
  return `0x01${ethers.sha256(commitment).slice(4)}`;
}

/**
 * Builds the blob submission chain (final block hash + shnarf per blob) from the JSON fixtures.
 * @dev The fixtures already carry the real KZG `commitment` for their `compressedData`, so it's reused
 *   directly here rather than recomputed — c-kzg/ethers will derive the identical commitment when the
 *   blob transaction is built and signed in `submitBlob`.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function buildBlobChain(dataSet: any[]): BlobEntry[] {
  const initialBlockHash = dataSet[0].parentStateRootHash;
  let parentShnarf = computeGenesisShnarf(initialBlockHash);

  return dataSet.map((data) => {
    const compressedData = ethers.hexlify(ethers.decodeBase64(data.compressedData));
    const finalBlockHash = data.finalStateRootHash;
    const dataHash = computeBlobVersionedHash(data.commitment);
    const expectedShnarf = computeShnarfV2(parentShnarf, finalBlockHash, dataHash);

    const entry: BlobEntry = { finalBlockHash, dataHash, compressedData, parentShnarf, expectedShnarf };
    parentShnarf = expectedShnarf;
    return entry;
  });
}

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable`);
  }

  return envVariable;
}

async function main() {
  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);

  kzg.loadTrustedSetup(0, `${__dirname}/trusted_setup.txt`);

  const blobChain = buildBlobChain(dataItems);

  const encodedCall = LineaRollup__factory.createInterface().encodeFunctionData("submitBlobs", [
    blobChain.map((blob) => blob.finalBlockHash),
    blobChain[0].parentShnarf,
    blobChain[blobChain.length - 1].expectedShnarf,
  ]);

  await submitBlob(
    provider,
    wallet,
    encodedCall,
    destinationAddress,
    blobChain.map((blob) => blob.compressedData),
  );

  await sendMessage();

  await sendProof(aggregateProof1to305, blobChain);
}

async function sendMessage() {
  console.log("sending the message");

  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);

  const encodedCall = ethers.concat([
    "0x9f3ce55a",
    ethers.AbiCoder.defaultAbiCoder().encode(
      ["address", "uint256", "bytes"],
      ["0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC", "50000000000000000", "0x"],
    ),
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.getFeeData();
  const nonce = await provider.getTransactionCount(wallet.address);

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: destinationAddress,
    chainId,
    nonce,
    value: 1050000000000000000n,
    gasLimit: 5_000_000,
  });

  const tx = await wallet.sendTransaction(transaction);
  const receipt = await tx.wait();
  console.log({ transaction: tx, receipt });
}

/**
 * Builds and broadcasts the EIP-4844 blob transaction carrying `_submitBlobsCall`.
 * @dev Only `blobs` + `kzg` are supplied; ethers derives the commitments, proofs and versioned hashes
 *   for the transaction's blob sidecar itself (no need to compute them manually, see also
 *   `contracts/test/hardhat/rollup/helpers/blob.ts#buildBlobTransaction`).
 */
async function submitBlob(
  provider: ethers.JsonRpcProvider,
  wallet: Wallet,
  submitBlobsCall: string,
  destinationAddress: string,
  compressedBlobs: string[],
) {
  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.getFeeData();
  const nonce = await provider.getTransactionCount(wallet.address);

  console.log(submitBlobsCall);

  const transaction = Transaction.from({
    data: submitBlobsCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: destinationAddress,
    chainId,
    type: 3,
    nonce,
    value: 0,
    kzg,
    blobs: compressedBlobs,
    gasLimit: 5_000_000,
    maxFeePerBlobGas: maxFeePerGas!,
  });

  const tx = await wallet.sendTransaction(transaction);
  const receipt = await tx.wait();

  console.log("BlobTX Hash: ", tx.hash);
  console.log(`BlobTX receipt: ${JSON.stringify(receipt, null, 2)}`);
}

/**
 * Submits the aggregated proof via `finalizeBlocks`.
 * @dev Assumes this is the *first* finalization after a fresh deploy (no prior finalized state, no
 *   forced transactions yet), matching the `aggregatedProof-1-305.json` fixture. `parentBlockHash` and
 *   `lastFinalizedTimestamp` must match whatever `INITIAL_L2_BLOCK_HASH` / `L2_GENESIS_TIMESTAMP` the
 *   contract was actually deployed with (see `contracts/deploy/03_deploy_LineaRollup.ts`) — the values
 *   below assume the deploy step reused the same fixture values as `blocks-1-46.json`.
 * @dev `aggregatedProof-1-305.json`'s `aggregatedProof`/`finalShnarf` were generated for the legacy
 *   5-field/Horner-Y public input formula, not the current blockhash-centric one, so verification will
 *   fail on-chain until the fixture is regenerated by the prover against the current ABI.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
async function sendProof(proofFile: any, blobChain: BlobEntry[]) {
  console.log("proof");

  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("DEPLOYER_PRIVATE_KEY");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);

  const lastBlob = blobChain[blobChain.length - 1];
  const initialBlockHash = dataItems[0].parentStateRootHash;

  const finalizationData: ILineaRollupBase.FinalizationDataV5Struct = {
    parentStateRootHash: proofFile.parentStateRootHash,
    parentBlockHash: initialBlockHash,
    endBlockNumber: BigInt(proofFile.finalBlockNumber),
    shnarfData: {
      parentShnarf: lastBlob.parentShnarf,
      snarkHash: ethers.ZeroHash,
      finalStateRootHash: ethers.ZeroHash,
      dataEvaluationPoint: ethers.ZeroHash,
      dataEvaluationClaim: ethers.ZeroHash,
    },
    lastFinalizedTimestamp: BigInt(proofFile.parentAggregationLastBlockTimestamp),
    finalTimestamp: BigInt(proofFile.finalTimestamp),
    lastFinalizedL1RollingHash: ethers.ZeroHash,
    l1RollingHash: proofFile.l1RollingHash,
    lastFinalizedL1RollingHashMessageNumber: 0n,
    l1RollingHashMessageNumber: BigInt(proofFile.l1RollingHashMessageNumber),
    l2MerkleTreesDepth: BigInt(proofFile.l2MerkleTreesDepth),
    lastFinalizedForcedTransactionNumber: 0n,
    finalForcedTransactionNumber: 0n,
    lastFinalizedForcedTransactionRollingHash: ethers.ZeroHash,
    finalBlockHash: lastBlob.finalBlockHash,
    finalBlobHash: lastBlob.dataHash,
    l2MerkleRoots: proofFile.l2MerkleRoots,
    filteredAddresses: [],
    verifierKeys: [],
    l2MessagingBlocksOffsets: proofFile.l2MessagingBlocksOffsets,
  };

  const encodedCall = LineaRollup__factory.createInterface().encodeFunctionData("finalizeBlocks", [
    proofFile.aggregatedProof,
    proofFile.aggregatedVerifierIndex,
    finalizationData,
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.getFeeData();
  const nonce = await provider.getTransactionCount(wallet.address);

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: destinationAddress,
    chainId,
    nonce,
    value: 0,
    gasLimit: 5_000_000,
  });

  const tx = await wallet.sendTransaction(transaction);
  const receipt = await tx.wait();
  console.log({ transaction: tx, receipt });
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
