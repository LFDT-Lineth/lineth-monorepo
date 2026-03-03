import { MessageProof } from "@consensys/linea-sdk-viem";
import { Address } from "viem";

import { Chain, Token, TransactionStatus } from "@/types";

export type NativeBridgeMessage = {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: string;
  messageHash: string;
  proof?: MessageProof;
  amountSent: bigint;
};

// Params expected for `receiveMessage` as per https://developers.circle.com/stablecoins/transfer-usdc-on-testnet-from-ethereum-to-avalanche
export type CctpV2BridgeMessage = {
  message?: string;
  attestation?: string;
  amountSent: bigint;
  nonce: `0x${string}`;
};

export type BridgeMessage = NativeBridgeMessage | CctpV2BridgeMessage;

export type AdapterModeId = string;

export enum ClaimType {
  // Only for L1 -> L2, sponsored by the Postman
  AUTO_SPONSORED = "AUTO_SPONSORED",
  // Only for L1 -> L2, practically this will only be available when the L2 token contract does not exist (costing ~460K gas to claimMessage on L2).
  AUTO_PAID = "AUTO_PAID",
  // L2 -> L1 must be MANUAL
  MANUAL = "MANUAL",
}

// BridgeTransaction object that is populated when user opens "TransactionHistory" component, and is passed to child components.
export interface BridgeTransaction {
  adapterId: string;
  status: TransactionStatus;
  timestamp: bigint;
  fromChain: Chain;
  toChain: Chain;
  token: Token;
  message: BridgeMessage;
  bridgingTx: string;
  claimingTx?: string;
  mode?: AdapterModeId;
}
