import { HASH_ZERO } from "./general";

import firstCompressedDataContent from "../../_testData/compressedData/blocks-1-46.json";
import fourthCompressedDataContent from "../../_testData/compressedData/blocks-115-155.json";
import secondCompressedDataContent from "../../_testData/compressedData/blocks-47-81.json";
import thirdCompressedDataContent from "../../_testData/compressedData/blocks-82-114.json";
import fourthBlobDataContent from "../../_testData/compressedDataEip4844/blocks-115-155.json";
import thirdBlobDataContent from "../../_testData/compressedDataEip4844/blocks-82-114.json";
import secondBlobDataContent from "../../_testData/compressedDataEip4844/blocks-47-81.json";
import firstBlobDataContent from "../../_testData/compressedDataEip4844/blocks-1-46.json";

import firstCompressedDataContent_multiple from "../../_testData/compressedData/multipleProofs/blocks-1-46.json";
import secondCompressedDataContent_multiple from "../../_testData/compressedData/multipleProofs/blocks-47-81.json";
import thirdCompressedDataContent_multiple from "../../_testData/compressedData/multipleProofs/blocks-82-119.json";
import fourthCompressedDataContent_multiple from "../../_testData/compressedData/multipleProofs/blocks-120-153.json";

import firstBlobContent_multiple from "../../_testData/compressedDataEip4844/multipleProofs/blocks-1-46.json";
import secondBlobContent_multiple from "../../_testData/compressedDataEip4844/multipleProofs/blocks-47-81.json";
import thirdBlobContent_multiple from "../../_testData/compressedDataEip4844/multipleProofs/blocks-82-119.json";
import fourthBlobContent_multiple from "../../_testData/compressedDataEip4844/multipleProofs/blocks-120-153.json";

export const DEFAULT_SUBMISSION_DATA = {
  dataParentHash: HASH_ZERO,
  compressedData: "0x",
  finalBlockInData: 0n,
  firstBlockInData: 0n,
  parentStateRootHash: HASH_ZERO,
  finalStateRootHash: HASH_ZERO,
  snarkHash: HASH_ZERO,
};

export const COMPRESSED_SUBMISSION_DATA = [
  firstCompressedDataContent,
  secondCompressedDataContent,
  thirdCompressedDataContent,
  fourthCompressedDataContent,
];

export const BLOB_SUBMISSION_DATA = [
  firstBlobDataContent,
  secondBlobDataContent,
  thirdBlobDataContent,
  fourthBlobDataContent,
];

export const COMPRESSED_SUBMISSION_DATA_MULTIPLE_PROOF = [
  firstCompressedDataContent_multiple,
  secondCompressedDataContent_multiple,
  thirdCompressedDataContent_multiple,
  fourthCompressedDataContent_multiple,
];

export const BLOB_SUBMISSION_DATA_MULTIPLE_PROOF = [
  firstBlobContent_multiple,
  secondBlobContent_multiple,
  thirdBlobContent_multiple,
  fourthBlobContent_multiple,
];
