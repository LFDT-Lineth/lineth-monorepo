import { AbiEvent, Address, Log } from "viem";

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

export const CctpDepositForBurnAbiEvent: AbiEvent = {
  anonymous: false,
  inputs: [
    { indexed: true, internalType: "address", name: "burnToken", type: "address" },
    { indexed: false, internalType: "uint256", name: "amount", type: "uint256" },
    { indexed: true, internalType: "address", name: "depositor", type: "address" },
    { indexed: false, internalType: "bytes32", name: "mintRecipient", type: "bytes32" },
    { indexed: false, internalType: "uint32", name: "destinationDomain", type: "uint32" },
    { indexed: false, internalType: "bytes32", name: "destinationTokenMessenger", type: "bytes32" },
    { indexed: false, internalType: "bytes32", name: "destinationCaller", type: "bytes32" },
    { indexed: false, internalType: "uint256", name: "maxFee", type: "uint256" },
    { indexed: true, internalType: "uint32", name: "minFinalityThreshold", type: "uint32" },
    { indexed: false, internalType: "bytes", name: "hookData", type: "bytes" },
  ],
  name: "DepositForBurn",
  type: "event",
};
