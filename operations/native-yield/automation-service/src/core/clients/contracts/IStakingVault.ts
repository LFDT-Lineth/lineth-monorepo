import { IBaseContractClient } from "@consensys/linea-shared-utils";

export interface IStakingVault extends IBaseContractClient {
  beaconChainDepositsPaused(): Promise<boolean>;
}
