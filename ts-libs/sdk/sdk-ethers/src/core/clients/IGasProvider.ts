import { Direction } from "../enums";

export type GasFees = {
  maxFeePerGas: bigint;
  maxPriorityFeePerGas: bigint;
};

export type LineaGasFees = GasFees & {
  gasLimit: bigint;
};

export type FeeHistory = {
  oldestBlock: number;
  reward: string[][];
  baseFeePerGas: string[];
  gasUsedRatio: number[];
};

export type LineaEstimateGasResponse = {
  baseFeePerGas: string;
  priorityFeePerGas: string;
  gasLimit: string;
};

type BaseGasProviderConfig = {
  maxFeePerGasCap: bigint;
  enforceMaxGasFee: boolean;
};

export type DefaultGasProviderConfig = BaseGasProviderConfig & {
  gasEstimationPercentile: number;
};

export type LineaGasProviderConfig = BaseGasProviderConfig;

export type GasProviderConfig = DefaultGasProviderConfig & {
  direction: Direction;
  enableLineaEstimateGas: boolean;
};

export interface IGasProvider<TransactionRequest> {
  getGasFees(transactionRequest?: TransactionRequest): Promise<GasFees | LineaGasFees>;
  getMaxFeePerGas(): bigint;
}

export interface IEthereumGasProvider<TransactionRequest> extends IGasProvider<TransactionRequest> {
  getGasFees(): Promise<GasFees>;
}

export interface ILineaGasProvider<TransactionRequest> extends IGasProvider<TransactionRequest> {
  getGasFees(transactionRequest: TransactionRequest): Promise<LineaGasFees>;
}

export function isLineaGasFees(fees: GasFees | LineaGasFees): fees is LineaGasFees {
  return "gasLimit" in fees;
}
