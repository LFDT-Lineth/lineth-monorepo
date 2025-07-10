import { BlockTag } from "../block";
import { Hex } from "../misc";

export type GetBlockExtraDataParameters<blockTag extends BlockTag = "latest"> = {
  blockHash?: Hex | undefined;
  blockNumber?: bigint | undefined;
  blockTag?: blockTag | BlockTag | undefined;
};

export type GetBlockExtraDataReturnType = {
  version: number;
  fixedCost: number;
  variableCost: number;
  ethGasPrice: number;
};
