import { Address, GetContractReturnType } from "viem";

export interface IBaseContractClient {
  getAddress(): Address;
  getContract(): GetContractReturnType;
  getBalance(): Promise<bigint>;
}
