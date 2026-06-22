import { IBaseContractClient } from "@lfdt-lineth/shared-utils";

export interface ILineaRollupYieldExtension<TransactionReceipt> extends IBaseContractClient {
  transferFundsForNativeYield(amount: bigint): Promise<TransactionReceipt>;
}
