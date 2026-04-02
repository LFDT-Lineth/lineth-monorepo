import { IProvider } from "../IProvider";

export type BlockExtraData = {
  version: number;
  fixedCost: number;
  variableCost: number;
  ethGasPrice: number;
};

export interface ILineaProvider extends IProvider {
  getBlockExtraData(blockNumber: number | bigint | string): Promise<BlockExtraData | null>;
}
