import { getContractsAddressesByChainId } from "@lfdt-lineth/sdk-core";
import {
  Client,
  Transport,
  Chain,
  Account,
  Hex,
  ClientChainNotConfiguredError,
  ChainNotFoundError,
  ContractFunctionZeroDataError,
  AbiDecodingZeroDataError,
  zeroHash,
} from "viem";
import { getBlockNumber, getContractEvents, getTransactionReceipt, readContract } from "viem/actions";

import { getMessageProof, getMessageSiblings } from "./getMessageProof";
import { getMessageSentEvents } from "./getMessageSentEvents";
import { TEST_MERKLE_ROOT, TEST_MERKLE_ROOT_2, TEST_MESSAGE_HASH, TEST_TRANSACTION_HASH } from "../../tests/constants";
import {
  generateL2MerkleTreeAddedLog,
  generateL2MessagingBlockAnchoredLog,
  generateMessageSentLog,
  generateTransactionReceipt,
} from "../../tests/utils";
import { MessageNotFoundError } from "../errors/bridge";

jest.mock("./getMessageSentEvents");
jest.mock("viem/actions", () => ({
  getBlockNumber: jest.fn(),
  getContractEvents: jest.fn(),
  getTransactionReceipt: jest.fn(),
  readContract: jest.fn(),
}));

type MockClient = Client<Transport, Chain, Account>;

describe("getMessageProof", () => {
  const mainnetId = 1;
  const lineaId = 59144;
  const messageHash: Hex = TEST_MESSAGE_HASH;
  const l2BlockNumber = 42n;
  const treeDepth = 5;
  const merkleRoot = TEST_MERKLE_ROOT;
  const proof = [
    "0x0000000000000000000000000000000000000000000000000000000000000000",
    "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
    "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
    "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
    "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
  ] as Hex[];
  const leafIndex = 0;

  const mockClient = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  const mockL2Client = (chainId?: number): MockClient =>
    ({ chain: chainId ? { id: chainId } : undefined }) as unknown as MockClient;

  afterEach(() => {
    jest.clearAllMocks();
    (getMessageSentEvents as jest.Mock).mockReset();
    (getBlockNumber as jest.Mock).mockReset();
    (getContractEvents as jest.Mock).mockReset();
    (getTransactionReceipt as jest.Mock).mockReset();
    (readContract as jest.Mock).mockReset();
  });

  it("throws if l2Client.chain is not set", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client();
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(ClientChainNotConfiguredError);
  });

  it("throws if client.chain is not set", async () => {
    const client = mockClient();
    const l2Client = mockL2Client(lineaId);
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(ChainNotFoundError);
  });

  it("throws if no MessageSent event is found", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([]);
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      `Message with hash ${messageHash} not found.`,
    );
  });

  it("throws if no L2MessagingBlockAnchored event is found", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([]);
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      `L2 block number ${l2BlockNumber} is not finalized on L1 yet.`,
    );
  });

  it("throws if the settlement client chain has no default rollup address and rollupAddress is not provided", async () => {
    const client = mockClient(lineaId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      `Cannot resolve a default rollup contract address for chain ID ${lineaId}.`,
    );
  });

  it("throws if no MessageSent events in block range", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>)
      .mockResolvedValueOnce([
        {
          messageSender: messageSentLog.args._from!,
          destination: messageSentLog.args._to!,
          fee: messageSentLog.args._fee!,
          value: messageSentLog.args._value!,
          messageNonce: messageSentLog.args._nonce!,
          calldata: messageSentLog.args._calldata!,
          messageHash: messageSentLog.args._messageHash!,
          blockNumber: messageSentLog.blockNumber,
          logIndex: messageSentLog.logIndex,
          contractAddress: messageSentLog.address,
          transactionHash: messageSentLog.transactionHash,
        },
      ])
      .mockResolvedValueOnce([]); // for block range call
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(merkleRoot, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      [
        "No messages found in the specified block range on L2.",
        `Block range: ${l2BlockNumber} - ${l2BlockNumber}`,
      ].join("\n"),
    );
  });

  it("throws if merkle root does not match", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT_2, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      [
        "Merkle root 0xfc3dfe7470d41465e77e7c929170578b14a066a2272c2469b60162c5282e05a6 not found in finalization data.",
        `Block range: ${l2BlockNumber} - ${l2BlockNumber}`,
      ].join("\n"),
    );
  });

  it("returns proof on success", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });
  });

  it("does not fall back to binary search when the full-range query succeeds", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });
    // The permissive provider answered on the first call: no binary search probes.
    expect(getBlockNumber).not.toHaveBeenCalled();
    expect(readContract).not.toHaveBeenCalled();
  });

  it("falls back to binary search when the provider rejects the full block range", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);

    // Full-range query rejected with a provider range-limit error; the single-block query then succeeds.
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>)
      .mockRejectedValueOnce(new Error("range 0x0-0xC350 exceeds limit of 10000"))
      .mockResolvedValueOnce([
        generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
          address: getContractsAddressesByChainId(mainnetId).messageService,
        }),
      ]);
    (getBlockNumber as jest.Mock).mockResolvedValue(1000n);
    // currentL2BlockNumber() >= target at every probed block => search converges.
    (readContract as jest.Mock).mockResolvedValue(l2BlockNumber);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });
    expect(getBlockNumber).toHaveBeenCalled();
    expect(readContract).toHaveBeenCalled();
  });

  it("falls back to the bounded path on any full-range failure, even without range-limit wording", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);

    // An opaque provider error with no recognizable range/limit wording: the full-range query is a
    // pure optimization, so ANY failure must still trigger the bounded fallback.
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>)
      .mockRejectedValueOnce(new Error("Internal JSON-RPC error"))
      .mockResolvedValueOnce([
        generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
          address: getContractsAddressesByChainId(mainnetId).messageService,
        }),
      ]);
    (getBlockNumber as jest.Mock).mockResolvedValue(1000n);
    (readContract as jest.Mock).mockResolvedValue(l2BlockNumber);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });
    expect(getBlockNumber).toHaveBeenCalled();
    expect(readContract).toHaveBeenCalled();
  });

  it("binary-searches a large chain and queries a <=10k window containing the finalization", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>)
      .mockRejectedValueOnce(new Error("range 0x0-0x4c4b40 exceeds limit of 10000"))
      .mockResolvedValueOnce([
        generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
          address: getContractsAddressesByChainId(mainnetId).messageService,
        }),
      ]);

    // Head far beyond the 10k window so the bisection actually iterates; the finalization boundary
    // (first L1 block whose currentL2BlockNumber() covers the target) sits at block 3,000,000.
    const head = 5_000_000n;
    const finalizationBoundary = 3_000_000n;
    (getBlockNumber as jest.Mock).mockResolvedValue(head);
    (readContract as jest.Mock).mockImplementation((_client, params: { blockNumber?: bigint }) => {
      const probedBlock = params.blockNumber ?? head;
      return Promise.resolve(probedBlock >= finalizationBoundary ? l2BlockNumber : l2BlockNumber - 1n);
    });
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });

    // The fallback (second getContractEvents call) must target a <=10k window that brackets the boundary.
    const fallbackCall = (getContractEvents as jest.Mock).mock.calls[1][1];
    const fromBlock = fallbackCall.fromBlock as bigint;
    const toBlock = fallbackCall.toBlock as bigint;
    expect(toBlock - fromBlock + 1n).toBeLessThanOrEqual(10_000n);
    expect(fromBlock).toBeLessThanOrEqual(finalizationBoundary);
    expect(toBlock).toBeGreaterThanOrEqual(finalizationBoundary);
  });

  it.each([
    [
      "ContractFunctionZeroDataError",
      () => new ContractFunctionZeroDataError({ functionName: "currentL2BlockNumber" }),
    ],
    ["AbiDecodingZeroDataError", () => new AbiDecodingZeroDataError()],
  ])("treats a pre-deployment %s probe as nothing finalized during binary search", async (_name, makeError) => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>)
      .mockRejectedValueOnce(new Error("range 0x0-0x4c4b40 exceeds limit of 10000"))
      .mockResolvedValueOnce([
        generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
          address: getContractsAddressesByChainId(mainnetId).messageService,
        }),
      ]);

    // Contract deployed at block 3,000,000: probes below it return `0x` (no code) and must be read
    // as "nothing finalized" (0), not propagated; probes at/after it cover the target.
    const head = 5_000_000n;
    const deploymentBlock = 3_000_000n;
    (getBlockNumber as jest.Mock).mockResolvedValue(head);
    (readContract as jest.Mock).mockImplementation((_client, params: { blockNumber: bigint }) => {
      if (params.blockNumber < deploymentBlock) {
        return Promise.reject(makeError());
      }
      return Promise.resolve(l2BlockNumber);
    });
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });

    // The window must bracket the deployment boundary (the first block whose state covers the target).
    const fallbackCall = (getContractEvents as jest.Mock).mock.calls[1][1];
    expect(fallbackCall.fromBlock as bigint).toBeLessThanOrEqual(deploymentBlock);
    expect(fallbackCall.toBlock as bigint).toBeGreaterThanOrEqual(deploymentBlock);
  });

  it("chunks the L2 MessageSent query into ascending <=10k windows for a wide finalization range", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    const messageSent = {
      messageSender: messageSentLog.args._from!,
      destination: messageSentLog.args._to!,
      fee: messageSentLog.args._fee!,
      value: messageSentLog.args._value!,
      messageNonce: messageSentLog.args._nonce!,
      calldata: messageSentLog.args._calldata!,
      messageHash: messageSentLog.args._messageHash!,
      blockNumber: messageSentLog.blockNumber,
      logIndex: messageSentLog.logIndex,
      contractAddress: messageSentLog.address,
      transactionHash: messageSentLog.transactionHash,
    };
    // The hash lookup (no bigint fromBlock) returns the message; the chunked range calls (bigint
    // fromBlock) return empty so the run terminates after sweeping every window.
    (getMessageSentEvents as jest.Mock).mockImplementation((_client, params: { fromBlock?: unknown }) =>
      Promise.resolve(typeof params.fromBlock === "bigint" ? [] : [messageSent]),
    );
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);

    // Receipt anchors blocks 42 and 25042 => finalization messaging range spans 25,001 blocks.
    const rangeStart = 42n;
    const rangeEnd = 25_042n;
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(merkleRoot, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(rangeStart, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(rangeEnd, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      "No messages found in the specified block range on L2.",
    );

    const rangeCalls = (getMessageSentEvents as jest.Mock).mock.calls
      .map((call) => call[1])
      .filter((params) => typeof params.fromBlock === "bigint");
    // Contiguous, disjoint, ascending windows covering [42, 25042], none wider than 10k blocks.
    expect(rangeCalls).toEqual([
      expect.objectContaining({ fromBlock: 42n, toBlock: 10_041n }),
      expect.objectContaining({ fromBlock: 10_042n, toBlock: 20_041n }),
      expect.objectContaining({ fromBlock: 20_042n, toBlock: 25_042n }),
    ]);
    for (const params of rangeCalls) {
      expect((params.toBlock as bigint) - (params.fromBlock as bigint) + 1n).toBeLessThanOrEqual(10_000n);
    }
  });

  it("throws L2BlockNotFinalized via fallback when the block is not finalized yet", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockRejectedValueOnce(
      new Error("query exceeds limit of 10000 blocks"),
    );
    (getBlockNumber as jest.Mock).mockResolvedValue(1000n);
    // currentL2BlockNumber() < target everywhere => not finalized.
    (readContract as jest.Mock).mockResolvedValue(l2BlockNumber - 1n);

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      `L2 block number ${l2BlockNumber} is not finalized on L1 yet.`,
    );
  });

  it("propagates transient readContract errors during fallback instead of treating them as pre-deployment", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock<ReturnType<typeof getContractEvents>>).mockRejectedValueOnce(
      new Error("query exceeds limit of 10000 blocks"),
    );
    (getBlockNumber as jest.Mock).mockResolvedValue(1000n);
    // A transient network failure (not a "no contract code"/zero-data error) must surface, not be
    // swallowed as `0n`.
    (readContract as jest.Mock).mockRejectedValue(new Error("HTTP request failed: 503 Service Unavailable"));

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      "HTTP request failed: 503 Service Unavailable",
    );
  });

  it("propagates errors from getMessageSentEvents", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    (getMessageSentEvents as jest.Mock).mockRejectedValueOnce(new Error("getMessageSentEvents failed"));
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow("getMessageSentEvents failed");
  });

  it("surfaces the fallback error when the full-range query fails and the bounded path also fails", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    // Full-range query fails (optimization), and the bounded fallback then fails too: the real
    // error from the fallback must surface rather than being swallowed.
    (getContractEvents as jest.Mock).mockRejectedValueOnce(new Error("getContractEvents failed"));
    (getBlockNumber as jest.Mock).mockRejectedValue(new Error("HTTP request failed: 503 Service Unavailable"));
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      "HTTP request failed: 503 Service Unavailable",
    );
  });

  it("propagates errors from getTransactionReceipt", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockRejectedValueOnce(new Error("getTransactionReceipt failed"));
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow("getTransactionReceipt failed");
  });

  it("handles multiple MessageSent events in block range, selects correct one by message hash", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog1 = generateMessageSentLog({
      blockNumber: l2BlockNumber,
      args: { _messageHash: TEST_MESSAGE_HASH },
    });
    const messageSentLog2 = generateMessageSentLog({ blockNumber: l2BlockNumber, args: { _messageHash: messageHash } });
    (getMessageSentEvents as jest.Mock<ReturnType<typeof getMessageSentEvents>>)
      .mockResolvedValue([
        {
          messageSender: messageSentLog2.args._from!,
          destination: messageSentLog2.args._to!,
          fee: messageSentLog2.args._fee!,
          value: messageSentLog2.args._value!,
          messageNonce: messageSentLog2.args._nonce!,
          calldata: messageSentLog2.args._calldata!,
          messageHash: messageSentLog2.args._messageHash!,
          blockNumber: messageSentLog2.blockNumber,
          logIndex: messageSentLog2.logIndex,
          contractAddress: messageSentLog2.address,
          transactionHash: messageSentLog2.transactionHash,
        },
      ])
      .mockResolvedValueOnce([
        {
          messageSender: messageSentLog1.args._from!,
          destination: messageSentLog1.args._to!,
          fee: messageSentLog1.args._fee!,
          value: messageSentLog1.args._value!,
          messageNonce: messageSentLog1.args._nonce!,
          calldata: messageSentLog1.args._calldata!,
          messageHash: messageSentLog1.args._messageHash!,
          blockNumber: messageSentLog1.blockNumber,
          logIndex: messageSentLog1.logIndex,
          contractAddress: messageSentLog1.address,
          transactionHash: messageSentLog1.transactionHash,
        },
        {
          messageSender: messageSentLog2.args._from!,
          destination: messageSentLog2.args._to!,
          fee: messageSentLog2.args._fee!,
          value: messageSentLog2.args._value!,
          messageNonce: messageSentLog2.args._nonce!,
          calldata: messageSentLog2.args._calldata!,
          messageHash: messageSentLog2.args._messageHash!,
          blockNumber: messageSentLog2.blockNumber,
          logIndex: messageSentLog2.logIndex,
          contractAddress: messageSentLog2.address,
          transactionHash: messageSentLog2.transactionHash,
        },
      ]);
    (getContractEvents as jest.Mock).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );
    const result = await getMessageProof(client, { l2Client, messageHash });
    expect(result).toEqual({ proof, root: merkleRoot, leafIndex });
  });

  it("throws if MerkleTreeAdded log is missing in receipt", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          // No MerkleTreeAdded log
          generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      ["Event L2MerkleRootAdded not found in finalization data.", `Transaction hash: ${TEST_TRANSACTION_HASH}`].join(
        "\n",
      ),
    );
  });

  it("throws if L2MessagingBlockAnchored log is missing in receipt", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    const messageSentLog = generateMessageSentLog({ blockNumber: l2BlockNumber });
    (getMessageSentEvents as jest.Mock).mockResolvedValue([
      {
        messageSender: messageSentLog.args._from!,
        destination: messageSentLog.args._to!,
        fee: messageSentLog.args._fee!,
        value: messageSentLog.args._value!,
        messageNonce: messageSentLog.args._nonce!,
        calldata: messageSentLog.args._calldata!,
        messageHash: messageSentLog.args._messageHash!,
        blockNumber: messageSentLog.blockNumber,
        logIndex: messageSentLog.logIndex,
        contractAddress: messageSentLog.address,
        transactionHash: messageSentLog.transactionHash,
      },
    ]);
    (getContractEvents as jest.Mock).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(l2BlockNumber, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, treeDepth, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          // No L2MessagingBlockAnchored log
        ],
      }),
    );
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      [
        "Event L2MessagingBlockAnchored not found in finalization data.",
        `Transaction hash: ${TEST_TRANSACTION_HASH}`,
      ].join("\n"),
    );
  });

  it("throws if message hash is not found in messages", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    // Mock getMessageSentEvents to return a different message hash
    (getMessageSentEvents as jest.Mock).mockResolvedValue([
      {
        messageSender: "0xabc",
        destination: "0xdef",
        fee: 1n,
        value: 2n,
        messageNonce: 3n,
        calldata: "0x",
        messageHash: "0xnotfound",
        blockNumber: 42n,
        logIndex: 0,
        contractAddress: "0xcontract",
        transactionHash: "0xtx",
      },
    ]);
    // Mock getContractEvents to return a valid block anchor
    (getContractEvents as jest.Mock).mockResolvedValue([
      generateL2MessagingBlockAnchoredLog(42n, {
        address: getContractsAddressesByChainId(mainnetId).messageService,
      }),
    ]);
    // Mock getTransactionReceipt to return a valid receipt
    (getTransactionReceipt as jest.Mock).mockResolvedValue(
      generateTransactionReceipt({
        logs: [
          generateL2MerkleTreeAddedLog(TEST_MERKLE_ROOT, 5, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
          generateL2MessagingBlockAnchoredLog(42n, {
            address: getContractsAddressesByChainId(mainnetId).messageService,
          }),
        ],
      }),
    );

    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow(
      `Message with hash ${messageHash} not found.`,
    );
  });
});

describe("getMessageSiblings", () => {
  const hash = (n: number): Hex => `0x${n.toString(16).padStart(64, "0")}` as Hex;

  it("pads the final partial subtree with zero hashes", () => {
    // treeDepth 1 => 2 leaves per subtree; a single message leaves a remainder that must be padded.
    const siblings = getMessageSiblings(hash(1), [hash(1)], 1);
    expect(siblings).toHaveLength(2);
    expect(siblings[1]).toBe(zeroHash);
  });

  it("does not pad when the subtree is already full", () => {
    // treeDepth 1 => 2 leaves per subtree; exactly two messages fill it, so no padding (remainder 0).
    const siblings = getMessageSiblings(hash(1), [hash(1), hash(2)], 1);
    expect(siblings).toEqual([hash(1), hash(2)]);
  });

  it("throws when the message hash is absent", () => {
    expect(() => getMessageSiblings(hash(9), [hash(1), hash(2)], 1)).toThrow(MessageNotFoundError);
  });
});
