import { Address, Log, AbiEvent } from "viem";

export type MessageSentLogEvent = Log & {
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

export type BridgingInitiatedV2LogEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    sender: Address;
    recipient: Address;
    token: Address;
    amount: bigint;
  };
};

export type DepositForBurnLogEvent = Log & {
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

export const CctpMessageReceivedAbiEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "caller", type: "address" },
    { indexed: false, internalType: "uint32", name: "sourceDomain", type: "uint32" },
    { indexed: true, internalType: "bytes32", name: "nonce", type: "bytes32" },
    { indexed: false, internalType: "bytes32", name: "sender", type: "bytes32" },
    { indexed: true, internalType: "uint32", name: "finalityThresholdExecuted", type: "uint32" },
    { indexed: false, internalType: "bytes", name: "messageBody", type: "bytes" },
  ],
  name: "MessageReceived",
  type: "event",
};

export const BridgingInitiatedABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "sender",
      type: "address",
    },
    {
      indexed: false,
      internalType: "address",
      name: "recipient",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "token",
      type: "address",
    },
    {
      indexed: true,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
  ],
  name: "BridgingInitiated",
  type: "event",
};

export const BridgingInitiatedV2ABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "sender",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "recipient",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "token",
      type: "address",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
  ],
  name: "BridgingInitiatedV2",
  type: "event",
};

export const MessageSentABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "_from",
      type: "address",
    },
    {
      indexed: true,
      internalType: "address",
      name: "_to",
      type: "address",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_fee",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_value",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "_nonce",
      type: "uint256",
    },
    {
      indexed: false,
      internalType: "bytes",
      name: "_calldata",
      type: "bytes",
    },
    {
      indexed: true,
      internalType: "bytes32",
      name: "_messageHash",
      type: "bytes32",
    },
  ],
  name: "MessageSent",
  type: "event",
};

export const MessageClaimedABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [{ indexed: true, internalType: "bytes32", name: "_messageHash", type: "bytes32" }],
  name: "MessageClaimed",
  type: "event",
};

export const CctpDepositForBurnAbiEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    {
      indexed: true,
      internalType: "address",
      name: "burnToken",
      type: "address",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "amount",
      type: "uint256",
    },
    {
      indexed: true,
      internalType: "address",
      name: "depositor",
      type: "address",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "mintRecipient",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "uint32",
      name: "destinationDomain",
      type: "uint32",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "destinationTokenMessenger",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "bytes32",
      name: "destinationCaller",
      type: "bytes32",
    },
    {
      indexed: false,
      internalType: "uint256",
      name: "maxFee",
      type: "uint256",
    },
    {
      indexed: true,
      internalType: "uint32",
      name: "minFinalityThreshold",
      type: "uint32",
    },
    {
      indexed: false,
      internalType: "bytes",
      name: "hookData",
      type: "bytes",
    },
  ],
  name: "DepositForBurn",
  type: "event",
};
