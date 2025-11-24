import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface IVaultHub<TTransactionReceipt> extends IBaseContractClient {
  getLiabilityPaymentFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
  getLidoFeePaymentFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
}
