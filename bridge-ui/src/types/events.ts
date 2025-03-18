import { Address, Log } from "viem";

export type MessageSentEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    _from: Address;
    _to: Address;
    _fee: bigint;
    _value: bigint;
    _nonce: bigint;
    _calldata: string;
    _messageHash: string;
  };
};

export type BridgingInitiatedV2Event = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    sender: Address;
    recipient: Address;
    token: Address;
    amount: bigint;
  };
};

export type DepositForBurnEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    burnToken: Address;
    amount: bigint;
    depositor: Address;
    mintRecipient: `0x${string}`;
    destinationDomain: number;
    destinationTokenMessenger: `0x${string}`;
    destinationCaller: `0x${string}`;
    maxFee: bigint;
    minFinalityThreshold: number;
    hookData: `0x${string}`;
  };
};

export type CCTPMessageReceivedEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    caller: Address;
    sourceDomain: number;
    nonce: `0x${string}`;
    sender: `0x${string}`;
    finalityThresholdExecuted: number;
    messageBody: `0x${string}`;
  };
};
