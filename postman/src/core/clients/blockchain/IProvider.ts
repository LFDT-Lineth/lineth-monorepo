import { GasFees } from "./IGasProvider";

export interface IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, Provider> {
  getTransactionCount(address: string, blockTag: string | number | bigint): Promise<number>;
  getBlockNumber(): Promise<number>;
  getTransactionReceipt(txHash: string): Promise<TransactionReceipt | null>;
  getBlock(blockNumber: number | bigint | string): Promise<Block | null>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  send(methodName: string, params: Array<any> | Record<string, any>): Promise<any>;
  estimateGas(transactionRequest: TransactionRequest): Promise<bigint>;
  getTransaction(transactionHash: string): Promise<TransactionResponse | null>;
  broadcastTransaction(signedTx: string): Promise<TransactionResponse>;
  call(transactionRequest: TransactionRequest): Promise<string>;
  getFees(): Promise<GasFees>;
  get provider(): Provider;
}
