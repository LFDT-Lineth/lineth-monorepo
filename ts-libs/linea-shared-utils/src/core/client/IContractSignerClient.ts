import { Address, Hex, TransactionSerializable } from "viem";

export interface IContractSignerClient {
  sign(tx: TransactionSerializable): Promise<Hex>;
  getAddress(): Address;
}
