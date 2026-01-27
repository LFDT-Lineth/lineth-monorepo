import { describe, afterEach, it, expect, beforeEach } from "@jest/globals";
import { MockProxy, mock, mockClear } from "jest-mock-extended";
import { EthersL2MessageServiceLogClient } from "../EthersL2MessageServiceLogClient";
import { TEST_MESSAGE_HASH, TEST_CONTRACT_ADDRESS_2 } from "../../../utils/testing/constants/common";
import {
  testMessageSentEvent,
  testMessageSentEventLog,
  testServiceVersionMigratedEvent,
  testServiceVersionMigratedEventLog,
} from "../../../utils/testing/constants/events";
import { L2MessageService, L2MessageService__factory } from "../../../contracts/typechain";
import { mockProperty } from "../../../utils/testing/helpers";
import { LineaProvider } from "../../providers";

describe("TestEthersL2MessgaeServiceLogClient", () => {
  let providerMock: MockProxy<LineaProvider>;
  let l2MessgaeServiceMock: MockProxy<L2MessageService>;
  let l2MessgaeServiceLogClient: EthersL2MessageServiceLogClient;

  beforeEach(() => {
    providerMock = mock<LineaProvider>();
    l2MessgaeServiceMock = mock<L2MessageService>();
    mockProperty(l2MessgaeServiceMock, "filters", {
      ...l2MessgaeServiceMock.filters,
      MessageSent: jest.fn(),
      ServiceVersionMigrated: jest.fn(),
    } as any);
    jest.spyOn(L2MessageService__factory, "connect").mockReturnValue(l2MessgaeServiceMock);

    l2MessgaeServiceLogClient = new EthersL2MessageServiceLogClient(providerMock, TEST_CONTRACT_ADDRESS_2);
  });

  afterEach(() => {
    mockClear(providerMock);
    mockClear(l2MessgaeServiceMock);
  });

  describe("getMessageSentEvents", () => {
    it("should return a MessageSentEvent", async () => {
      jest.spyOn(l2MessgaeServiceMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await l2MessgaeServiceLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 1,
      });

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });

    it("should return empty MessageSentEvent as event index is less than fromBlockLogIndex", async () => {
      jest.spyOn(l2MessgaeServiceMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await l2MessgaeServiceLogClient.getMessageSentEvents({
        fromBlock: 51,
        fromBlockLogIndex: 10,
      });

      expect(messageSentEvents).toStrictEqual([]);
    });
  });

  describe("getMessageSentEventsByMessageHash", () => {
    it("should return a MessageSentEvent", async () => {
      jest.spyOn(l2MessgaeServiceMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await l2MessgaeServiceLogClient.getMessageSentEventsByMessageHash({
        messageHash: TEST_MESSAGE_HASH,
      });

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });
  });

  describe("getMessageSentEventsByBlockRange", () => {
    it("should return a MessageSentEvent", async () => {
      jest.spyOn(l2MessgaeServiceMock, "queryFilter").mockResolvedValue([testMessageSentEventLog]);

      const messageSentEvents = await l2MessgaeServiceLogClient.getMessageSentEventsByBlockRange(51, 51);

      expect(messageSentEvents).toStrictEqual([testMessageSentEvent]);
    });
  });

  describe("getServiceVersionMigratedEvents", () => {
    it("should return a ServiceVersionMigratedEvent", async () => {
      jest.spyOn(l2MessgaeServiceMock, "queryFilter").mockResolvedValue([testServiceVersionMigratedEventLog]);

      const serviceVersionMigratedEvents = await l2MessgaeServiceLogClient.getServiceVersionMigratedEvents({});

      expect(serviceVersionMigratedEvents).toStrictEqual([testServiceVersionMigratedEvent]);
    });
  });
});
