import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { MockProxy, mock, mockClear } from "jest-mock-extended";
import { EthersLineaRollupLogClient } from "../EthersLineaRollupLogClient";
import { TEST_CONTRACT_ADDRESS_1 } from "../../../utils/testing/constants/common";
import {
  testL2MessagingBlockAnchoredEvent,
  testL2MessagingBlockAnchoredEventLog,
  testMessageClaimedEvent,
  testMessageClaimedEventLog,
  testMessageSentEvent,
  testMessageSentEventLog,
} from "../../../utils/testing/constants/events";
import { LineaRollup, LineaRollup__factory } from "../../../contracts/typechain";
import { mockProperty } from "../../../utils/testing/helpers";
import { Provider } from "../../providers";

describe("TestEthersLineaRollupLogClient", () => {
  let providerMock: MockProxy<Provider>;
  let lineaRollupMock: MockProxy<LineaRollup>;
  let lineaRollupLogClient: EthersLineaRollupLogClient;

  beforeEach(() => {
    providerMock = mock<Provider>();
    lineaRollupMock = mock<LineaRollup>();
    mockProperty(lineaRollupMock, "filters", {
      ...lineaRollupMock.filters,
      MessageSent: jest.fn(),
      L2MessagingBlockAnchored: jest.fn(),
      MessageClaimed: jest.fn(),
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
    } as any);
    jest.spyOn(LineaRollup__factory, "connect").mockReturnValue(lineaRollupMock);

    lineaRollupLogClient = new EthersLineaRollupLogClient(providerMock, TEST_CONTRACT_ADDRESS_1);
  });

  afterEach(() => {
    mockClear(providerMock);
    mockClear(lineaRollupMock);
  });

  describe("getMessageSentEvents", () => {
    it("should return a MessageSentEvent", async () => {
      jest.spyOn(lineaRollupMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await lineaRollupLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 1,
      });

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });

    it("should return empty MessageSentEvent as event index is less than fromBlockLogIndex", async () => {
      jest.spyOn(lineaRollupMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await lineaRollupLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 10,
      });

      expect(messageSentEvents).toStrictEqual([]);
    });
  });

  describe("getL2MessagingBlockAnchoredEvents", () => {
    it("should return a L2MessagingBlockAnchoredEvent", async () => {
      jest.spyOn(lineaRollupMock, "queryFilter").mockResolvedValue([testL2MessagingBlockAnchoredEventLog]);

      const l2MessagingBlockAnchoredEvents = await lineaRollupLogClient.getL2MessagingBlockAnchoredEvents({});

      expect(l2MessagingBlockAnchoredEvents).toStrictEqual([testL2MessagingBlockAnchoredEvent]);
    });
  });

  describe("getMessageClaimedEvents", () => {
    it("should return a MessageClaimedEvent", async () => {
      jest.spyOn(lineaRollupMock, "queryFilter").mockResolvedValue([testMessageClaimedEventLog]);

      const messageClaimedEvents = await lineaRollupLogClient.getMessageClaimedEvents({});

      expect(messageClaimedEvents).toStrictEqual([testMessageClaimedEvent]);
    });
  });
});
