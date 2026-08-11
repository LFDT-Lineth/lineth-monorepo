import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { MockProxy, mock, mockClear } from "jest-mock-extended";

import { LinethRollup, LinethRollup__factory } from "../../../contracts/typechain";
import { TEST_CONTRACT_ADDRESS_1 } from "../../../utils/testing/constants/common";
import {
  testL2MessagingBlockAnchoredEvent,
  testL2MessagingBlockAnchoredEventLog,
  testMessageClaimedEvent,
  testMessageClaimedEventLog,
  testMessageSentEvent,
  testMessageSentEventLog,
} from "../../../utils/testing/constants/events";
import { mockProperty } from "../../../utils/testing/helpers";
import { Provider } from "../../providers";
import { EthersLinethRollupLogClient } from "../EthersLinethRollupLogClient";

describe("TestEthersLinethRollupLogClient", () => {
  let providerMock: MockProxy<Provider>;
  let linethRollupMock: MockProxy<LinethRollup>;
  let linethRollupLogClient: EthersLinethRollupLogClient;

  beforeEach(() => {
    providerMock = mock<Provider>();
    linethRollupMock = mock<LinethRollup>();
    mockProperty(linethRollupMock, "filters", {
      ...linethRollupMock.filters,
      MessageSent: jest.fn(),
      L2MessagingBlockAnchored: jest.fn(),
      MessageClaimed: jest.fn(),
    } as any);
    jest.spyOn(LinethRollup__factory, "connect").mockReturnValue(linethRollupMock);

    linethRollupLogClient = new EthersLinethRollupLogClient(providerMock, TEST_CONTRACT_ADDRESS_1);
  });

  afterEach(() => {
    mockClear(providerMock);
    mockClear(linethRollupMock);
  });

  describe("getMessageSentEvents", () => {
    it("should return a MessageSentEvent", async () => {
      jest.spyOn(linethRollupMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await linethRollupLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 1,
      });

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });

    it("should return empty MessageSentEvent as event index is less than fromBlockLogIndex", async () => {
      jest.spyOn(linethRollupMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await linethRollupLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 10,
      });

      expect(messageSentEvents).toStrictEqual([]);
    });
  });

  describe("getL2MessagingBlockAnchoredEvents", () => {
    it("should return a L2MessagingBlockAnchoredEvent", async () => {
      jest.spyOn(linethRollupMock, "queryFilter").mockResolvedValue([testL2MessagingBlockAnchoredEventLog]);

      const l2MessagingBlockAnchoredEvents = await linethRollupLogClient.getL2MessagingBlockAnchoredEvents({});

      expect(l2MessagingBlockAnchoredEvents).toStrictEqual([testL2MessagingBlockAnchoredEvent]);
    });
  });

  describe("getMessageClaimedEvents", () => {
    it("should return a MessageClaimedEvent", async () => {
      jest.spyOn(linethRollupMock, "queryFilter").mockResolvedValue([testMessageClaimedEventLog]);

      const messageClaimedEvents = await linethRollupLogClient.getMessageClaimedEvents({});

      expect(messageClaimedEvents).toStrictEqual([testMessageClaimedEvent]);
    });
  });
});
