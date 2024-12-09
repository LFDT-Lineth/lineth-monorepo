import { Block, TransactionReceipt, TransactionRequest, TransactionResponse } from "ethers";
import { DefaultGasProvider } from "./DefaultGasProvider";
import { LineaGasProvider } from "./LineaGasProvider";
import { IProvider } from "../../core/clients/IProvider";
import { GasFees, GasProviderConfig, IGasProvider, LineaGasFees } from "../../core/clients/IGasProvider";
import { Direction } from "../../core/enums";
import { BaseError } from "../../core/errors";
import { BrowserProvider, LineaBrowserProvider, LineaProvider, Provider } from "../providers";

export class GasProvider implements IGasProvider<TransactionRequest> {
  private defaultGasProvider: DefaultGasProvider;
  private lineaGasProvider: LineaGasProvider;

  /**
   * Creates an instance of `GasProvider`.
   *
   * @param {IProvider} provider - The provider for interacting with the blockchain.
   * @param {GasProviderConfig} config - The configuration for the gas provider.
   */
  constructor(
    private provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      Provider | LineaProvider | LineaBrowserProvider | BrowserProvider
    >,
    private readonly config: GasProviderConfig,
  ) {
    this.defaultGasProvider = new DefaultGasProvider(this.provider, {
      maxFeePerGasCap: config.maxFeePerGasCap,
      gasEstimationPercentile: config.gasEstimationPercentile,
      enforceMaxGasFee: config.enforceMaxGasFee,
    });
    this.lineaGasProvider = new LineaGasProvider(this.provider, {
      maxFeePerGasCap: config.maxFeePerGasCap,
      enforceMaxGasFee: config.enforceMaxGasFee,
    });
  }

  /**
   * Fetches gas fee estimates.
   * Determines which gas provider to use based on the presence of transactionRequest.
   *
   * @param {TransactionRequest} [transactionRequest] - Optional transaction request to determine specific gas provider.
   * @returns {Promise<GasFees | LineaGasFees>} A promise that resolves to an object containing gas fee estimates.
   * @throws {BaseError} If transactionRequest is not provided when required.
   */
  public async getGasFees(transactionRequest?: TransactionRequest): Promise<GasFees | LineaGasFees> {
    if (this.config.direction === Direction.L1_TO_L2) {
      if (this.config.enableLineaEstimateGas) {
        if (!transactionRequest) {
          throw new BaseError(
            "You need to provide transaction request as param to call the getGasFees function on Linea.",
          );
        }
        return this.lineaGasProvider.getGasFees(transactionRequest);
      } else {
        const fees = await this.defaultGasProvider.getGasFees();
        const gasLimit = await this.provider.estimateGas({
          ...transactionRequest,
          maxPriorityFeePerGas: fees.maxPriorityFeePerGas,
          maxFeePerGas: fees.maxFeePerGas,
        });
        return {
          ...fees,
          gasLimit,
        };
      }
    } else {
      return this.defaultGasProvider.getGasFees();
    }
  }

  /**
   * Gets the maximum fee per gas as configured.
   *
   * @returns {bigint} The maximum fee per gas.
   */
  public getMaxFeePerGas(): bigint {
    if (this.config.direction === Direction.L1_TO_L2) {
      return this.lineaGasProvider.getMaxFeePerGas();
    }
    return this.defaultGasProvider.getMaxFeePerGas();
  }
}
