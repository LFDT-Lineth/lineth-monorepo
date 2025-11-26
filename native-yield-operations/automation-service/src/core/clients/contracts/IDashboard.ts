import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface IDashboard<TTransactionReceipt> extends IBaseContractClient {
  getNodeOperatorFeesPaidFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
  peekUnpaidLidoProtocolFees(): Promise<bigint | undefined>;
}

