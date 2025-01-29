import fs from "fs";
import path from "path";
import { RawBlockData, FormattedBlockData, DebugData } from "../types";

export function getProverTestData(
  folder: string,
  filename: string,
): {
  blocks: FormattedBlockData[];
  proverMode: string;
  parentStateRootHash: string;
  version: string;
  firstBlockNumber: number;
  proof: string;
  debugData: DebugData;
} {
  const testFilePath = path.resolve(__dirname, "..", "testData", folder, filename);
  const testData = JSON.parse(fs.readFileSync(testFilePath, "utf8"));

  return {
    blocks: testData.blocksData.map((block: RawBlockData) => ({
      blockRootHash: block.rootHash,
      transactions: block.rlpEncodedTransactions,
      l2BlockTimestamp: block.timestamp,
      l2ToL1MsgHashes: block.l2ToL1MsgHashes,
      fromAddresses: block.fromAddresses,
      batchReceptionIndices: block.batchReceptionIndices,
    })),
    proverMode: testData.proverMode,
    parentStateRootHash: testData.parentStateRootHash,
    version: testData.version,
    firstBlockNumber: testData.firstBlockNumber,
    proof: testData.proof,
    debugData: testData.DebugData,
  };
}

export function getRLPEncodeTransactions(filename: string): {
  shortEip1559Transaction: string;
  eip1559Transaction: string;
  eip1559TransactionHashes: string[];
  legacyTransaction: string;
  legacyTransactionHashes: string[];
  eip2930Transaction: string;
  eip2930TransactionHashes: string[];
} {
  const testFilePath = path.resolve(__dirname, "..", "testData", filename);
  const testData = JSON.parse(fs.readFileSync(testFilePath, "utf8"));

  return {
    shortEip1559Transaction: testData.shortEip1559Transaction,
    eip1559Transaction: testData.eip1559Transaction,
    eip1559TransactionHashes: testData.eip1559TransactionHashes,
    legacyTransaction: testData.legacyTransaction,
    legacyTransactionHashes: testData.legacyTransactionHashes,
    eip2930Transaction: testData.eip2930Transaction,
    eip2930TransactionHashes: testData.eip2930TransactionHashes,
  };
}

export function getTransactionsToBeDecoded(blocks: FormattedBlockData[]): string[] {
  const txsToBeDecoded = [];
  for (let i = 0; i < blocks.length; i++) {
    for (let j = 0; j < blocks[i].batchReceptionIndices.length; j++) {
      const txIndex = blocks[i].batchReceptionIndices[j];
      txsToBeDecoded.push(blocks[i].transactions[txIndex]);
    }
  }
  return txsToBeDecoded;
}
