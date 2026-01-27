import { IProvider } from "../IProvider";

export type BlockExtraData = {
  version: number;
  fixedCost: number;
  variableCost: number;
  ethGasPrice: number;
};

export interface ILineaProvider<
  TransactionReceipt,
  Block,
  TransactionRequest,
  TransactionResponse,
  Provider,
> extends IProvider<TransactionReceipt, Block, TransactionRequest, TransactionResponse, Provider> {
  getBlockExtraData(blockNumber: number | bigint | string): Promise<BlockExtraData | null>;
}
