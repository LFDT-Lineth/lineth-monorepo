import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import * as viemActions from "viem/actions";

import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_1,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../../../../utils/testing/constants";
import { ViemLineaRollupLogClient } from "../ViemLineaRollupLogClient";

import type { PublicClient } from "viem";

jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
}));

describe("ViemLineaRollupLogClient", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let logClient: ViemLineaRollupLogClient;
  const getContractEventsMock = viemActions.getContractEvents as jest.MockedFunction<
    typeof viemActions.getContractEvents
  >;

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    logClient = new ViemLineaRollupLogClient(publicClient, TEST_CONTRACT_ADDRESS_1);
    getContractEventsMock.mockReset();
  });

  describe("getMessageSentEvents", () => {
    it("returns mapped MessageSent events", async () => {
      getContractEventsMock.mockResolvedValue([
        {
          removed: false,
          blockNumber: 100n,
          transactionHash: TEST_TRANSACTION_HASH,
          logIndex: 2,
          args: {
            _messageHash: TEST_MESSAGE_HASH,
            _from: TEST_ADDRESS_1,
            _to: TEST_ADDRESS_2,
            _fee: 0n,
            _value: 1000n,
            _nonce: 5n,
            _calldata: "0x",
          },
        },
      ] as never);

      const events = await logClient.getMessageSentEvents({ fromBlock: 90n, toBlock: 110n });

      expect(events).toHaveLength(1);
      expect(events[0]).toMatchObject({
        messageHash: TEST_MESSAGE_HASH,
        messageSender: TEST_ADDRESS_1,
        destination: TEST_ADDRESS_2,
        fee: 0n,
        value: 1000n,
        messageNonce: 5n,
        calldata: "0x",
        blockNumber: 100,
        transactionHash: TEST_TRANSACTION_HASH,
        logIndex: 2,
      });
    });

    it("filters out removed events", async () => {
      getContractEventsMock.mockResolvedValue([
        {
          removed: true,
          blockNumber: 100n,
          logIndex: 0,
          transactionHash: TEST_TRANSACTION_HASH,
          args: {
            _messageHash: TEST_MESSAGE_HASH,
            _from: TEST_ADDRESS_1,
            _to: TEST_ADDRESS_2,
            _fee: 0n,
            _value: 0n,
            _nonce: 1n,
            _calldata: "0x",
          },
        },
      ] as never);

      const events = await logClient.getMessageSentEvents({});
      expect(events).toHaveLength(0);
    });

    it("converts numeric fromBlock to bigint via toBlockParam", async () => {
      getContractEventsMock.mockResolvedValue([
        {
          removed: false,
          blockNumber: 90n,
          logIndex: 0,
          transactionHash: TEST_TRANSACTION_HASH,
          args: {
            _messageHash: TEST_MESSAGE_HASH,
            _from: TEST_ADDRESS_1,
            _to: TEST_ADDRESS_2,
            _fee: 0n,
            _value: 0n,
            _nonce: 1n,
            _calldata: "0x",
          },
        },
      ] as never);

      const events = await logClient.getMessageSentEvents({ fromBlock: 90 as unknown as bigint });

      expect(events).toHaveLength(1);
      expect(getContractEventsMock).toHaveBeenCalledWith(publicClient, expect.objectContaining({ fromBlock: 90n }));
    });

    it("filters by fromBlockLogIndex within same block", async () => {
      getContractEventsMock.mockResolvedValue([
        {
          removed: false,
          blockNumber: 100n,
          logIndex: 0,
          transactionHash: TEST_TRANSACTION_HASH,
          args: {
            _messageHash: TEST_MESSAGE_HASH,
            _from: TEST_ADDRESS_1,
            _to: TEST_ADDRESS_2,
            _fee: 0n,
            _value: 0n,
            _nonce: 1n,
            _calldata: "0x",
          },
        },
        {
          removed: false,
          blockNumber: 100n,
          logIndex: 3,
          transactionHash: TEST_TRANSACTION_HASH,
          args: {
            _messageHash: TEST_MESSAGE_HASH,
            _from: TEST_ADDRESS_1,
            _to: TEST_ADDRESS_2,
            _fee: 0n,
            _value: 0n,
            _nonce: 2n,
            _calldata: "0x",
          },
        },
      ] as never);

      const events = await logClient.getMessageSentEvents({ fromBlock: 100n, fromBlockLogIndex: 2 });
      expect(events).toHaveLength(1);
      expect(events[0].messageNonce).toBe(2n);
    });
  });
});
