import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface ILineaRollupYieldExtension<TransactionReceipt> extends IBaseContractClient {
  transferFundsForNativeYield(amount: bigint): Promise<TransactionReceipt>;
}
