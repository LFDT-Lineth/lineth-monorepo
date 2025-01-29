import { ethers } from "ethers";
import { GasFees } from "../../core/clients/IGasProvider";
import { makeBaseError } from "../../core/errors/utils";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type Constructor<T = object> = new (...args: any[]) => T;

export function ProviderMixIn<TBase extends Constructor<ethers.Provider>>(Base: TBase) {
  return class Provider extends Base {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    constructor(...args: any[]) {
      super(...args);
    }

    /**
     * Retrieves the current gas fees.
     *
     * @returns {Promise<GasFees>} A promise that resolves to an object containing the current gas fees.
     * @throws {BaseError} If there is an error getting the fee data.
     */
    public async getFees(): Promise<GasFees> {
      const { maxPriorityFeePerGas, maxFeePerGas } = await this.getFeeData();

      if (!maxPriorityFeePerGas || !maxFeePerGas) {
        throw makeBaseError("Error getting fee data");
      }

      return { maxPriorityFeePerGas, maxFeePerGas };
    }
  };
}

export class Provider extends ProviderMixIn(ethers.JsonRpcProvider) {}
export class BrowserProvider extends ProviderMixIn(ethers.BrowserProvider) {}
