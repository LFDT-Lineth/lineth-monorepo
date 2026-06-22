import { type AccessList, type Address, type Hash, type Hex } from "viem";

export type Quantity = string | number | bigint;

export type Transaction = {
  hash: Hash;
  nonce: Quantity;
  gas: Quantity;
  maxFeePerGas?: Quantity;
  maxPriorityFeePerGas?: Quantity;
  gasPrice?: Quantity;
  input?: Hex;
  value: Quantity;
  chainId?: Quantity;
  accessList?: AccessList | null;
  r?: Hex;
  s?: Hex;
  v?: Quantity;
  yParity?: Quantity;
  type: string | number;
  to?: Address | null;
};

export type Txpool = {
  pending: {
    [address: string]: {
      [nonce: string]: Transaction;
    };
  };
  queued: object;
};
