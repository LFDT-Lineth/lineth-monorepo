import { IBaseContractClient } from "@lfdt-lineth/shared-utils";

export interface ILinethRollupYieldExtension<TransactionReceipt> extends IBaseContractClient {
  transferFundsForNativeYield(amount: bigint): Promise<TransactionReceipt>;
}
