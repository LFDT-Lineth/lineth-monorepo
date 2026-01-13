import { getMessageProof } from "./getMessageProof";
import { Client, Transport, Chain, Account, Hex, ClientChainNotConfiguredError, ChainNotFoundError } from "viem";
import { getMessageSentEvents } from "./getMessageSentEvents";
import { getContractEvents, getTransactionReceipt } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";
import {
  generateL2MerkleTreeAddedLog,
  generateL2MessagingBlockAnchoredLog,
  generateMessageSentLog,
  generateTransactionReceipt,
} from "../../tests/utils";
import { TEST_MERKLE_ROOT, TEST_MERKLE_ROOT_2, TEST_MESSAGE_HASH, TEST_TRANSACTION_HASH } from "../../tests/constants";

jest.mock("./getMessageSentEvents");
jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
  getTransactionReceipt: jest.fn(),
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
    (getContractEvents as jest.Mock).mockReset();
    (getTransactionReceipt as jest.Mock).mockReset();
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

  it("propagates errors from getMessageSentEvents", async () => {
    const client = mockClient(mainnetId);
    const l2Client = mockL2Client(lineaId);
    (getMessageSentEvents as jest.Mock).mockRejectedValueOnce(new Error("getMessageSentEvents failed"));
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow("getMessageSentEvents failed");
  });

  it("propagates errors from getContractEvents", async () => {
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
    (getContractEvents as jest.Mock).mockRejectedValueOnce(new Error("getContractEvents failed"));
    await expect(getMessageProof(client, { l2Client, messageHash })).rejects.toThrow("getContractEvents failed");
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
