import { Address } from "viem";
import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface IVaultHub<TTransactionReceipt> extends IBaseContractClient {
  getLiabilityPaymentFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
  getLidoFeePaymentFromTxReceipt(txReceipt: TTransactionReceipt): bigint;
  settleableLidoFeesValue(vault: Address): Promise<bigint | undefined>;
  getLatestVaultReportTimestamp(vault: Address): Promise<bigint>;
  isReportFresh(vault: Address): Promise<boolean>;
  isVaultConnected(vault: Address): Promise<boolean>;
}
