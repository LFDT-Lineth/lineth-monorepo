import { ethers } from "ethers";

export type Transaction = {
  hash: string;
  nonce: number;
  gas: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  gasPrice?: string;
  input?: string;
  value: string;
  chainId?: string;
  accessList?: ethers.AccessListish | null;
  type: string | number;
  to?: string;
};

export type Txpool = {
  pending: {
    [address: string]: {
      [nonce: string]: Transaction;
    };
  };
  queued: object;
};
