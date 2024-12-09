import { BlockTag, dataSlice, ethers, toNumber } from "ethers";
import { BlockExtraData } from "../../core/clients/linea";
import { GasFees } from "../../core/clients/IGasProvider";
import { BaseError } from "../../core/errors";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Constructor<T = object> = new (...args: any[]) => T;

function LineaProviderMixIn<TBase extends Constructor<ethers.Provider>>(Base: TBase) {
  return class extends Base {
    public blockExtraDataCache: BlockExtraData;
    public cacheIsValidForBlockNumber: bigint;

    /**
     * Retrieves the current gas fees.
     *
     * @returns {Promise<GasFees>} A promise that resolves to an object containing the current gas fees.
     * @throws {BaseError} If there is an error getting the fee data.
     */
    public async getFees(): Promise<GasFees> {
      const { maxPriorityFeePerGas, maxFeePerGas } = await this.getFeeData();

      if (!maxPriorityFeePerGas || !maxFeePerGas) {
        throw new BaseError("Error getting fee data");
      }

      return { maxPriorityFeePerGas, maxFeePerGas };
    }

    /**
     * Fetches and formats extra data from a block.
     *
     * @param {BlockTag} blockNumber - The block number or tag to fetch extra data for.
     * @returns {Promise<BlockExtraData | null>} A promise that resolves to an object containing the formatted block's extra data, or null if the block is not found.
     */
    public async getBlockExtraData(blockNumber: BlockTag): Promise<BlockExtraData | null> {
      if (typeof blockNumber === "number" && this.isCacheValid(blockNumber)) {
        return this.blockExtraDataCache;
      }

      const block = await this.getBlock(blockNumber);

      if (!block) {
        return null;
      }

      const version = dataSlice(block.extraData, 0, 1);
      const fixedCost = dataSlice(block.extraData, 1, 5);
      const variableCost = dataSlice(block.extraData, 5, 9);
      const ethGasPrice = dataSlice(block.extraData, 9, 13);

      // Original values are in Kwei; convert them back to wei
      const extraData = {
        version: toNumber(version),
        fixedCost: toNumber(fixedCost) * 1000,
        variableCost: toNumber(variableCost) * 1000,
        ethGasPrice: toNumber(ethGasPrice) * 1000,
      };

      if (typeof blockNumber === "number") {
        this.cacheIsValidForBlockNumber = BigInt(blockNumber);
        this.blockExtraDataCache = extraData;
      }
      return extraData;
    }

    /**
     * Checks if the cached block extra data is still valid based on the current block number.
     *
     * @private
     * @param {number} currentBlockNumber - The current block number.
     * @returns {boolean} True if the cache is valid, false otherwise.
     */
    public isCacheValid(currentBlockNumber: number): boolean {
      return this.cacheIsValidForBlockNumber >= BigInt(currentBlockNumber);
    }
  };
}

export class LineaProvider extends LineaProviderMixIn(ethers.JsonRpcProvider) {}

export class LineaBrowserProvider extends LineaProviderMixIn(ethers.BrowserProvider) {}
