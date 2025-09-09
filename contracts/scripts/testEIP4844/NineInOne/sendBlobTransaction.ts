import { AbiCoder, BytesLike, Transaction, Wallet, ethers } from "ethers";
import { commitmentsToVersionedHashes } from "@ethereumjs/util";
import * as kzg from "c-kzg";
import submissionDataJson1 from "./blocks-1-46.json";
import submissionDataJson2 from "./blocks-47-81.json";
import submissionDataJson3 from "./blocks-82-114.json";
import submissionDataJson4 from "./blocks-115-155.json";
import submissionDataJson5 from "./blocks-156-175.json";
import submissionDataJson6 from "./blocks-176-206.json";
import submissionDataJson7 from "./blocks-207-228.json";
import submissionDataJson8 from "./blocks-229-265.json";
import submissionDataJson9 from "./blocks-266-305.json";
import aggregateProof1to305 from "./aggregatedProof-1-305.json";
import { DataHexString } from "ethers/lib.commonjs/utils/data";

const chainId = 31648428;

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
  return ethers.keccak256(encodeData(types, values, packed));
}

export const encodeData = (types: string[], values: unknown[], packed?: boolean) => {
  if (packed) {
    return ethers.solidityPacked(types, values);
  }
  return AbiCoder.defaultAbiCoder().encode(types, values);
};

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
  kzgCommitment: BytesLike;
  kzgProof: BytesLike;
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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  parsedJSONData: any,
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
    blob: ethers.decodeBase64(parsedJSONData.compressedData),
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

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function generateParentSubmissionDataFromJson(parsedJSONData: any): ParentSubmissionData {
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
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  parsedJSONData: any,
): { submissionData: SubmissionData } {
  const returnData = {
    parentStateRootHash: parsedJSONData.parentStateRootHash,
    dataParentHash: parsedJSONData.parentDataHash,
    finalStateRootHash: parsedJSONData.finalStateRootHash,
    firstBlockInData: BigInt(startingBlockNumber),
    finalBlockInData: BigInt(endingBlockNumber),
    snarkHash: parsedJSONData.snarkHash,
    compressedData: ethers.hexlify(ethers.decodeBase64(parsedJSONData.compressedData)),
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

function kzgCommitmentsToVersionedHashes(commitments: Uint8Array[]): string[] {
  const versionedHashes = commitmentsToVersionedHashes(commitments);
  return versionedHashes.map((versionedHash) => ethers.hexlify(versionedHash));
}

async function main() {
  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("PRIVATE_KEY");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);

  kzg.loadTrustedSetup(`${__dirname}/trusted_setup.txt`);

  const parentSubmissionData1 = generateParentSubmissionData(
    "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd",
    ethers.ZeroHash,
  );

  const kzgProofsArray: Uint8Array[] = [];
  const commitmentsArray: Uint8Array[] = [];
  const versionedHashesArray: string[] = [];
  const blobsArray: Uint8Array[] = [];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const kzgProofsContractArray: any[] = [];
  // const parentSubmissionData: ParentSubmissionData[] = [];

  const blobSubmissionData: BlobSubmissionData[] = [];

  let previousSubmissionData = generateParentSubmissionData(
    "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd",
    ethers.ZeroHash,
  );

  const finalSubmissionData = generateParentSubmissionDataFromJson(submissionDataJson6);

  for (let i = 0; i < dataItems.length; i++) {
    const { submissionData, blob } = generateSubmissionDataFromJSON(
      dataItems[i].conflationOrder.startingBlockNumber,
      dataItems[i].conflationOrder.upperBoundaries.slice(-1)[0],
      dataItems[i],
    );
    const fullblob = getPadded(blob);
    const commitment = kzg.blobToKzgCommitment(fullblob);
    const kzgProof = kzg.computeBlobKzgProof(fullblob, commitment);

    blobsArray.push(fullblob);
    versionedHashesArray.push(kzgCommitmentsToVersionedHashes([commitment])[0]);
    commitmentsArray.push(commitment);
    kzgProofsArray.push(kzgProof);
    kzgProofsContractArray.push(dataItems[i].kzgProofContract);

    blobSubmissionData.push({
      submissionData: submissionData,
      parentSubmissionData: previousSubmissionData,
      dataEvaluationClaim: BigInt(dataItems[i].expectedY),
      kzgCommitment: commitment,
      kzgProof: dataItems[i].kzgProofContract,
    });

    previousSubmissionData = generateParentSubmissionDataFromJson(dataItems[i]);
  }

  const encodedCall = encodeCall(blobSubmissionData);

  await submitBlob(provider, wallet, encodedCall, destinationAddress, versionedHashesArray, blobsArray);

  await sendMessage();

  await sendProof(aggregateProof1to305, parentSubmissionData1, finalSubmissionData);
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function mapToTuple(blobSubmissionDataItems: BlobSubmissionData[]): any {
  return blobSubmissionDataItems.map((blobSubmissionData) => [
    blobSubmissionData.dataEvaluationClaim,
    blobSubmissionData.kzgCommitment,
    blobSubmissionData.kzgProof,
    blobSubmissionData.submissionData.finalStateRootHash,
    blobSubmissionData.submissionData.snarkHash,
  ]);
}

function encodeCall(submissionData: BlobSubmissionData[]): DataHexString {
  //submitBlobs((uint256,bytes,bytes,bytes32,bytes32)[],bytes32,bytes32)": "99467a35"

  const encodedCall = ethers.concat([
    "0x99467a35",
    ethers.AbiCoder.defaultAbiCoder().encode(
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

  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("PRIVATE_KEY");
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

async function submitBlob(
  provider: ethers.JsonRpcProvider,
  wallet: Wallet,
  encodedCall: string,
  destinationAddress: string,
  versionedHashes: string[],
  fullblobs: Uint8Array[],
) {
  const { maxFeePerGas, maxPriorityFeePerGas } = await provider.getFeeData();
  const nonce = await provider.getTransactionCount(wallet.address);

  console.log(encodedCall);

  const transaction = Transaction.from({
    data: encodedCall,
    maxPriorityFeePerGas: maxPriorityFeePerGas!,
    maxFeePerGas: maxFeePerGas!,
    to: destinationAddress,
    chainId,
    type: 3,
    nonce,
    value: 0,
    kzg,
    blobs: fullblobs,
    gasLimit: 5_000_000,
    blobVersionedHashes: versionedHashes,
    maxFeePerBlobGas: maxFeePerGas!,
  });

  const tx = await wallet.sendTransaction(transaction);
  const receipt = await tx.wait();

  console.log(versionedHashes);
  console.log("BlobTX Hash: ", tx.hash);
  console.log(`BlobTX receipt: ${JSON.stringify(receipt, null, 2)}`);
}

async function sendProof(
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  proofFile: any,
  submissionData: ParentSubmissionData,
  finalSubmissionData: ParentSubmissionData,
) {
  console.log("proof");

  const rpcUrl = requireEnv("RPC_URL");
  const privateKey = requireEnv("PRIVATE_KEY");
  const destinationAddress = requireEnv("DESTINATION_ADDRESS");

  const provider = new ethers.JsonRpcProvider(rpcUrl);
  const wallet = new Wallet(privateKey, provider);

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
  const encodedCall = ethers.concat([
    "0x5603c65f",
    ethers.AbiCoder.defaultAbiCoder().encode(
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
