import { Message } from "@consensys/linea-sdk-core";
import { Block, Hex, Log, padHex, toHex, TransactionReceipt } from "viem";

import {
  L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE,
  L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE,
  MESSAGE_SENT_EVENT_SIGNATURE,
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_BLOCK_HASH,
  TEST_CONTRACT_ADDRESS_1,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "./constants";

export const generateTransactionReceipt = (overrides?: Partial<TransactionReceipt>): TransactionReceipt => {
  return {
    transactionHash: TEST_TRANSACTION_HASH,
    blockHash: TEST_BLOCK_HASH,
    to: TEST_CONTRACT_ADDRESS_1,
    from: TEST_ADDRESS_1,
    contractAddress: TEST_CONTRACT_ADDRESS_1,
    transactionIndex: 0,
    gasUsed: 70_000n,
    logsBloom: "0x",
    logs: [
      {
        transactionIndex: 0,
        blockNumber: 100_000n,
        removed: false,
        transactionHash: TEST_TRANSACTION_HASH,
        address: TEST_CONTRACT_ADDRESS_1,
        topics: [
          MESSAGE_SENT_EVENT_SIGNATURE,
          `0x000000000000000000000000${TEST_ADDRESS_1.slice(2)}`,
          `0x000000000000000000000000${TEST_ADDRESS_1.slice(2)}`,
          TEST_MESSAGE_HASH,
        ],
        data: "0x00000000000000000000000000000000000000000000000000038d7ea4c68000000000000000000000000000000000000000000000000000015fb7f9b8c3800000000000000000000000000000000000000000000000000000000000000003d700000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000",
        logIndex: 0,
        blockHash: TEST_BLOCK_HASH,
      },
    ],
    blockNumber: 100_000n,
    cumulativeGasUsed: 75_000n,
    effectiveGasPrice: 30_000n,
    root: "0x",
    type: "eip1559",
    status: "success",
    ...overrides,
  };
};

export const generateL2MessagingBlockAnchoredLog = (
  l2Block: bigint,
  overrides?: Partial<
    Log<
      bigint,
      number,
      false,
      undefined,
      undefined,
      readonly [
        {
          anonymous: false;
          inputs: [{ indexed: true; internalType: "uint256"; name: "l2Block"; type: "uint256" }];
          name: "L2MessagingBlockAnchored";
          type: "event";
        },
      ],
      "L2MessagingBlockAnchored"
    >
  >,
): Log<
  bigint,
  number,
  false,
  undefined,
  undefined,
  readonly [
    {
      anonymous: false;
      inputs: [{ indexed: true; internalType: "uint256"; name: "l2Block"; type: "uint256" }];
      name: "L2MessagingBlockAnchored";
      type: "event";
    },
  ],
  "L2MessagingBlockAnchored"
> => {
  return {
    args: {
      l2Block,
    },
    transactionIndex: 0,
    blockNumber: 100_000n,
    removed: false,
    transactionHash: TEST_TRANSACTION_HASH,
    address: TEST_CONTRACT_ADDRESS_1,
    topics: [L2_MESSAGING_BLOCK_ANCHORED_EVENT_SIGNATURE, padHex(toHex(l2Block), { size: 32, dir: "left" })],
    data: "0x",
    logIndex: 0,
    blockHash: TEST_BLOCK_HASH,
    eventName: "L2MessagingBlockAnchored",
    ...overrides,
  };
};

export const generateL2MerkleTreeAddedLog = (
  l2MerkleRoot: Hex,
  treeDepth: number,
  overrides?: Partial<
    Log<
      bigint,
      number,
      false,
      undefined,
      undefined,
      readonly [
        {
          anonymous: false;
          inputs: [
            { indexed: true; internalType: "bytes32"; name: "l2MerkleRoot"; type: "bytes32" },
            { indexed: true; internalType: "uint256"; name: "treeDepth"; type: "uint256" },
          ];
          name: "L2MerkleRootAdded";
          type: "event";
        },
      ],
      "L2MerkleRootAdded"
    >
  >,
): Log<
  bigint,
  number,
  false,
  undefined,
  undefined,
  readonly [
    {
      anonymous: false;
      inputs: [
        { indexed: true; internalType: "bytes32"; name: "l2MerkleRoot"; type: "bytes32" },
        { indexed: true; internalType: "uint256"; name: "treeDepth"; type: "uint256" },
      ];
      name: "L2MerkleRootAdded";
      type: "event";
    },
  ],
  "L2MerkleRootAdded"
> => {
  return {
    args: {
      l2MerkleRoot,
      treeDepth: BigInt(treeDepth),
    },
    transactionIndex: 0,
    blockNumber: 100_000n,
    removed: false,
    transactionHash: TEST_TRANSACTION_HASH,
    address: TEST_CONTRACT_ADDRESS_1,
    topics: [L2_MERKLE_TREE_ADDED_EVENT_SIGNATURE, l2MerkleRoot, padHex(toHex(treeDepth), { size: 32, dir: "right" })],
    data: "0x",
    logIndex: 0,
    blockHash: TEST_BLOCK_HASH,
    eventName: "L2MerkleRootAdded",
    ...overrides,
  };
};

export const generateMessageSentLog = (
  overrides?: Partial<
    Log<
      bigint,
      number,
      false,
      undefined,
      undefined,
      readonly [
        {
          anonymous: false;
          inputs: [
            { indexed: true; internalType: "address"; name: "_from"; type: "address" },
            { indexed: true; internalType: "address"; name: "_to"; type: "address" },
            { indexed: false; internalType: "uint256"; name: "_fee"; type: "uint256" },
            { indexed: false; internalType: "uint256"; name: "_value"; type: "uint256" },
            { indexed: false; internalType: "uint256"; name: "_nonce"; type: "uint256" },
            { indexed: false; internalType: "bytes"; name: "_calldata"; type: "bytes" },
            { indexed: true; internalType: "bytes32"; name: "_messageHash"; type: "bytes32" },
          ];
          name: "MessageSent";
          type: "event";
        },
      ],
      "MessageSent"
    >
  >,
): Log<
  bigint,
  number,
  false,
  undefined,
  undefined,
  readonly [
    {
      anonymous: false;
      inputs: [
        { indexed: true; internalType: "address"; name: "_from"; type: "address" },
        { indexed: true; internalType: "address"; name: "_to"; type: "address" },
        { indexed: false; internalType: "uint256"; name: "_fee"; type: "uint256" },
        { indexed: false; internalType: "uint256"; name: "_value"; type: "uint256" },
        { indexed: false; internalType: "uint256"; name: "_nonce"; type: "uint256" },
        { indexed: false; internalType: "bytes"; name: "_calldata"; type: "bytes" },
        { indexed: true; internalType: "bytes32"; name: "_messageHash"; type: "bytes32" },
      ];
      name: "MessageSent";
      type: "event";
    },
  ],
  "MessageSent"
> => {
  return {
    args: {
      _from: TEST_ADDRESS_1,
      _to: TEST_ADDRESS_2,
      _fee: 0n,
      _value: 0n,
      _nonce: 1n,
      _calldata: "0x",
      _messageHash: TEST_MESSAGE_HASH,
    },
    transactionIndex: 0,
    blockNumber: 100_000n,
    removed: false,
    transactionHash: TEST_TRANSACTION_HASH,
    address: TEST_CONTRACT_ADDRESS_1,
    data: "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000800000000000000000000000000000000000000000000000000000000000000000",
    topics: [
      MESSAGE_SENT_EVENT_SIGNATURE,
      `0x000000000000000000000000${TEST_ADDRESS_1.slice(2)}`,
      `0x000000000000000000000000${TEST_ADDRESS_2.slice(2)}`,
      TEST_MESSAGE_HASH,
    ],
    eventName: "MessageSent",
    logIndex: 0,
    blockHash: TEST_BLOCK_HASH,
    ...overrides,
  };
};

export const generateMessage = (overrides?: Partial<Message>): Message => {
  return {
    from: TEST_ADDRESS_1,
    to: TEST_CONTRACT_ADDRESS_1,
    fee: 10n,
    value: 2n,
    nonce: 1n,
    calldata: "0x",
    messageHash: TEST_MESSAGE_HASH,
    ...overrides,
  };
};

export const generateBlock = (overrides?: Partial<Block>): Block => {
  return {
    number: 100_000n,
    hash: TEST_BLOCK_HASH,
    parentHash: TEST_BLOCK_HASH,
    timestamp: 1_000_000_000n,
    transactions: [],
    logsBloom: "0x",
    difficulty: 0n,
    gasLimit: 15_000_000n,
    gasUsed: 10_000_000n,
    miner: TEST_ADDRESS_1,
    baseFeePerGas: 7n,
    extraData:
      "0x0100989680015eb3c80000ea600000000000000000000000000000000000000024997ceb570c667b9c369d351b384ce97dcfe0dda90696fc3b007b8d7160672548a6716cc33ffe0e4004c555a0c7edd9ddc2545a630f2276a2964dcf856e6ab501",
    withdrawals: [],
    withdrawalsRoot: "0x",
    uncles: [],
    receiptsRoot: "0x",
    stateRoot: "0x",
    mixHash: "0x",
    nonce: "0x",
    sha3Uncles: "0x",
    size: 100_000n,
    transactionsRoot: "0x",
    totalDifficulty: 1_000_000n,
    blobGasUsed: 0n,
    excessBlobGas: 0n,
    parentBeaconBlockRoot: "0x",
    sealFields: [],
    ...overrides,
  };
};
