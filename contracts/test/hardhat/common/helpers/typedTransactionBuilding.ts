import { address, hex } from "./general.js";

import type { AccessListEntryInput, Eip1559Transaction } from "../types.js";

/** An EIP-1559 transaction as returned by JSON-RPC and stored in the test fixtures (quantities as hex strings). */
export type RpcEip1559Transaction = {
  nonce: string | number;
  maxPriorityFeePerGas: string | number;
  maxFeePerGas: string | number;
  gas: string | number;
  to: string;
  value: string | number;
  input: string;
  accessList: AccessListEntryInput[];
  yParity: string | number;
  r: string;
  s: string;
};

/** Converts a JSON-RPC EIP-1559 transaction to the ABI struct shape, with every field in its declared type. */
export function buildEip1559Transaction(data: RpcEip1559Transaction): Eip1559Transaction {
  return {
    nonce: BigInt(data.nonce),
    maxPriorityFeePerGas: BigInt(data.maxPriorityFeePerGas),
    maxFeePerGas: BigInt(data.maxFeePerGas),
    gasLimit: BigInt(data.gas),
    to: address(data.to),
    value: BigInt(data.value),
    input: hex(data.input),
    accessList: data.accessList.map((entry) => ({
      contractAddress: address(entry.address),
      storageKeys: entry.storageKeys.map(hex),
    })),
    yParity: Number(data.yParity),
    r: BigInt(data.r),
    s: BigInt(data.s),
  };
}
