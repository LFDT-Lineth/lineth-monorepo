import { AbiEvent, Address, Log } from "viem";

export type MTokenReceivedLogEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    sourceChainId: bigint;
    destinationToken: Address;
    sender: Address;
    recipient: Address;
    amount: bigint;
    index: bigint;
  };
};

export const MTokenReceivedAbiEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: false, internalType: "uint256", name: "sourceChainId", type: "uint256" },
    { indexed: true, internalType: "address", name: "destinationToken", type: "address" },
    { indexed: true, internalType: "address", name: "sender", type: "address" },
    { indexed: true, internalType: "address", name: "recipient", type: "address" },
    { indexed: false, internalType: "uint256", name: "amount", type: "uint256" },
    { indexed: false, internalType: "uint128", name: "index", type: "uint128" },
  ],
  name: "MTokenReceived",
  type: "event",
};

export type MTokenSentLogEvent = Log & {
  blockNumber: bigint;
  transactionHash: Address;
  args: {
    sourceToken: Address;
    destinationChainId: bigint;
    destinationToken: Address;
    sender: Address;
    recipient: Address;
    amount: bigint;
    index: bigint;
    messageId: `0x${string}`;
  };
};

export const MTokenSentAbiEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "sourceToken", type: "address" },
    { indexed: false, internalType: "uint256", name: "destinationChainId", type: "uint256" },
    { indexed: false, internalType: "address", name: "destinationToken", type: "address" },
    { indexed: true, internalType: "address", name: "sender", type: "address" },
    { indexed: true, internalType: "address", name: "recipient", type: "address" },
    { indexed: false, internalType: "uint256", name: "amount", type: "uint256" },
    { indexed: false, internalType: "uint128", name: "index", type: "uint128" },
    { indexed: false, internalType: "bytes32", name: "messageId", type: "bytes32" },
  ],
  name: "MTokenSent",
  type: "event",
};
