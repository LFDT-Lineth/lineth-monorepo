import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface IDashboard<TTransactionReceipt> extends IBaseContractClient {
  getNodeOperatorFeesPaidFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
  withdrawableValue(): Promise<bigint>;
  totalValue(): Promise<bigint>;
  liabilityShares(): Promise<bigint>;
}
