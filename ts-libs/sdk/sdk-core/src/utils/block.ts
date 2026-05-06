import { hexToNumber, slice } from "./misc";
import { Hex } from "../types/misc";

export type ParseBlockExtraDataReturnType = {
  version: number;
  fixedCost: number;
  variableCost: number;
  ethGasPrice: number;
};

/**
 * Parses the extra data field of a block to extract version, fixed cost, variable cost, and ETH gas price.
 *
 * @param extraData - The extra data field from a block, expected to be in Hex format.
 * @returns An object containing the parsed values.
 */
export function parseBlockExtraData(extraData: Hex): ParseBlockExtraDataReturnType {
  const version = slice(extraData, 0, 1);
  const fixedCost = slice(extraData, 1, 5);
  const variableCost = slice(extraData, 5, 9);
  const ethGasPrice = slice(extraData, 9, 13);

  return {
    version: hexToNumber(version),
    fixedCost: hexToNumber(fixedCost) * 1000,
    variableCost: hexToNumber(variableCost) * 1000,
    ethGasPrice: hexToNumber(ethGasPrice) * 1000,
  };
}
