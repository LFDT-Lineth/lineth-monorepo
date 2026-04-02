import type { Block, Log, TransactionReceipt, TransactionSubmission } from "../../../../core/types";
import type { GetTransactionReceiptReturnType, GetTransactionReturnType } from "viem";

export function mapViemReceiptToCoreReceipt(receipt: GetTransactionReceiptReturnType): TransactionReceipt {
  return {
    hash: receipt.transactionHash,
    blockNumber: Number(receipt.blockNumber),
    status: receipt.status === "success" ? "success" : "reverted",
    gasUsed: receipt.gasUsed,
    gasPrice: receipt.effectiveGasPrice,
    logs: receipt.logs.map(
      (log): Log => ({
        address: log.address,
        topics: [...log.topics],
        data: log.data,
        blockNumber: Number(log.blockNumber),
        transactionHash: log.transactionHash ?? "0x",
        logIndex: log.logIndex ?? 0,
      }),
    ),
  };
}

export function mapViemBlockToCoreBlock(block: {
  number: bigint | null;
  timestamp: bigint;
  hash: `0x${string}` | null;
}): Block {
  return {
    number: Number(block.number ?? 0n),
    timestamp: Number(block.timestamp),
    hash: block.hash ?? "0x",
  };
}

export function mapViemTransactionToCoreSubmission(tx: GetTransactionReturnType): TransactionSubmission {
  return {
    hash: tx.hash,
    nonce: tx.nonce,
    gasLimit: tx.gas,
    maxFeePerGas: tx.maxFeePerGas ?? undefined,
    maxPriorityFeePerGas: tx.maxPriorityFeePerGas ?? undefined,
  };
}
