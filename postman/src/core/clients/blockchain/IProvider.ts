import { GasFees } from "./IGasProvider";
import { Address, Hash, Hex, Block, TransactionReceipt, TransactionRequest, TransactionSubmission } from "../../types";

export interface ITransactionCountProvider {
  getTransactionCount(address: Address, blockTag: string | number | bigint): Promise<number>;
}

export interface IBlockProvider {
  getBlockNumber(): Promise<number>;
  getBlock(blockNumber: number | bigint | string): Promise<Block | null>;
}

export interface ITransactionProvider {
  getTransactionReceipt(txHash: Hash): Promise<TransactionReceipt | null>;
  getTransaction(transactionHash: Hash): Promise<TransactionSubmission | null>;
}

export interface IGasEstimator {
  estimateGas(transactionRequest: TransactionRequest): Promise<bigint>;
  call(transactionRequest: TransactionRequest): Promise<Hex>;
  getFees(): Promise<GasFees>;
}

export interface IProvider extends ITransactionCountProvider, IBlockProvider, ITransactionProvider, IGasEstimator {}
