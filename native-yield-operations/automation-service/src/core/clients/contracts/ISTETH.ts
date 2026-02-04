import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface ISTETH extends IBaseContractClient {
  getPooledEthBySharesRoundUp(sharesAmount: bigint): Promise<bigint | undefined>;
}
