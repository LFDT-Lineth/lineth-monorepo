import { TransactionRequest } from "../../types";

export type GasFees = {
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
};

export type LineaGasFees = GasFees & {
  gasLimit: bigint;
};

type BaseGasProviderConfig = {
  maxFeePerGasCap: bigint;
  enforceMaxGasFee: boolean;
};

export type DefaultGasProviderConfig = BaseGasProviderConfig & {
  gasEstimationPercentile: number;
};

export type LineaGasProviderConfig = BaseGasProviderConfig & {
  enableLineaEstimateGas?: boolean;
  gasEstimationPercentile?: number;
};

export interface IGasProvider {
  getGasFees(transactionRequest?: TransactionRequest): Promise<GasFees | LineaGasFees>;
  getMaxFeePerGas(): bigint;
}

export interface IEthereumGasProvider extends IGasProvider {
  getGasFees(): Promise<GasFees>;
}

export interface ILineaGasProvider extends IGasProvider {
  getGasFees(transactionRequest: TransactionRequest): Promise<LineaGasFees>;
}
