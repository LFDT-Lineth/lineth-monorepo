import { IBaseContractClient } from "@lfdt-lineth/shared-utils";

export interface IStakingVault extends IBaseContractClient {
  beaconChainDepositsPaused(): Promise<boolean>;
}
