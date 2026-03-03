import { AbiEvent, Address, Log } from "viem";

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

export const BridgingInitiatedABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "sender", type: "address" },
    { indexed: false, internalType: "address", name: "recipient", type: "address" },
    { indexed: true, internalType: "address", name: "token", type: "address" },
    { indexed: true, internalType: "uint256", name: "amount", type: "uint256" },
  ],
  name: "BridgingInitiated",
  type: "event",
};

export const BridgingInitiatedV2ABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "sender", type: "address" },
    { indexed: true, internalType: "address", name: "recipient", type: "address" },
    { indexed: true, internalType: "address", name: "token", type: "address" },
    { indexed: false, internalType: "uint256", name: "amount", type: "uint256" },
  ],
  name: "BridgingInitiatedV2",
  type: "event",
};

export const MessageSentABIEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "_from", type: "address" },
    { indexed: true, internalType: "address", name: "_to", type: "address" },
    { indexed: false, internalType: "uint256", name: "_fee", type: "uint256" },
    { indexed: false, internalType: "uint256", name: "_value", type: "uint256" },
    { indexed: false, internalType: "uint256", name: "_nonce", type: "uint256" },
    { indexed: false, internalType: "bytes", name: "_calldata", type: "bytes" },
    { indexed: true, internalType: "bytes32", name: "_messageHash", type: "bytes32" },
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
