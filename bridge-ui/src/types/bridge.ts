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

export type HyperlaneBridgeMessage = {
  messageId: `0x${string}`;
  amountSent: bigint;
  transferIndex: bigint;
  sender: `0x${string}`;
  recipient: `0x${string}`;
};

export type BridgeMessage = NativeBridgeMessage | CctpV2BridgeMessage | HyperlaneBridgeMessage;

export type AdapterModeId = string;

export enum ClaimType {
  /** Destination-chain claim is sponsored (no extra fee). Used by the native bridge for L1→L2 when gas is below the Postman threshold. */
  AUTO_SPONSORED = "AUTO_SPONSORED",
  /** Destination-chain claim fee is paid upfront by the sender. Used by the native bridge for expensive L1→L2 claims and by Hyperlane for all directions. */
  AUTO_PAID = "AUTO_PAID",
  /** User must manually claim on the destination chain. Required for native bridge L2→L1 withdrawals. */
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
