import { Address } from "abitype";

import { Hex } from "./misc";

export type Log<TUnit = bigint> = {
  address: Address;
  topics: Hex[];
  data: Hex;
  blockNumber: TUnit;
  transactionHash: Hex | null;
  transactionIndex: number | null;
  blockHash: Hex;
  logIndex: number | null;
  removed: boolean;
};

export type TransactionReceipt<TUnit = bigint> = {
  blockHash: Hex;
  blockNumber: TUnit;
  contractAddress: Address | null | undefined;
  cumulativeGasUsed: TUnit;
  effectiveGasPrice: TUnit;
  from: Address;
  gasUsed: TUnit;
  logs: Log[];
  logsBloom: Hex;
  status: string;
  to: Address | null;
  transactionHash: Hex;
  transactionIndex: number;
  type: string;
};

type BaseTransactionRequest<TUnit = bigint, TType = string> = {
  from?: Hex;
  to?: Hex | null;
  nonce?: TUnit;
  data?: Hex;
  value?: TUnit;
  gas?: TUnit;
  type: TType;
};

export type AccessList = {
  address: string;
  storageKeys: string[];
}[];

export type AuthorizationList<TUnit = bigint, IncludeProof extends boolean = boolean> = {
  ephemeralAddress: string;
  proof?: IncludeProof extends true ? string : undefined;
  validity?: { start?: TUnit; end?: TUnit };
}[];

/**
 * Legacy transaction (pre-EIP-1559, pre-EIP-2930)
 */
export type LegacyTransactionRequest<TUnit = bigint, TType = string> = BaseTransactionRequest<TUnit, TType> & {
  gasPrice?: TUnit;
};

/**
 * EIP-2930 transaction (adds access list)
 */
export type EIP2930TransactionRequest<TUnit = bigint, TType = string> = BaseTransactionRequest<TUnit, TType> & {
  gasPrice?: TUnit;
  accessList: AccessList;
  chainId: TUnit;
};

/**
 * EIP-1559 transaction (adds maxFeePerGas and maxPriorityFeePerGas)
 */
export type EIP1559TransactionRequest<TUnit = bigint, TType = string> = BaseTransactionRequest<TUnit, TType> & {
  maxFeePerGas: TUnit;
  maxPriorityFeePerGas: TUnit;
  accessList?: AccessList;
  chainId: TUnit;
};

/**
 * EIP-4844 transaction (blob-carrying transaction)
 */
export type EIP4844TransactionRequest<TUnit = bigint, TType = string> = EIP1559TransactionRequest<TUnit, TType> & {
  maxFeePerBlobGas: TUnit;
  blobVersionedHashes: Hex[];
};

/**
 * EIP-7702 transaction (ephemeral authorization list)
 */
export type EIP7702TransactionRequest<TUnit = bigint, TType = string> = BaseTransactionRequest<TUnit, TType> & {
  authorizationList: AuthorizationList<TUnit, boolean>;
  chainId: TUnit;
};

export type TransactionRequest<TUnit = bigint, TType = string> =
  | LegacyTransactionRequest<TUnit, TType>
  | EIP2930TransactionRequest<TUnit, TType>
  | EIP1559TransactionRequest<TUnit, TType>
  | EIP4844TransactionRequest<TUnit, TType>
  | EIP7702TransactionRequest<TUnit, TType>;
