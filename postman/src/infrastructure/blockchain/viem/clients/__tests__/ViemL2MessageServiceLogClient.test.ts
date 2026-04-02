import { describe, it, expect, beforeEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import * as viemActions from "viem/actions";

import {
  TEST_ADDRESS_1,
  TEST_ADDRESS_2,
  TEST_CONTRACT_ADDRESS_2,
  TEST_MESSAGE_HASH,
  TEST_TRANSACTION_HASH,
} from "../../../../../utils/testing/constants";
import { ViemL2MessageServiceLogClient } from "../ViemL2MessageServiceLogClient";

import type { PublicClient } from "viem";

jest.mock("viem/actions", () => ({
  getContractEvents: jest.fn(),
}));

function makeMessageSentEvent(overrides: Partial<{ blockNumber: bigint; logIndex: number; nonce: bigint }> = {}) {
  return {
    removed: false,
    blockNumber: overrides.blockNumber ?? 100n,
    transactionHash: TEST_TRANSACTION_HASH,
    logIndex: overrides.logIndex ?? 0,
    args: {
      _messageHash: TEST_MESSAGE_HASH,
      _from: TEST_ADDRESS_1,
      _to: TEST_ADDRESS_2,
      _fee: 0n,
      _value: 500n,
      _nonce: overrides.nonce ?? 1n,
      _calldata: "0x",
    },
  };
}

describe("ViemL2MessageServiceLogClient", () => {
  let publicClient: ReturnType<typeof mock<PublicClient>>;
  let logClient: ViemL2MessageServiceLogClient;
  const getContractEventsMock = viemActions.getContractEvents as jest.MockedFunction<
    typeof viemActions.getContractEvents
  >;

  beforeEach(() => {
    publicClient = mock<PublicClient>();
    logClient = new ViemL2MessageServiceLogClient(publicClient, TEST_CONTRACT_ADDRESS_2);
    getContractEventsMock.mockReset();
  });

  describe("getMessageSentEvents", () => {
    it("returns mapped MessageSent events", async () => {
      getContractEventsMock.mockResolvedValue([makeMessageSentEvent()] as never);

      const events = await logClient.getMessageSentEvents({ fromBlock: 90n, toBlock: 110n });

      expect(events).toHaveLength(1);
      expect(events[0]).toMatchObject({
        messageHash: TEST_MESSAGE_HASH,
        messageSender: TEST_ADDRESS_1,
        destination: TEST_ADDRESS_2,
        fee: 0n,
        value: 500n,
        messageNonce: 1n,
        blockNumber: 100,
        transactionHash: TEST_TRANSACTION_HASH,
        logIndex: 0,
      });
    });

    it("filters out removed events", async () => {
      getContractEventsMock.mockResolvedValue([{ ...makeMessageSentEvent(), removed: true }] as never);
      const events = await logClient.getMessageSentEvents({});
      expect(events).toHaveLength(0);
    });

    it("filters events by fromBlockLogIndex within the fromBlock", async () => {
      getContractEventsMock.mockResolvedValue([
        makeMessageSentEvent({ blockNumber: 100n, logIndex: 0, nonce: 1n }),
        makeMessageSentEvent({ blockNumber: 100n, logIndex: 5, nonce: 2n }),
        makeMessageSentEvent({ blockNumber: 100n, logIndex: 10, nonce: 3n }),
        makeMessageSentEvent({ blockNumber: 101n, logIndex: 0, nonce: 4n }),
      ] as never);

      const events = await logClient.getMessageSentEvents({
        fromBlock: 100n,
        toBlock: 110n,
        fromBlockLogIndex: 5,
      });

      expect(events).toHaveLength(3);
      expect(events[0].logIndex).toBe(5);
      expect(events[1].logIndex).toBe(10);
      expect(events[2].logIndex).toBe(0);
    });

    it("converts number-typed block params to bigint via toBlockParam", async () => {
      getContractEventsMock.mockResolvedValue([makeMessageSentEvent()] as never);

      await logClient.getMessageSentEvents({
        fromBlock: 90 as unknown as bigint,
        toBlock: 110 as unknown as bigint,
      });

      expect(getContractEventsMock).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          fromBlock: 90n,
          toBlock: 110n,
        }),
      );
    });

    it("uses fallback block params when fromBlock and toBlock are undefined", async () => {
      getContractEventsMock.mockResolvedValue([makeMessageSentEvent()] as never);

      await logClient.getMessageSentEvents({});

      expect(getContractEventsMock).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          fromBlock: "earliest",
          toBlock: "latest",
        }),
      );
    });

    it("passes string block tags through directly", async () => {
      getContractEventsMock.mockResolvedValue([makeMessageSentEvent()] as never);

      await logClient.getMessageSentEvents({
        toBlock: "finalized" as unknown as bigint,
      });

      expect(getContractEventsMock).toHaveBeenCalledWith(
        publicClient,
        expect.objectContaining({
          toBlock: "finalized",
        }),
      );
    });
  });
});
