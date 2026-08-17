import { IBaseContractClient } from "@lfdt-lineth/shared-utils";

export interface ISTETH extends IBaseContractClient {
  getPooledEthBySharesRoundUp(sharesAmount: bigint): Promise<bigint | undefined>;
}
