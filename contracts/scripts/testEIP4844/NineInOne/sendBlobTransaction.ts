import cKzg from "c-kzg";
import {
  type Hex,
  type PrivateKeyAccount,
  type PublicClient,
  bytesToHex,
  commitmentsToVersionedHashes,
  concat,
  createPublicClient,
  defineKzg,
  getAddress,
  http,
  isHex,
  keccak256,
  zeroHash,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";

import aggregateProof1to305 from "./aggregatedProof-1-305.json" with { type: "json" };
import submissionDataJson1 from "./blocks-1-46.json" with { type: "json" };
import submissionDataJson4 from "./blocks-115-155.json" with { type: "json" };
import submissionDataJson5 from "./blocks-156-175.json" with { type: "json" };
import submissionDataJson6 from "./blocks-176-206.json" with { type: "json" };
import submissionDataJson7 from "./blocks-207-228.json" with { type: "json" };
import submissionDataJson8 from "./blocks-229-265.json" with { type: "json" };
import submissionDataJson9 from "./blocks-266-305.json" with { type: "json" };
import submissionDataJson2 from "./blocks-47-81.json" with { type: "json" };
import submissionDataJson3 from "./blocks-82-114.json" with { type: "json" };
import { encodeData as encodeAbiData } from "../../../common/helpers/encoding.js";

const chainId = 31648428;

type SubmissionJson = typeof submissionDataJson1;
type AggregatedProofJson = typeof aggregateProof1to305;

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

export function generateKeccak256(types: string[], values: unknown[], packed?: boolean) {
  return keccak256(encodeData(types, values, packed));
}

export const encodeData = (types: string[], values: unknown[], packed?: boolean): Hex =>
  encodeAbiData(types, values, packed);

function toHex(value: string, label: string): Hex {
  if (!isHex(value)) {
    throw new Error(`Invalid ${label}: expected a 0x-prefixed hex string`);
  }
  return value;
}

function decodeBase64(value: string): Uint8Array {
  return new Uint8Array(Buffer.from(value, "base64"));
}

export type SubmissionData = {
  parentStateRootHash: string;
  dataParentHash: string;
  finalStateRootHash: string;
  firstBlockInData: bigint;
  finalBlockInData: bigint;
  snarkHash: string;
};

export type BlobSubmissionData = {
  submissionData: SubmissionData;
  parentSubmissionData: ParentSubmissionData;
  dataEvaluationClaim: bigint;
  kzgCommitment: Hex;
  kzgProof: Hex;
};

export type ParentSubmissionData = {
  finalStateRootHash: string;
  firstBlockInData: bigint;
  finalBlockInData: bigint;
  shnarf: string;
  dataParentHash: string;
};

export function generateSubmissionDataFromJSON(
  startingBlockNumber: number,
  endingBlockNumber: number,
  parsedJSONData: SubmissionJson,
): { submissionData: SubmissionData; blob: Uint8Array } {
  const returnData = {
    parentStateRootHash: parsedJSONData.parentStateRootHash,
    dataParentHash: parsedJSONData.parentDataHash,
    finalStateRootHash: parsedJSONData.finalStateRootHash,
    firstBlockInData: BigInt(startingBlockNumber),
    finalBlockInData: BigInt(endingBlockNumber),
    snarkHash: parsedJSONData.snarkHash,
  };

  return {
    submissionData: returnData,
    blob: decodeBase64(parsedJSONData.compressedData),
  };
}

export function generateParentSubmissionData(finalStateRootHash: string, parentDataHash: string): ParentSubmissionData {
  return {
    finalStateRootHash: finalStateRootHash,
    firstBlockInData: BigInt(0),
    finalBlockInData: BigInt(0),
    shnarf: "0x47452a1b9ebadfe02bdd02f580fa1eba17680d57eec968a591644d05d78ee84f",
    dataParentHash: parentDataHash,
  };
}

export function generateParentSubmissionDataFromJson(parsedJSONData: SubmissionJson): ParentSubmissionData {
  return {
    finalStateRootHash: parsedJSONData.finalStateRootHash,
    firstBlockInData: BigInt(parsedJSONData.conflationOrder.startingBlockNumber),
    finalBlockInData: BigInt(parsedJSONData.conflationOrder.upperBoundaries.slice(-1)[0]),
    shnarf: parsedJSONData.expectedShnarf,
    dataParentHash: parsedJSONData.parentDataHash,
  };
}

export function generateSubmissionCallDataFromJSON(
  startingBlockNumber: number,
  endingBlockNumber: number,
  parsedJSONData: SubmissionJson,
): { submissionData: SubmissionData } {
  const returnData = {
    parentStateRootHash: parsedJSONData.parentStateRootHash,
    dataParentHash: parsedJSONData.parentDataHash,
    finalStateRootHash: parsedJSONData.finalStateRootHash,
    firstBlockInData: BigInt(startingBlockNumber),
    finalBlockInData: BigInt(endingBlockNumber),
    snarkHash: parsedJSONData.snarkHash,
    compressedData: bytesToHex(decodeBase64(parsedJSONData.compressedData)),
  };

  return {
    submissionData: returnData,
  };
}

function getPadded(data: Uint8Array): Uint8Array {
  const pdata = new Uint8Array(131072).fill(0);
  pdata.set(data);
  return pdata;
}

function requireEnv(name: string): string {
  const envVariable = process.env[name];
  if (!envVariable) {
    throw new Error(`Missing ${name} environment variable`);
  }

  return envVariable;
}

function kzgCommitmentsToVersionedHashes(commitments: Uint8Array[]): Hex[] {
  return [...commitmentsToVersionedHashes({ commitments: commitments.map((commitment) => bytesToHex(commitment)) })];
}

function getClients(): { provider: PublicClient; wallet: PrivateKeyAccount; destinationAddress: Hex } {
  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = toHex(requireEnv("DEPLOYER_PRIVATE_KEY"), "DEPLOYER_PRIVATE_KEY");
  const destinationAddress = getAddress(requireEnv("DESTINATION_ADDRESS"));

  const provider = createPublicClient({ transport: http(rpcUrl) });
  const wallet = privateKeyToAccount(privateKey);
  return { provider, wallet, destinationAddress };
}

const trustedSetupPath = `${import.meta.dirname}/trusted_setup.txt`;

async function main() {
  const { provider, wallet, destinationAddress } = getClients();

  cKzg.loadTrustedSetup(0, trustedSetupPath);

  const parentSubmissionData1 = generateParentSubmissionData(
    "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd",
    zeroHash,
  );

  const kzgProofsArray: Uint8Array[] = [];
  const commitmentsArray: Uint8Array[] = [];
  const versionedHashesArray: Hex[] = [];
  const blobsArray: Uint8Array[] = [];
  const kzgProofsContractArray: string[] = [];
  // const parentSubmissionData: ParentSubmissionData[] = [];

  const blobSubmissionData: BlobSubmissionData[] = [];

  let previousSubmissionData = generateParentSubmissionData(
    "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd",
    zeroHash,
  );

  const finalSubmissionData = generateParentSubmissionDataFromJson(submissionDataJson6);

  for (let i = 0; i < dataItems.length; i++) {
    const { submissionData, blob } = generateSubmissionDataFromJSON(
      dataItems[i].conflationOrder.startingBlockNumber,
      dataItems[i].conflationOrder.upperBoundaries.slice(-1)[0],
      dataItems[i],
    );
    const fullblob = getPadded(blob);
    const commitment = cKzg.blobToKzgCommitment(fullblob);
    const kzgProof = cKzg.computeBlobKzgProof(fullblob, commitment);

    blobsArray.push(fullblob);
    versionedHashesArray.push(kzgCommitmentsToVersionedHashes([commitment])[0]);
    commitmentsArray.push(commitment);
    kzgProofsArray.push(kzgProof);
    kzgProofsContractArray.push(dataItems[i].kzgProofContract);

    blobSubmissionData.push({
      submissionData: submissionData,
      parentSubmissionData: previousSubmissionData,
      dataEvaluationClaim: BigInt(dataItems[i].expectedY),
      kzgCommitment: bytesToHex(commitment),
      kzgProof: toHex(dataItems[i].kzgProofContract, "kzgProofContract"),
    });

    previousSubmissionData = generateParentSubmissionDataFromJson(dataItems[i]);
  }

  const encodedCall = encodeCall(blobSubmissionData);

  await submitBlob(provider, wallet, encodedCall, destinationAddress, versionedHashesArray, blobsArray);

  await sendMessage();

  await sendProof(aggregateProof1to305, parentSubmissionData1, finalSubmissionData);
}

function mapToTuple(blobSubmissionDataItems: BlobSubmissionData[]): unknown[] {
  return blobSubmissionDataItems.map((blobSubmissionData) => [
    blobSubmissionData.dataEvaluationClaim,
    blobSubmissionData.kzgCommitment,
    blobSubmissionData.kzgProof,
    blobSubmissionData.submissionData.finalStateRootHash,
    blobSubmissionData.submissionData.snarkHash,
  ]);
}

function encodeCall(submissionData: BlobSubmissionData[]): Hex {
  //submitBlobs((uint256,bytes,bytes,bytes32,bytes32)[],bytes32,bytes32)": "99467a35"

  const encodedCall = concat([
    "0x99467a35",
    encodeData(
      ["tuple(uint256,bytes,bytes,bytes32,bytes32)[]", "bytes32", "bytes32"],
      [
        mapToTuple(submissionData),
        submissionData[0].parentSubmissionData.shnarf,
        dataItems[dataItems.length - 1].expectedShnarf,
      ],
    ),
  ]);

  return encodedCall;
}

async function sendMessage() {
  console.log("sending the message");

  const { provider, wallet, destinationAddress } = getClients();

  const encodedCall = concat([
    "0x9f3ce55a",
    encodeData(
      ["address", "uint256", "bytes"],
      ["0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC", 50000000000000000n, "0x"],
    ),
  ]);

  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.estimateFeesPerGas();
  const nonce = await provider.getTransactionCount({ address: wallet.address });

  const transaction = {
    type: "eip1559",
    data: encodedCall,
    maxPriorityFeePerGas,
    maxFeePerGas,
    to: destinationAddress,
    chainId,
    nonce,
    value: 1050000000000000000n,
    gas: 5_000_000n,
  } as const;

  const hash = await provider.sendRawTransaction({ serializedTransaction: await wallet.signTransaction(transaction) });
  const receipt = await provider.waitForTransactionReceipt({ hash });
  console.log({ transaction: { ...transaction, hash }, receipt });
}

async function submitBlob(
  provider: PublicClient,
  wallet: PrivateKeyAccount,
  encodedCall: Hex,
  destinationAddress: Hex,
  versionedHashes: Hex[],
  fullblobs: Uint8Array[],
) {
  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.estimateFeesPerGas();
  const nonce = await provider.getTransactionCount({ address: wallet.address });

  console.log(encodedCall);

  const serializedTransaction = await wallet.signTransaction({
    data: encodedCall,
    maxPriorityFeePerGas,
    maxFeePerGas,
    to: destinationAddress,
    chainId,
    type: "eip4844",
    nonce,
    value: 0n,
    kzg: defineKzg({
      blobToKzgCommitment: cKzg.blobToKzgCommitment,
      computeBlobKzgProof: cKzg.computeBlobKzgProof,
    }),
    blobs: fullblobs,
    gas: 5_000_000n,
    blobVersionedHashes: versionedHashes,
    maxFeePerBlobGas: maxFeePerGas,
  });

  const hash = await provider.sendRawTransaction({ serializedTransaction });
  const receipt = await provider.waitForTransactionReceipt({ hash });

  console.log(versionedHashes);
  console.log("BlobTX Hash: ", hash);
  console.log(
    `BlobTX receipt: ${JSON.stringify(receipt, (_key, value) => (typeof value === "bigint" ? value.toString() : value), 2)}`,
  );
}

async function sendProof(
  proofFile: AggregatedProofJson,
  submissionData: ParentSubmissionData,
  finalSubmissionData: ParentSubmissionData,
) {
  console.log("proof");

  const { provider, wallet, destinationAddress } = getClients();

  const finalSubmission = dataItems[dataItems.length - 1];

  const proofData = [
    proofFile.aggregatedProof,
    0,
    [
      proofFile.parentStateRootHash,
      finalSubmissionData.finalBlockInData,
      [
        finalSubmission.prevShnarf,
        finalSubmission.snarkHash,
        finalSubmission.finalStateRootHash,
        finalSubmission.expectedX,
        finalSubmission.expectedY,
      ],
      proofFile.parentAggregationLastBlockTimestamp,
      proofFile.finalTimestamp,
      "0x0000000000000000000000000000000000000000000000000000000000000000",
      proofFile.l1RollingHash,
      0, // last finalized message number
      proofFile.l1RollingHashMessageNumber,
      proofFile.l2MerkleTreesDepth,
      proofFile.l2MerkleRoots,
      proofFile.l2MessagingBlocksOffsets,
    ],
  ];

  console.log(proofData);
  //finalizeBlocks(bytes,uint256,(bytes32,uint256,(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,bytes32[],bytes))": "5603c65f"
  //finalizeBlocks(bytes,uint256,(bytes32,uint256,(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,bytes32[],bytes))": "5603c65f"
  const encodedCall = concat([
    "0x5603c65f",
    encodeData(
      [
        "bytes",
        "uint256",
        "tuple(bytes32,uint256,tuple(bytes32,bytes32,bytes32,bytes32,bytes32),uint256,uint256,bytes32,bytes32,uint256,uint256,uint256,bytes32[],bytes)",
      ],
      proofData,
    ),
  ]);

  console.log(submissionData.shnarf);
  console.log(finalSubmission.expectedShnarf);
  console.log(BigInt(finalSubmission.conflationOrder.upperBoundaries.slice(-1)[0]));

  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.estimateFeesPerGas();
  const nonce = await provider.getTransactionCount({ address: wallet.address });

  const transaction = {
    type: "eip1559",
    data: encodedCall,
    maxPriorityFeePerGas,
    maxFeePerGas,
    to: destinationAddress,
    chainId,
    nonce,
    value: 0n,
    gas: 5_000_000n,
  } as const;

  const hash = await provider.sendRawTransaction({ serializedTransaction: await wallet.signTransaction(transaction) });
  const receipt = await provider.waitForTransactionReceipt({ hash });
  console.log({ transaction: { ...transaction, hash }, receipt });
}

main()
  .then(() => process.exit(0))
  .catch((error) => {
    console.error(error);
    process.exit(1);
  });
